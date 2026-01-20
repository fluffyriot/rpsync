package config

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/authhelp"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"golang.org/x/oauth2"

	_ "github.com/lib/pq"
)

type SyncMethodEnum string

const (
	Dev    SyncMethodEnum = "None / Dev"
	Csv    SyncMethodEnum = "CSV"
	Notion SyncMethodEnum = "Notion"
	NocoDb SyncMethodEnum = "NocoDb"
)

const AppVersion = "0.11"

type User struct {
	Id        uuid.UUID
	Username  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AppConfig struct {
	AppPort             string
	HttpsPort           string
	ClientIP            string
	InstagramAPIVersion string
	OauthEncryptionKey  string
	TokenEncryptionKey  []byte
	FBConfig            *oauth2.Config
	DBInitErr           error
	KeyB64Err2          error
	InstVerErr          error
	KeyB64Err1          error
}

func LoadConfig() (*AppConfig, error) {
	cfg := &AppConfig{}

	cfg.AppPort = os.Getenv("APP_PORT")
	if cfg.AppPort == "" {
		return nil, errors.New("APP_PORT is not set in the .env")
	}

	cfg.HttpsPort = os.Getenv("HTTPS_PORT")
	if cfg.HttpsPort == "" {
		return nil, errors.New("HTTPS_PORT is not set in the .env")
	}

	cfg.ClientIP = os.Getenv("LOCAL_IP")
	if cfg.ClientIP == "" {
		return nil, errors.New("LOCAL_IP is not set in the .env")
	}

	cfg.InstagramAPIVersion = os.Getenv("INSTAGRAM_API_VERSION")
	if cfg.InstagramAPIVersion == "" {
		cfg.InstVerErr = errors.New("INSTAGRAM_API_VERSION not set in .env")
	}

	cfg.OauthEncryptionKey = os.Getenv("OAUTH_ENCRYPTION_KEY")

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if keyB64 == "" {
		cfg.KeyB64Err1 = errors.New("TOKEN_ENCRYPTION_KEY not set in .env")
	} else {
		var err error
		cfg.TokenEncryptionKey, err = base64.StdEncoding.DecodeString(keyB64)
		if err != nil || len(cfg.TokenEncryptionKey) != 32 {
			cfg.KeyB64Err2 = fmt.Errorf("Error encoding encryption key: %v", err)
		}
	}

	cfg.FBConfig = authhelp.GenerateFacebookConfig(
		os.Getenv("FACEBOOK_APP_ID"),
		os.Getenv("FACEBOOK_APP_SECRET"),
		cfg.ClientIP,
		cfg.HttpsPort,
	)

	return cfg, nil
}

func LoadDatabase() (*database.Queries, error) {

	dbName := os.Getenv("POSTGRES_DB")
	dbUserName := os.Getenv("POSTGRES_USER")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")

	if dbName == "" || dbUserName == "" || dbPassword == "" {
		return nil, fmt.Errorf("Failed to load the environment configuration.")
	}

	connectDbUrl := fmt.Sprintf("postgres://%v:%v@db:5432/%v?sslmode=disable", dbUserName, dbPassword, dbName)

	db, err := sql.Open("postgres", connectDbUrl)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the DB. Error: %v", err)
	}

	migrationsDir := "./sql/schema"
	if err := goose.Up(db, migrationsDir); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	version, err := goose.EnsureDBVersion(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get DB version: %v", err)
	}
	fmt.Printf("Migrations applied successfully. Current DB version: %d\n", version)

	dbQueries := database.New(db)

	return dbQueries, nil
}

func CreateUserFromForm(dbQueries *database.Queries, userName string) (name, id string, e error) {

	u, err := dbQueries.CreateUser(context.Background(), database.CreateUserParams{
		ID:        uuid.New(),
		Username:  userName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	if err != nil {
		return "", "", fmt.Errorf("Failed to create user. Error: %v", err)
	}

	return u.Username, u.ID.String(), nil

}

func CreateSourceFromForm(dbQueries *database.Queries, uid, network, username, tgBotToken, tgChannelId, tgAppId, tgAppHash, googleKey, googlePropertyId string, encryptionKey []byte) (id, networkName string, e error) {

	uidParse, err := uuid.Parse(uid)
	if err != nil {
		return "", "", fmt.Errorf("Failed to parse UUID. Error: %v", err)
	}

	if network == "Telegram" && (tgBotToken == "" || tgChannelId == "" || tgAppId == "" || tgAppHash == "") {
		return "", "", fmt.Errorf("Channel Id, Bot Token, App Id and App Hash are required for Telegram")
	}

	if network == "Google Analytics" && (googleKey == "" || googlePropertyId == "") {
		return "", "", fmt.Errorf("Property ID and Service Account Key are required for Google Analytics")
	}

	s, err := dbQueries.CreateSource(context.Background(), database.CreateSourceParams{
		ID:           uuid.New(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Network:      network,
		UserName:     username,
		UserID:       uidParse,
		IsActive:     true,
		SyncStatus:   "Initialized",
		StatusReason: sql.NullString{},
	})

	if err != nil {
		return "", "", fmt.Errorf("Failed to create source. Error: %v", err)
	}

	if network == "Telegram" {
		tokenFormatted := tgBotToken + ":::" + tgAppId + ":::" + tgAppHash
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, tokenFormatted, tgChannelId, encryptionKey)
		if err != nil {
			dbQueries.DeleteSource(context.Background(), s.ID)
			return "", "", fmt.Errorf("Failed to create source with auth key. Error: %v", err)
		}
	}

	if network == "Google Analytics" {
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, googleKey, googlePropertyId, encryptionKey)
		if err != nil {
			dbQueries.DeleteSource(context.Background(), s.ID)
			return "", "", fmt.Errorf("Failed to create source with auth key. Error: %v", err)
		}
	}

	return s.ID.String(), s.Network, nil

}

func CreateTargetFromForm(dbQueries *database.Queries, uid, target, dbId, period, token, hostUrl string, encryptionKey []byte) (id, targetName string, e error) {

	uidParse, err := uuid.Parse(uid)
	if err != nil {
		return "", "", fmt.Errorf("Failed to parse UUID. Error: %v", err)
	}

	t, err := dbQueries.CreateTarget(context.Background(), database.CreateTargetParams{
		ID:            uuid.New(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		TargetType:    target,
		DbID:          sql.NullString{String: dbId, Valid: true},
		UserID:        uidParse,
		IsActive:      true,
		SyncStatus:    "Initialized",
		SyncFrequency: period,
		HostUrl:       sql.NullString{String: hostUrl, Valid: true},
	})

	if err != nil {
		return "", "", fmt.Errorf("Failed to create target. Error: %v", err)
	}

	if token != "" {

		err = authhelp.InsertTargetToken(context.Background(), dbQueries, t.ID, token, dbId, encryptionKey)

		if err != nil {
			return "", "", fmt.Errorf("Failed to store token. Error: %v", err)
		}

	}

	return t.ID.String(), t.TargetType, nil

}

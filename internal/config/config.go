// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"

	_ "github.com/lib/pq"
)

type SyncMethodEnum string

const (
	Dev    SyncMethodEnum = "None / Dev"
	Csv    SyncMethodEnum = "CSV"
	Notion SyncMethodEnum = "Notion"
	NocoDb SyncMethodEnum = "NocoDb"
)

var AppVersion string = "unknown"

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
	DomainName          string
	BaseURL             string
	InstagramAPIVersion string
	OauthEncryptionKey  string
	TokenEncryptionKey  []byte
	DBInitErr           error
	KeyB64Err2          error
	KeyB64Err1          error
	SessionKey          []byte
	WebAuthn            *webauthn.WebAuthn
	GinMode             string
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

	cfg.DomainName = os.Getenv("DOMAIN_NAME")

	cfg.InstagramAPIVersion = "v25.0"

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

	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		return nil, errors.New("SESSION_KEY not set in .env")
	} else {
		cfg.SessionKey = []byte(sessionKey)
	}

	baseURL := ""
	if cfg.DomainName != "" {
		baseURL = fmt.Sprintf("https://%s", cfg.DomainName)
	} else {
		baseURL = fmt.Sprintf("https://%s:%s", cfg.ClientIP, cfg.HttpsPort)
	}
	cfg.BaseURL = baseURL

	wConfig := &webauthn.Config{
		RPDisplayName: "RPSync",
		RPID:          cfg.ClientIP,
		RPOrigins:     []string{baseURL},
	}

	if cfg.DomainName != "" {
		wConfig.RPID = cfg.DomainName
	}

	var errW error
	cfg.WebAuthn, errW = webauthn.New(wConfig)
	if errW != nil {
		fmt.Printf("Warning: Failed to initialize WebAuthn: %v. Passkeys will be disabled.\n", errW)
	}

	cfg.GinMode = os.Getenv("GIN_MODE")
	if cfg.GinMode == "" {
		cfg.GinMode = "release"
	}

	return cfg, nil
}

func LoadDatabase() (*database.Queries, *sql.DB, error) {

	dbName := os.Getenv("POSTGRES_DB")
	dbUserName := os.Getenv("POSTGRES_USER")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")

	if dbName == "" || dbUserName == "" || dbPassword == "" {
		return nil, nil, fmt.Errorf("Failed to load the environment configuration.")
	}

	dbHost := os.Getenv("POSTGRES_HOST")
	if dbHost == "" {
		dbHost = "db"
	}

	dbSslMode := os.Getenv("POSTGRES_SSLMODE")
	if dbSslMode == "" {
		dbSslMode = "disable"
	}

	connectDbUrl := fmt.Sprintf("postgres://%v:%v@%v:5432/%v?sslmode=%v", dbUserName, dbPassword, dbHost, dbName, dbSslMode)

	db, err := sql.Open("postgres", connectDbUrl)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to connect to the DB. Error: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	migrationsDir := "./sql/schema"
	if err := goose.Up(db, migrationsDir); err != nil {
		return nil, nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	version, err := goose.EnsureDBVersion(db)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get DB version: %v", err)
	}
	fmt.Printf("Migrations applied successfully. Current DB version: %d\n", version)

	dbQueries := database.New(db)

	return dbQueries, db, nil
}

func CreateUserFromForm(dbQueries *database.Queries, userName string) (name, id string, e error) {

	u, err := dbQueries.CreateUser(context.Background(), database.CreateUserParams{
		ID:         uuid.New(),
		Username:   userName,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		SyncPeriod: "30m",
	})

	if err != nil {
		return "", "", fmt.Errorf("Failed to create user. Error: %v", err)
	}

	return u.Username, u.ID.String(), nil

}

type SourceCreationParams struct {
	UserID        string
	Network       string
	Username      string
	Field1        string
	Field2        string
	Field3        string
	Field4        string
	Field5        string
	FieldLong     string
	EncryptionKey []byte
}

func CreateSourceFromForm(dbQueries *database.Queries, params SourceCreationParams) (id, networkName string, e error) {

	uidParse, err := uuid.Parse(params.UserID)
	if err != nil {
		return "", "", fmt.Errorf("Failed to parse UUID. Error: %v", err)
	}

	if params.Network == "Telegram" && (params.Field1 == "" || params.Field2 == "" || params.Field3 == "" || params.Field4 == "") {
		return "", "", fmt.Errorf("Channel Id, Bot Token, App Id and App Hash are required for Telegram")
	}

	if params.Network == "Google Analytics" && (params.FieldLong == "" || params.Field1 == "") {
		return "", "", fmt.Errorf("Property ID and Service Account Key are required for Google Analytics")
	}

	if params.Network == "YouTube" && params.FieldLong == "" {
		return "", "", fmt.Errorf("Service Account Key is required for YouTube")
	}

	if params.Network == "Discord" && (params.Field1 == "" || params.Field2 == "" || params.Field3 == "") {
		return "", "", fmt.Errorf("Bot Token, Server ID, and Channel ID(s) are required for Discord")
	}

	if params.Network == "e621" && (params.Field1 == "" || params.Field2 == "") {
		return "", "", fmt.Errorf("API Key and API Username are required for e621")
	}

	if params.Network == "Twitch" && (params.Field1 == "" || params.Field2 == "") {
		return "", "", fmt.Errorf("Client ID and Client Secret are required for Twitch")
	}

	if params.Network == "DeviantArt" && (params.Field1 == "" || params.Field2 == "") {
		return "", "", fmt.Errorf("Client ID and Client Secret are required for DeviantArt")
	}

	if params.Network == "Weasyl" && params.Field1 == "" {
		return "", "", fmt.Errorf("API Key is required for Weasyl")
	}

	if params.Network == "Google Search Console" && params.FieldLong == "" {
		return "", "", fmt.Errorf("Service Account Key is required for Google Search Console")
	}

	if params.Network == "Threads" && params.Field1 == "" {
		return "", "", fmt.Errorf("Access Token is required for Threads")
	}

	if params.Network == "Murrtube" && params.FieldLong == "" {
		return "", "", fmt.Errorf("Cookie JSON is required for Murrtube")
	}

	if params.Network == "Murrtube" {
		var rawCookies []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(params.FieldLong), &rawCookies); err != nil {
			return "", "", fmt.Errorf("Invalid cookie JSON for Murrtube: %v", err)
		}
		hasSession := false
		for _, c := range rawCookies {
			if c.Name == "_murrtube_ultra_session" {
				hasSession = true
				break
			}
		}
		if !hasSession {
			return "", "", fmt.Errorf("Cookie JSON must contain a _murrtube_ultra_session cookie")
		}
	}

	if params.Network == "Twitter" && params.FieldLong == "" {
		return "", "", fmt.Errorf("Cookie JSON is required for Twitter")
	}

	if params.Network == "Twitter" {
		var rawCookies []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(params.FieldLong), &rawCookies); err != nil {
			return "", "", fmt.Errorf("Invalid cookie JSON for Twitter: %v", err)
		}
		hasAuthToken := false
		for _, c := range rawCookies {
			if c.Name == "auth_token" {
				hasAuthToken = true
				break
			}
		}
		if !hasAuthToken {
			return "", "", fmt.Errorf("Cookie JSON must contain an auth_token cookie")
		}
	}

	s, err := dbQueries.CreateSource(context.Background(), database.CreateSourceParams{
		ID:           uuid.New(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Network:      params.Network,
		UserName:     params.Username,
		UserID:       uidParse,
		IsActive:     true,
		SyncStatus:   "Initialized",
		StatusReason: sql.NullString{},
	})

	if err != nil {
		return "", "", fmt.Errorf("Failed to create source. Error: %v", err)
	}

	var tokenFormatted, profileFormatted string

	switch params.Network {
	case "Telegram":
		tokenFormatted = params.Field1 + ":::" + params.Field3 + ":::" + params.Field4
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, tokenFormatted, params.Field2, nil, params.EncryptionKey)

	case "Google Analytics":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.FieldLong, params.Field1, nil, params.EncryptionKey)

	case "YouTube":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.FieldLong, "", nil, params.EncryptionKey)

	case "Discord":
		tokenFormatted = params.Field1
		profileFormatted = params.Field2 + ":::" + params.Field3
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, tokenFormatted, profileFormatted, nil, params.EncryptionKey)

	case "e621":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.Field2, params.Field1, nil, params.EncryptionKey)

	case "Reddit":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.FieldLong, params.Field1, nil, params.EncryptionKey)

	case "Twitch":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.Field2, params.Field1, nil, params.EncryptionKey)

	case "DeviantArt":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.Field2, params.Field1, nil, params.EncryptionKey)

	case "Weasyl":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.Field1, "", nil, params.EncryptionKey)

	case "Google Search Console":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.FieldLong, "", nil, params.EncryptionKey)

	case "Threads":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.Field1, "", nil, params.EncryptionKey)

	case "Murrtube":
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.FieldLong, "", nil, params.EncryptionKey)

	case "Twitter":
		profileID := ""
		var rawCookies []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if jsonErr := json.Unmarshal([]byte(params.FieldLong), &rawCookies); jsonErr == nil {
			for _, c := range rawCookies {
				if c.Name == "twid" {
					decoded, _ := url.QueryUnescape(c.Value)
					profileID = strings.TrimPrefix(decoded, "u=")
					break
				}
			}
		}
		err = authhelp.InsertSourceToken(context.Background(), dbQueries, s.ID, params.FieldLong, profileID, nil, params.EncryptionKey)
	}

	if err != nil {
		dbQueries.DeleteSource(context.Background(), s.ID)
		return "", "", fmt.Errorf("Failed to create source with auth key. Error: %v", err)
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

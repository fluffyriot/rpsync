package config

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/auth"
	"github.com/fluffyriot/commission-tracker/internal/database"
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

type User struct {
	Id        uuid.UUID
	Username  string
	CreatedAt time.Time
	UpdatedAt time.Time
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

func CreateSourceFromForm(dbQueries *database.Queries, uid, network, username string) (id, networkName string, e error) {

	uidParse, err := uuid.Parse(uid)
	if err != nil {
		return "", "", fmt.Errorf("Failed to parse UUID. Error: %v", err)
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

		err = auth.InsertTargetToken(context.Background(), dbQueries, t.ID, token, dbId, encryptionKey)

		if err != nil {
			return "", "", fmt.Errorf("Failed to store token. Error: %v", err)
		}

	}

	return t.ID.String(), t.TargetType, nil

}

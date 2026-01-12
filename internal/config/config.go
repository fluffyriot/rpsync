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

	_ "github.com/lib/pq"
)

type SyncMethodEnum string

const (
	Dev    SyncMethodEnum = "None / Dev"
	Csv    SyncMethodEnum = "CSV"
	Notion SyncMethodEnum = "Notion"
)

type User struct {
	Id         uuid.UUID
	Username   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	SyncMethod SyncMethodEnum
	AccessKey  string
	TargetDbId string
}

func LoadDatabase() (*database.Queries, error) {
	dbName := os.Getenv("POSTGRES_DB")
	dbUserName := os.Getenv("POSTGRES_USER")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	dbPort := os.Getenv("POSTGRES_PORT")

	if dbName == "" || dbUserName == "" || dbPassword == "" {
		return nil, fmt.Errorf("Failed to load the environment configuration.")
	}

	connectDbUrl := fmt.Sprintf("postgres://%v:%v@localhost:%v/%v?sslmode=disable", dbUserName, dbPassword, dbPort, dbName)

	db, err := sql.Open("postgres", connectDbUrl)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the DB. Error: %v", err)
	}

	dbQueries := database.New(db)

	return dbQueries, nil
}

func CreateUserFromForm(dbQueries *database.Queries, userName, syncMethod string) (name, id string, e error) {

	u, err := dbQueries.CreateUser(context.Background(), database.CreateUserParams{
		ID:               uuid.New(),
		Username:         userName,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		SyncMethod:       syncMethod,
		AccessKey:        sql.NullString{},
		TargetDatabaseID: sql.NullString{},
	})

	if err != nil {
		return "", "", fmt.Errorf("Failed to create user. Error: %v", err)
	}

	return u.Username, u.ID.String(), nil
}

func CreateSourceFromForm(dbQueries *database.Queries, uid, network, username, token string, encryptionKey []byte) (id, networkName string, e error) {

	uidParse, err := uuid.Parse(uid)
	if err != nil {
		return "", "", fmt.Errorf("Failed to parse UUID. Error: %v", err)
	}

	if network == "Instagram" && token == "" {
		return "", "", fmt.Errorf("Failed to create Instagram source. Api token is required.")
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

	if network == "Instagram" {
		err = auth.InsertToken(dbQueries, s.ID, token, encryptionKey)
	}

	return s.ID.String(), s.Network, nil
}

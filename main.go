package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/auth"
	"github.com/fluffyriot/commission-tracker/internal/config"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/fluffyriot/commission-tracker/internal/fetcher"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// For demo purposes: mock service status
type ServiceStatus struct {
	Database string
	Broker   string
}

func main() {

	dbStatus := "DOWN"
	dbQueries, err := config.LoadDatabase()
	if err != nil {
		log.Fatalln(err)
		dbStatus = "DOWN"
	}

	// dev temp section

	err = dbQueries.EmptyUsers(context.Background())
	if err != nil {
		log.Fatalln(err)
	}

	user, err := dbQueries.CreateUser(context.Background(), database.CreateUserParams{
		ID:               uuid.New(),
		Username:         "riot.photos",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		SyncMethod:       "CSV",
		AccessKey:        sql.NullString{},
		TargetDatabaseID: sql.NullString{},
	})
	if err != nil {
		log.Fatalln(err)
	}

	bskySource, err := dbQueries.CreateSource(context.Background(), database.CreateSourceParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Network:   "Bluesky",
		UserName:  "riot.photos",
		UserID:    user.ID,
		IsActive:  true,
	})

	instaSource, err := dbQueries.CreateSource(context.Background(), database.CreateSourceParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Network:   "Instagram",
		UserName:  "_riotphotos_",
		UserID:    user.ID,
		IsActive:  true,
	})

	murrSource, err := dbQueries.CreateSource(context.Background(), database.CreateSourceParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Network:   "Murrtube",
		UserName:  "riotphotos",
		UserID:    user.ID,
		IsActive:  true,
	})

	badpupsSource, err := dbQueries.CreateSource(context.Background(), database.CreateSourceParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Network:   "BadPups",
		UserName:  "fluffyriot",
		UserID:    user.ID,
		IsActive:  true,
	})

	httpClient := fetcher.NewClient(60 * time.Second)

	err = fetcher.FetchBlueskyPosts(dbQueries, httpClient, user.ID, bskySource.ID)
	if err != nil {
		log.Fatalln(err)
	}

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if keyB64 == "" {
		log.Fatal("TOKEN_ENCRYPTION_KEY not set")
	}

	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(key) != 32 {
		log.Fatal("TOKEN_ENCRYPTION_KEY must be 32 bytes (base64)")
	}

	encryptionKey := key

	err = auth.InsertToken(dbQueries, user.ID, os.Getenv("INSTAGRAM_API"), encryptionKey)

	err = fetcher.FetchInstagramPosts(dbQueries, httpClient, user.ID, instaSource.ID, os.Getenv("INSTAGRAM_API_VERSION"), encryptionKey)
	if err != nil {
		log.Fatalln(err)
	}

	err = fetcher.FetchMurrtubePosts(user.ID, dbQueries, httpClient, murrSource.ID)
	if err != nil {
		log.Fatalln(err)
	}

	err = fetcher.FetchBadpupsPosts(user.ID, dbQueries, httpClient, badpupsSource.ID)
	if err != nil {
		log.Fatalln(err)
	}

	// dev temp section

	r := gin.Default()

	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./static")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Setup page
	r.GET("/setup", func(c *gin.Context) {
		c.HTML(http.StatusOK, "setup.html", nil)
	})

	r.POST("/setup", func(c *gin.Context) {
		username := c.PostForm("username")
		syncMethod := c.PostForm("sync_method")
		nameCreated, idCreated, err := config.CreateUserFromForm(dbQueries, username, syncMethod)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "setup.html", gin.H{
				"Message1": fmt.Sprintln("User failed to create!"),
				"Message2": fmt.Sprintf("Error: %v", err),
			})
		} else {
			c.HTML(http.StatusOK, "setup.html", gin.H{
				"Message1": fmt.Sprintf("User %v created successfully!", nameCreated),
				"Message2": fmt.Sprintf("Your Id: %v", idCreated),
			})
		}

	})

	// Status page
	r.GET("/status", func(c *gin.Context) {
		// TODO: replace with real checks
		brokerStatus := "OK"

		status := map[string]string{
			"Database":      dbStatus,
			"DatabaseClass": "ok",
			"Broker":        brokerStatus,
			"BrokerClass":   "ok",
		}

		if dbStatus != "OK" {
			status["DatabaseClass"] = "fail"
		}
		if brokerStatus != "OK" {
			status["BrokerClass"] = "fail"
		}

		c.HTML(http.StatusOK, "status.html", status)
	})

	r.Run(":8080")
}

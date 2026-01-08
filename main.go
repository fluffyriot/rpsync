package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/config"
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

	httpClient := fetcher.NewClient(60 * time.Second)

	parse1, _ := uuid.Parse("8f30934e-3135-4fcf-8c59-bc7246171694")
	parse2, _ := uuid.Parse("2e7a1d09-bfe8-4cdd-9cd1-fdec628fa948")

	err = fetcher.FetchBlueskyPosts(dbQueries, httpClient, parse1, parse2)
	if err != nil {
		log.Fatalln(err)
	}

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

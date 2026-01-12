package main

import (
	"context"
	"log"
	"net/http"

	"github.com/fluffyriot/commission-tracker/internal/config"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/gin-gonic/gin"
)

var (
	dbQueries *database.Queries
	dbInitErr error
)

func main() {
	r := gin.Default()

	r.Static("/static", "./static")

	r.LoadHTMLGlob("templates/*.html")

	dbQueries, dbInitErr = config.LoadDatabase()
	if dbInitErr != nil {
		log.Printf("database init failed: %v", dbInitErr)
	}

	r.GET("/", rootHandler)
	r.POST("/user/setup", userSetupHandler)
	r.POST("/sources/setup", sourcesSetupHandler)

	r.POST("/reset", func(c *gin.Context) {
		err := dbQueries.EmptyUsers(context.Background())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

func rootHandler(c *gin.Context) {

	if dbInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": dbInitErr.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	users, err := dbQueries.GetAllUsers(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	if len(users) == 0 {
		c.HTML(http.StatusOK, "user-setup.html", nil)
		return
	}

	user := users[0]

	sources, err := dbQueries.GetUserActiveSources(ctx, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}
	c.HTML(http.StatusOK, "index.html", gin.H{
		"username": user.Username,
		"user_id":  user.ID,
		"sources":  sources,
	})
}

func userSetupHandler(c *gin.Context) {
	username := c.PostForm("username")
	if username == "" {
		c.HTML(http.StatusBadRequest, "user-setup.html", gin.H{
			"error": "username is required",
		})
		return
	}

	syncMethod := c.PostForm("sync_method")
	if syncMethod == "" {
		c.HTML(http.StatusBadRequest, "user-setup.html", gin.H{
			"error": "Sync method is required",
		})
		return
	}

	_, _, err := config.CreateUserFromForm(dbQueries, username, syncMethod)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "user-setup.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func sourcesSetupHandler(c *gin.Context) {
	userID := c.PostForm("user_id")
	network := c.PostForm("network")
	username := c.PostForm("username")

	if userID == "" || network == "" || username == "" {
		c.HTML(http.StatusBadRequest, "sources-setup.html", gin.H{
			"error": "all fields are required",
		})
		return
	}

	_, _, err := config.CreateSourceFromForm(dbQueries, userID, network, username)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "sources-setup.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

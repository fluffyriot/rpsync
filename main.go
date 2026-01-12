package main

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"os"

	"github.com/fluffyriot/commission-tracker/internal/config"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/gin-gonic/gin"
)

var (
	dbQueries  *database.Queries
	dbInitErr  error
	keyB64err1 error
	keyB64err2 error
)

func main() {

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if keyB64 == "" {
		log.Fatal("TOKEN_ENCRYPTION_KEY not set")
	}

	encryptKey, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(encryptKey) != 32 {
		log.Fatal("TOKEN_ENCRYPTION_KEY must be 32 bytes (base64)")
	}

	r := gin.Default()

	r.Static("/static", "./static")

	r.LoadHTMLGlob("templates/*.html")

	dbQueries, dbInitErr = config.LoadDatabase()
	if dbInitErr != nil {
		log.Printf("database init failed: %v", dbInitErr)
	}

	r.GET("/", rootHandler)
	r.POST("/user/setup", userSetupHandler)
	r.POST("/sources/setup", sourcesSetupHandler(encryptKey))

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

	if keyB64err1 != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": keyB64err1.Error(),
		})
		return
	}

	if keyB64err2 != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": keyB64err2.Error(),
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
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "username is required",
		})
		return
	}

	syncMethod := c.PostForm("sync_method")
	if syncMethod == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Sync method is required",
		})
		return
	}

	_, _, err := config.CreateUserFromForm(dbQueries, username, syncMethod)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func sourcesSetupHandler(encryptKey []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.PostForm("user_id")
		network := c.PostForm("network")
		username := c.PostForm("username")
		token := c.PostForm("api_token")

		if userID == "" || network == "" || username == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "all fields are required",
			})
			return
		}

		_, _, err := config.CreateSourceFromForm(
			dbQueries,
			userID,
			network,
			username,
			token,
			encryptKey,
		)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": err.Error(),
			})
			return
		}

		c.Redirect(http.StatusSeeOther, "/")
	}
}

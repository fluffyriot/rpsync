package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/config"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/fluffyriot/commission-tracker/internal/exports"
	"github.com/fluffyriot/commission-tracker/internal/fetcher"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	dbQueries  *database.Queries
	dbInitErr  error
	keyB64Err1 error
	keyB64Err2 error
	instVerErr error
)

func main() {

	appPort := ":" + os.Getenv("APP_PORT")
	if appPort == ":" {
		log.Fatal("APP_PORT is not set in the .env")
	}

	instVer := os.Getenv("INSTAGRAM_API_VERSION")
	if instVer == "" {
		instVerErr = errors.New("INSTAGRAM_API_VERSION not set in .env")
	}

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if keyB64 == "" {
		keyB64Err1 = errors.New("TOKEN_ENCRYPTION_KEY not set in .env")
	}

	encryptKey, keyB64Err2 := base64.StdEncoding.DecodeString(keyB64)
	if keyB64Err2 != nil || len(encryptKey) != 32 {
		keyB64Err2 = fmt.Errorf("Error encoding encryption key: %v", keyB64Err2)
	}

	client := fetcher.NewClient(60 * time.Second)

	r := gin.Default()

	r.Static("/static", "./static")

	r.LoadHTMLGlob("templates/*.html")

	dbQueries, dbInitErr = config.LoadDatabase()
	if dbInitErr != nil {
		log.Printf("database init failed: %v", dbInitErr)
	}

	r.GET("/", rootHandler)
	r.GET("/exports", exportsHandler)
	r.POST("/export/start", exportStartHandler(dbQueries))
	r.POST("/exports/deleteAll", exportDeleteAllHandler(dbQueries))

	r.GET("/outputs/*filepath", func(c *gin.Context) {
		p := c.Param("filepath")[1:]
		c.FileAttachment(filepath.Join("./outputs", p), filepath.Base(p))
	})
	r.POST("/user/setup", userSetupHandler)
	r.POST("/sources/setup", sourcesSetupHandler(encryptKey))
	r.POST("/sources/deactivate", deactivateSourceHandler)
	r.POST("/sources/activate", activateSourceHandler)
	r.POST("/sources/delete", deleteSourceHandler)
	r.POST("/sources/sync", syncSourceHandler(encryptKey, dbQueries, client, instVer))
	r.POST("/sources/syncAll", syncAllHandler(encryptKey, dbQueries, client, instVer))

	r.POST("/reset", func(c *gin.Context) {
		err := dbQueries.EmptyUsers(context.Background())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if err := r.Run(appPort); err != nil {
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

	if keyB64Err1 != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": keyB64Err1.Error(),
		})
		return
	}

	if keyB64Err2 != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": keyB64Err2.Error(),
		})
		return
	}

	if instVerErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": instVerErr.Error(),
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

	sources, err := dbQueries.GetUserSources(ctx, user.ID)
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

func exportsHandler(c *gin.Context) {

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

	exports, err := dbQueries.GetLast20ExportsByUserId(ctx, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}
	c.HTML(http.StatusOK, "exports.html", gin.H{
		"username":    user.Username,
		"user_id":     user.ID,
		"sync_method": user.SyncMethod,
		"exports":     exports,
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

func exportStartHandler(dbQueries *database.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := uuid.Parse(c.PostForm("user_id"))
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": err.Error(),
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

		go func(uid uuid.UUID, method string) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic in background sync: %v", r)
				}
			}()
			exports.InitiateExport(uid, method, dbQueries)
		}(userId, syncMethod)

		c.Redirect(http.StatusSeeOther, "/")
	}
}

func exportDeleteAllHandler(dbQueries *database.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := uuid.Parse(c.PostForm("user_id"))
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": err.Error(),
			})
			return
		}

		go func(uid uuid.UUID) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic in background sync: %v", r)
				}
			}()
			exports.DeleteAllExports(uid, dbQueries)
		}(userId)

		c.Redirect(http.StatusSeeOther, "/")
	}
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

func deactivateSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	_, err = dbQueries.ChangeSourceStatusById(
		context.Background(),
		database.ChangeSourceStatusByIdParams{
			ID:           sourceID,
			IsActive:     false,
			SyncStatus:   "Deactivated",
			StatusReason: sql.NullString{String: "Sync stopped by the user", Valid: true},
		},
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func deleteSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	err = dbQueries.DeleteSource(context.Background(), sourceID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func activateSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	_, err = dbQueries.ChangeSourceStatusById(
		context.Background(),
		database.ChangeSourceStatusByIdParams{
			ID:           sourceID,
			IsActive:     true,
			SyncStatus:   "Initialized",
			StatusReason: sql.NullString{},
		},
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func syncSourceHandler(encryptKey []byte, dbQueries *database.Queries, client *fetcher.Client, ver string) gin.HandlerFunc {
	return func(c *gin.Context) {
		sourceID, err := uuid.Parse(c.PostForm("source_id"))
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": err.Error(),
			})
			return
		}

		go func(sid uuid.UUID) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic in background sync: %v", r)
				}
			}()
			fetcher.SyncBySource(sid, dbQueries, client, ver, encryptKey)
		}(sourceID)

		c.Redirect(http.StatusSeeOther, "/")
	}
}

func syncAllHandler(encryptKey []byte, dbQueries *database.Queries, client *fetcher.Client, ver string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := uuid.Parse(c.PostForm("user_id"))
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": err.Error(),
			})
			return
		}

		sources, err := dbQueries.GetUserActiveSources(context.Background(), userID)

		for _, sourceID := range sources {
			go func(sid uuid.UUID) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("panic in background sync: %v", r)
					}
				}()
				fetcher.SyncBySource(sid, dbQueries, client, ver, encryptKey)
			}(sourceID.ID)
		}

		c.Redirect(http.StatusSeeOther, "/")
	}
}

package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/fluffyriot/commission-tracker/internal/config"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/fluffyriot/commission-tracker/internal/fetcher"
	"github.com/fluffyriot/commission-tracker/internal/puller"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) UserSetupHandler(c *gin.Context) {
	username := c.PostForm("username")
	if username == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "username is required",
		})
		return
	}

	_, _, err := config.CreateUserFromForm(h.DB, username)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) SourcesSetupHandler(c *gin.Context) {
	userID := c.PostForm("user_id")
	network := c.PostForm("network")
	username := c.PostForm("username")
	instaProfileId := c.PostForm("instagram_profile_id")
	tgBotToken := c.PostForm("telegram_bot_token")
	tgChannelId := c.PostForm("telegram_channel_id")
	tgAppId := c.PostForm("telegram_app_id")
	tgAppHash := c.PostForm("telegram_app_hash")

	if userID == "" || network == "" || username == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "all fields are required",
		})
		return
	}

	sid, _, err := config.CreateSourceFromForm(
		h.DB,
		userID,
		network,
		username,
		tgBotToken,
		tgChannelId,
		tgAppId,
		tgAppHash,
		h.EncryptKey,
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	if network == "Instagram" {
		c.Redirect(http.StatusSeeOther, "/auth/facebook/login?sid="+sid+"&pid="+instaProfileId)
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) DeactivateSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	_, err = h.DB.ChangeSourceStatusById(
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

func (h *Handler) ActivateSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	_, err = h.DB.ChangeSourceStatusById(
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

func (h *Handler) DeleteSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	syncedTargets, err := h.DB.GetSourcesOfTarget(context.Background(), sourceID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	for _, target := range syncedTargets {
		err = puller.RemoveByTarget(target.TargetID, sourceID, h.DB, h.Puller, h.EncryptKey)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	err = h.DB.DeleteSource(context.Background(), sourceID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) SyncSourceHandler(c *gin.Context) {
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
		fetcher.SyncBySource(sid, h.DB, h.Fetcher, h.InstVer, h.EncryptKey)
	}(sourceID)

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) SyncAllHandler(c *gin.Context) {
	userID, err := uuid.Parse(c.PostForm("user_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	sources, err := h.DB.GetUserActiveSources(context.Background(), userID)
	if err != nil {
		log.Printf("Error getting user active sources: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	for _, sourceID := range sources {
		go func(sid uuid.UUID) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic in background sync: %v", r)
				}
			}()
			fetcher.SyncBySource(sid, h.DB, h.Fetcher, h.InstVer, h.EncryptKey)
		}(sourceID.ID)
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) ResetHandler(c *gin.Context) {
	err := h.DB.EmptyUsers(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

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

func (h *Handler) SourcesHandler(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       h.Config.DBInitErr.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	ctx := c.Request.Context()

	users, err := h.DB.GetAllUsers(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	if len(users) == 0 {
		c.HTML(http.StatusOK, "user-setup.html", nil)
		return
	}

	user := users[0]

	sources, err := h.DB.GetUserSources(ctx, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	c.HTML(http.StatusOK, "sources.html", gin.H{
		"username":    user.Username,
		"user_id":     user.ID,
		"sources":     sources,
		"app_version": config.AppVersion,
	})
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
	googlePropertyId := c.PostForm("google_analytics_property_id")
	googleKey := c.PostForm("google_service_account_key")

	if userID == "" || network == "" || username == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":       "all fields are required",
			"app_version": config.AppVersion,
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
		googleKey,
		googlePropertyId,
		h.Config.TokenEncryptionKey,
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	if network == "Instagram" {
		c.Redirect(http.StatusSeeOther, "/auth/facebook/login?sid="+sid+"&pid="+instaProfileId)
		return
	}

	if network == "TikTok" {
		c.Redirect(http.StatusSeeOther, "/auth/tiktok/login?username="+username)
		return
	}

	c.Redirect(http.StatusSeeOther, "/sources")
}

func (h *Handler) DeactivateSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
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
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/sources")
}

func (h *Handler) ActivateSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
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
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/sources")
}

func (h *Handler) DeleteSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	syncedTargets, err := h.DB.GetSourcesOfTarget(context.Background(), sourceID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	for _, target := range syncedTargets {
		err = puller.RemoveByTarget(target.TargetID, sourceID, h.DB, h.Puller, h.Config.TokenEncryptionKey)
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
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/sources")
}

func (h *Handler) SyncSourceHandler(c *gin.Context) {
	sourceID, err := uuid.Parse(c.PostForm("source_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	go func(sid uuid.UUID) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in background sync: %v", r)
			}
		}()
		fetcher.SyncBySource(sid, h.DB, h.Fetcher, h.Config.InstagramAPIVersion, h.Config.TokenEncryptionKey)
	}(sourceID)

	c.Redirect(http.StatusSeeOther, "/sources")
}

func (h *Handler) SyncAllHandler(c *gin.Context) {
	userID, err := uuid.Parse(c.PostForm("user_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	sources, err := h.DB.GetUserActiveSources(context.Background(), userID)
	if err != nil {
		log.Printf("Error getting user active sources: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
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
			fetcher.SyncBySource(sid, h.DB, h.Fetcher, h.Config.InstagramAPIVersion, h.Config.TokenEncryptionKey)
		}(sourceID.ID)
	}

	c.Redirect(http.StatusSeeOther, "/sources")
}

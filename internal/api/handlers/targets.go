package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/pusher"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) TargetsHandler(c *gin.Context) {

	if h.Config.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": h.Config.DBInitErr.Error(),
			"title": "Error",
		}))
		return
	}

	ctx := c.Request.Context()

	users, err := h.DB.GetAllUsers(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	if len(users) == 0 {
		c.HTML(http.StatusOK, "user-setup.html", h.CommonData(gin.H{
			"title": "Setup",
		}))
		return
	}

	user := users[0]

	targets, err := h.DB.GetUserTargets(ctx, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}
	c.HTML(http.StatusOK, "targets.html", h.CommonData(gin.H{
		"username": user.Username,
		"user_id":  user.ID,
		"targets":  targets,
		"title":    "Targets",
	}))
}

func (h *Handler) TargetsSetupHandler(c *gin.Context) {
	userID := c.PostForm("user_id")
	target := c.PostForm("target")
	dbId := c.PostForm("db_id")
	token := c.PostForm("api_token")
	hostUrl := c.PostForm("host_url")
	period := "PT30M"

	if userID == "" || target == "" || period == "" {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(gin.H{
			"error": "All fields are required",
			"title": "Error",
		}))
		return
	}

	_, _, err := config.CreateTargetFromForm(
		h.DB,
		userID,
		target,
		dbId,
		period,
		token,
		hostUrl,
		h.Config.TokenEncryptionKey,
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	c.Redirect(http.StatusSeeOther, "/targets")
}

func (h *Handler) ActivateTargetHandler(c *gin.Context) {
	targetID, err := uuid.Parse(c.PostForm("target_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	_, err = h.DB.ChangeTargetStatusById(
		context.Background(),
		database.ChangeTargetStatusByIdParams{
			ID:           targetID,
			IsActive:     true,
			SyncStatus:   "Initialized",
			StatusReason: sql.NullString{},
		},
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	c.Redirect(http.StatusSeeOther, "/targets")
}

func (h *Handler) DeactivateTargetHandler(c *gin.Context) {
	targetID, err := uuid.Parse(c.PostForm("target_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	_, err = h.DB.ChangeTargetStatusById(
		context.Background(),
		database.ChangeTargetStatusByIdParams{
			ID:           targetID,
			IsActive:     false,
			SyncStatus:   "Deactivated",
			StatusReason: sql.NullString{String: "Sync stopped by the user", Valid: true},
		},
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	c.Redirect(http.StatusSeeOther, "/targets")
}

func (h *Handler) DeleteTargetHandler(c *gin.Context) {
	targetID, err := uuid.Parse(c.PostForm("target_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	err = h.DB.DeleteTarget(context.Background(), targetID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	c.Redirect(http.StatusSeeOther, "/targets")
}

func (h *Handler) SyncTargetHandler(c *gin.Context) {
	targetID, err := uuid.Parse(c.PostForm("target_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	go func(tid uuid.UUID) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in background sync: %v", r)
			}
		}()
		pusher.PullByTarget(tid, h.DB, h.Puller, h.Config.TokenEncryptionKey, true)
	}(targetID)

	c.Redirect(http.StatusSeeOther, "/targets")
}

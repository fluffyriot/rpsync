// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"database/sql"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	fetcher_common "github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/fluffyriot/rpsync/internal/updater"
	"github.com/fluffyriot/rpsync/internal/worker"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	DB      *database.Queries
	DBConn  *sql.DB
	Fetcher *fetcher_common.Client
	Puller  *common.Client
	Config  *config.AppConfig
	Worker  *worker.Worker
	Updater *updater.Updater
}

func NewHandler(db *database.Queries, dbConn *sql.DB, clientFetch *fetcher_common.Client, clientPull *common.Client, cfg *config.AppConfig, w *worker.Worker, upd *updater.Updater) *Handler {
	return &Handler{
		DB:      db,
		DBConn:  dbConn,
		Fetcher: clientFetch,
		Puller:  clientPull,
		Config:  cfg,
		Worker:  w,
		Updater: upd,
	}
}

func (h *Handler) CommonData(c *gin.Context, data gin.H) gin.H {
	data["app_version"] = config.AppVersion
	if h.Updater.IsUpdateAvailable() {
		data["update_available"] = true
		info := h.Updater.GetUpdateInfo()
		data["update_version"] = info.Latest
	}

	if val, exists := c.Get("username"); exists {
		data["username"] = val
	}
	if val, exists := c.Get("user_id"); exists {
		data["user_id"] = val
	}
	if val, exists := c.Get("has_avatar"); exists {
		data["has_avatar"] = val
	}
	if val, exists := c.Get("username_initial"); exists {
		data["username_initial"] = val
	}
	if val, exists := c.Get("avatar_version"); exists {
		data["avatar_version"] = val
	}
	if val, exists := c.Get("last_seen_version"); exists {
		data["user_last_seen_version"] = val
	}
	if val, exists := c.Get("intro_completed"); exists {
		data["intro_completed"] = val
	}

	networkColors := make(map[string]string)
	for _, source := range helpers.AvailableSources {
		networkColors[source.Name] = source.Color
	}
	for _, target := range helpers.AvailableTargets {
		networkColors[target.Name] = target.Color
	}
	data["network_colors"] = networkColors

	return data
}

func (h *Handler) GetAuthenticatedUser(c *gin.Context) (*database.User, bool) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		return nil, false
	}

	id, err := uuid.Parse(userID.(string))
	if err != nil {
		return nil, false
	}

	user, err := h.DB.GetUserByID(c.Request.Context(), id)
	if err != nil {
		return nil, false
	}

	return &user, true
}

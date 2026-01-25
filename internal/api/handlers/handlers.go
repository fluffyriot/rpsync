package handlers

import (
	"database/sql"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/fluffyriot/rpsync/internal/updater"
	"github.com/fluffyriot/rpsync/internal/worker"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	DB      *database.Queries
	DBConn  *sql.DB
	Fetcher *fetcher.Client
	Puller  *common.Client
	Config  *config.AppConfig
	Worker  *worker.Worker
	Updater *updater.Updater
}

func NewHandler(db *database.Queries, dbConn *sql.DB, clientFetch *fetcher.Client, clientPull *common.Client, cfg *config.AppConfig, w *worker.Worker, upd *updater.Updater) *Handler {
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

func (h *Handler) CommonData(data gin.H) gin.H {
	data["app_version"] = config.AppVersion
	if h.Updater.IsUpdateAvailable() {
		data["update_available"] = true
		info := h.Updater.GetUpdateInfo()
		data["update_version"] = info.Latest
		data["update_url"] = info.Url
		data["update_desc"] = info.ShortDescription
	}
	return data
}

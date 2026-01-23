package handlers

import (
	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/fluffyriot/rpsync/internal/worker"
)

type Handler struct {
	DB      *database.Queries
	Fetcher *fetcher.Client
	Puller  *common.Client
	Config  *config.AppConfig
	Worker  *worker.Worker
}

func NewHandler(db *database.Queries, clientFetch *fetcher.Client, clientPull *common.Client, cfg *config.AppConfig, w *worker.Worker) *Handler {
	return &Handler{
		DB:      db,
		Fetcher: clientFetch,
		Puller:  clientPull,
		Config:  cfg,
		Worker:  w,
	}
}

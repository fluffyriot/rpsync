// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	fetcher_common "github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/google/uuid"
)

type Worker struct {
	DB               *database.Queries
	Fetcher          *fetcher_common.Client
	Puller           *common.Client
	Config           *config.AppConfig
	StopChan         chan bool
	mu               sync.Mutex
	active           bool
	activeManualSync bool
}

func NewWorker(db *database.Queries, fetcher *fetcher_common.Client, puller *common.Client, cfg *config.AppConfig) *Worker {
	return &Worker{
		DB:       db,
		Fetcher:  fetcher,
		Puller:   puller,
		Config:   cfg,
		StopChan: make(chan bool),
	}
}

func (w *Worker) Start() {
	w.mu.Lock()
	if w.active {
		w.mu.Unlock()
		log.Println("Worker: Scheduler already active.")
		return
	}
	w.active = true
	w.mu.Unlock()

	ctx := context.Background()
	users, err := w.DB.GetAllUsers(ctx)
	if err != nil {
		log.Printf("Worker: Failed to get users for scheduler: %v", err)
		w.mu.Lock()
		w.active = false
		w.mu.Unlock()
		return
	}

	go func() {
		for i, user := range users {
			select {
			case <-w.StopChan:
				return
			default:
			}

			if i > 0 {
				time.Sleep(10 * time.Second)
			}

			syncPeriod := 30 * time.Minute
			if user.SyncPeriod != "" {
				if d, err := time.ParseDuration(user.SyncPeriod); err == nil {
					syncPeriod = d
				}
			}

			log.Printf("Worker: Starting scheduler for user %s with period %v", user.Username, syncPeriod)

			go w.spawnUserWorker(user.ID, syncPeriod)
		}
	}()

	log.Println("Background worker system started")
}

func (w *Worker) spawnUserWorker(userID uuid.UUID, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			SyncUser(context.Background(), userID, w.DB, w.Fetcher, w.Puller, w.Config)
		case <-w.StopChan:
			return
		}
	}
}

func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.active {
		w.mu.Unlock()
		log.Println("Worker: Scheduler not active")
		return
	}
	w.active = false
	w.mu.Unlock()

	close(w.StopChan)
	log.Println("Background worker stopped")
}

func (w *Worker) Restart() {
	w.Stop()
	time.Sleep(100 * time.Millisecond)
	w.StopChan = make(chan bool)
	w.Start()
}

func (w *Worker) IsActive() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active
}

func (w *Worker) SyncSource(sid uuid.UUID) {
	w.mu.Lock()
	if w.activeManualSync {
		w.mu.Unlock()
		log.Println("Worker: Sync already in progress, skipping...")
		return
	}
	w.activeManualSync = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.activeManualSync = false
		w.mu.Unlock()
	}()

	RunSyncSource(sid, w.DB, w.Fetcher, w.Config)
}

func (w *Worker) SyncUserManual(userID uuid.UUID) {
	w.mu.Lock()
	if w.activeManualSync {
		w.mu.Unlock()
		log.Println("Worker: Sync already in progress, skipping...")
		return
	}
	w.activeManualSync = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.activeManualSync = false
		w.mu.Unlock()
	}()

	SyncUser(context.Background(), userID, w.DB, w.Fetcher, w.Puller, w.Config)
}

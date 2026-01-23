package worker

import (
	"log"
	"sync"
	"time"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/google/uuid"
)

type Worker struct {
	DB       *database.Queries
	Fetcher  *fetcher.Client
	Puller   *common.Client
	Config   *config.AppConfig
	Ticker   *time.Ticker
	StopChan chan bool
	mu       sync.Mutex
	running  bool
	active   bool
}

func NewWorker(db *database.Queries, fetcher *fetcher.Client, puller *common.Client, cfg *config.AppConfig) *Worker {
	return &Worker{
		DB:       db,
		Fetcher:  fetcher,
		Puller:   puller,
		Config:   cfg,
		StopChan: make(chan bool),
	}
}

func (w *Worker) Start(interval time.Duration) {
	w.mu.Lock()
	if w.active {
		w.mu.Unlock()
		log.Println("Worker: Scheduler already active, use Restart to change interval")
		return
	}
	w.active = true
	w.mu.Unlock()

	w.Ticker = time.NewTicker(interval)
	go func() {
		defer func() {
			w.mu.Lock()
			w.active = false
			w.mu.Unlock()
		}()
		for {
			select {
			case <-w.Ticker.C:
				w.SyncAll()
			case <-w.StopChan:
				w.Ticker.Stop()
				return
			}
		}
	}()
	log.Printf("Background worker started with interval: %v", interval)
}

func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.active {
		w.mu.Unlock()
		log.Println("Worker: Scheduler not active")
		return
	}
	w.mu.Unlock()

	w.StopChan <- true
	log.Println("Background worker stopped")
}

func (w *Worker) Restart(interval time.Duration) {
	w.mu.Lock()
	isActive := w.active
	w.mu.Unlock()

	if isActive {
		w.Stop()
		time.Sleep(100 * time.Millisecond)
	}
	w.Start(interval)
}

func (w *Worker) IsActive() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active
}

func (w *Worker) SyncAll() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		log.Println("Worker: Sync already in progress, skipping...")
		return
	}
	w.running = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	RunSync(w.DB, w.Fetcher, w.Puller, w.Config)
}

func (w *Worker) SyncSource(sid uuid.UUID) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		log.Println("Worker: Sync already in progress, skipping...")
		return
	}
	w.running = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	RunSyncSource(sid, w.DB, w.Fetcher, w.Config)
}

package worker

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"log"
	"sync"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/config"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/fluffyriot/commission-tracker/internal/fetcher"
	"github.com/fluffyriot/commission-tracker/internal/puller"
	"github.com/google/uuid"
)

type Worker struct {
	DB       *database.Queries
	Fetcher  *fetcher.Client
	Puller   *puller.Client
	Config   *config.AppConfig
	Ticker   *time.Ticker
	StopChan chan bool
}

func backoffWithJitter(attempt int) time.Duration {
	const (
		baseDelay = 10 * time.Second
		maxDelay  = 15 * time.Minute
	)

	delay := baseDelay * (1 << attempt)
	if delay > maxDelay {
		delay = maxDelay
	}

	var b [8]byte
	_, _ = rand.Read(b[:])
	jitter := time.Duration(binary.LittleEndian.Uint64(b[:]) % uint64(delay))

	return jitter
}

func NewWorker(db *database.Queries, fetcher *fetcher.Client, puller *puller.Client, cfg *config.AppConfig) *Worker {
	return &Worker{
		DB:       db,
		Fetcher:  fetcher,
		Puller:   puller,
		Config:   cfg,
		StopChan: make(chan bool),
	}
}

func (w *Worker) Start(interval time.Duration) {
	w.Ticker = time.NewTicker(interval)
	go func() {
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
	w.StopChan <- true
	log.Println("Background worker stopped")
}

func (w *Worker) SyncAll() {
	log.Println("Worker: Starting scheduled sync...")
	ctx := context.Background()

	err := w.DB.DeleteOldStats(ctx)
	if err != nil {
		log.Printf("Worker Data deletion error: %v", err)
	}

	users, err := w.DB.GetAllUsers(ctx)
	if err != nil {
		log.Printf("Worker Error getting users: %v", err)
		return
	}

	var (
		sourceWG    sync.WaitGroup
		targetWG    sync.WaitGroup
		countSource int
		countTarget int
	)

	for _, user := range users {
		sources, err := w.DB.GetUserActiveSources(ctx, user.ID)
		if err != nil {
			log.Printf("Worker Error getting sources for user %s: %v", user.Username, err)
			continue
		}

		for _, source := range sources {
			sourceWG.Add(1)
			countSource++

			go func(sid uuid.UUID) {
				defer sourceWG.Done()

				const maxRetries = 5

				for attempt := 0; attempt <= maxRetries; attempt++ {

					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("Worker Panic in source sync (source=%s attempt=%d): %v", sid, attempt+1, r)
							}
						}()

						err := fetcher.SyncBySource(sid, w.DB, w.Fetcher, w.Config.InstagramAPIVersion, w.Config.TokenEncryptionKey)

						if err == nil {
							return
						}

						if attempt == maxRetries {
							log.Printf("Worker Source sync FAILED after %d attempts (source=%s): %v", attempt+1, sid, err)
							return
						}

						delay := backoffWithJitter(attempt)
						log.Printf("Worker Source sync error (source=%s attempt=%d). Retrying in %s: %v", sid, attempt+1, delay, err)
						time.Sleep(delay)
					}()
				}
			}(source.ID)
		}
	}

	sourceWG.Wait()

	for _, user := range users {
		targets, err := w.DB.GetUserActiveTargets(ctx, user.ID)
		if err != nil {
			log.Printf("Worker Error getting targets for user %s: %v", user.Username, err)
			continue
		}

		for _, target := range targets {
			targetWG.Add(1)
			countTarget++

			go func(tid uuid.UUID) {
				defer targetWG.Done()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Worker Panic in target sync: %v", r)
					}
				}()

				puller.PullByTarget(tid, w.DB, w.Puller, w.Config.TokenEncryptionKey)
			}(target.ID)
		}
	}

	targetWG.Wait()

	log.Printf(
		"Worker: Completed sync for %d sources and %d targets",
		countSource,
		countTarget,
	)
}

package worker

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"log"
	"sync"
	"time"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher"
	fetcher_common "github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/fluffyriot/rpsync/internal/pusher"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/google/uuid"
)

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

func SyncUser(ctx context.Context, userID uuid.UUID, db *database.Queries, f *fetcher_common.Client, p *common.Client, cfg *config.AppConfig) {
	var (
		sourceWG    sync.WaitGroup
		targetWG    sync.WaitGroup
		countSource int
		countTarget int
	)

	visitedSources := make(map[uuid.UUID]bool)

	sources, err := db.GetUserActiveSources(ctx, userID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Worker Error getting sources for user %s: %v", userID, err)
		}
	} else {
		for _, source := range sources {
			if visitedSources[source.ID] {
				continue
			}
			visitedSources[source.ID] = true

			sourceWG.Add(1)
			countSource++

			go func(sid uuid.UUID) {
				defer sourceWG.Done()
				syncSourceInternal(sid, db, f, cfg)
			}(source.ID)
		}
	}

	sourceWG.Wait()

	targets, err := db.GetUserActiveTargets(ctx, userID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Worker Error getting targets for user %s: %v", userID, err)
		}
	} else {
		for _, target := range targets {
			targetWG.Add(1)
			countTarget++

			go func(tid uuid.UUID) {
				defer targetWG.Done()
				syncTargetInternal(tid, db, p, cfg)
			}(target.ID)
		}
	}

	targetWG.Wait()

	log.Printf(
		"Worker: Completed sync for user %s (sources=%d targets=%d)",
		userID,
		countSource,
		countTarget,
	)
}

func RunSyncSource(sid uuid.UUID, db *database.Queries, f *fetcher_common.Client, cfg *config.AppConfig) {
	log.Printf("Worker: Starting manual sync for source %s", sid)
	syncSourceInternal(sid, db, f, cfg)
}

func syncSourceInternal(sid uuid.UUID, db *database.Queries, f *fetcher_common.Client, cfg *config.AppConfig) {
	const maxRetries = 5

	for attempt := 0; attempt <= maxRetries; attempt++ {
		isLastRetry := attempt == maxRetries

		err := func() error {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Worker Panic in source sync (source=%s attempt=%d): %v", sid, attempt+1, r)
				}
			}()

			err := fetcher.SyncBySource(sid, db, f, cfg.InstagramAPIVersion, cfg.TokenEncryptionKey, isLastRetry)

			if err == nil {
				return nil
			}

			if isLastRetry {
				log.Printf("Worker Source sync FAILED after %d attempts (source=%s): %v", attempt+1, sid, err)
				return err
			}

			delay := backoffWithJitter(attempt)
			log.Printf("Worker Source sync error (source=%s attempt=%d). Retrying in %s: %v", sid, attempt+1, delay, err)
			time.Sleep(delay)
			return err
		}()

		if err == nil {
			return
		}
	}
}

func syncTargetInternal(tid uuid.UUID, db *database.Queries, p *common.Client, cfg *config.AppConfig) {
	const maxRetries = 5

	for attempt := 0; attempt <= maxRetries; attempt++ {
		isLastRetry := attempt == maxRetries

		err := func() error {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Worker Panic in target sync (target=%s attempt=%d): %v", tid, attempt+1, r)
				}
			}()

			err := pusher.PullByTarget(tid, db, p, cfg.TokenEncryptionKey, isLastRetry)

			if err == nil {
				return nil
			}

			if isLastRetry {
				log.Printf("Worker Target sync FAILED after %d attempts (target=%s): %v", attempt+1, tid, err)
				return err
			}

			delay := backoffWithJitter(attempt)
			log.Printf("Worker Target sync error (target=%s attempt=%d). Retrying in %s: %v", tid, attempt+1, delay, err)
			time.Sleep(delay)
			return err
		}()

		if err == nil {
			return
		}
	}
}

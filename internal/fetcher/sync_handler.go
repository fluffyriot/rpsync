// SPDX-License-Identifier: AGPL-3.0-only
package fetcher

import (
	"context"
	"database/sql"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/fluffyriot/rpsync/internal/fetcher/sources"
	"github.com/google/uuid"
)

func executeSync(
	ctx context.Context,
	dbQueries *database.Queries,
	sourceID uuid.UUID,
	syncFunc func() error,
	isLastRetry bool,
) error {
	syncStartTime := time.Now()

	_, err := dbQueries.UpdateSourceSyncStatusById(ctx, database.UpdateSourceSyncStatusByIdParams{
		ID:         sourceID,
		SyncStatus: "Syncing",
	})
	if err != nil {
		return err
	}

	err = syncFunc()
	if err != nil {
		_, _ = dbQueries.UpdateSourceSyncStatusById(ctx, database.UpdateSourceSyncStatusByIdParams{
			ID:           sourceID,
			SyncStatus:   "Failed",
			StatusReason: sql.NullString{String: err.Error(), Valid: true},
			LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
		})
		if isLastRetry {
			_, _ = dbQueries.CreateLog(ctx, database.CreateLogParams{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				SourceID:  uuid.NullUUID{UUID: sourceID, Valid: true},
				Message:   err.Error(),
			})
		}
		return err
	}

	if err := dbQueries.ArchiveUnsyncedPosts(ctx, database.ArchiveUnsyncedPostsParams{
		SourceID:     sourceID,
		LastSyncedAt: syncStartTime.Add(-36 * time.Hour),
	}); err != nil {
		return err
	}

	_, err = dbQueries.UpdateSourceSyncStatusById(ctx, database.UpdateSourceSyncStatusByIdParams{
		ID:           sourceID,
		SyncStatus:   "Synced",
		StatusReason: sql.NullString{},
		LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
	})

	return err
}

func SyncBySource(sid uuid.UUID, dbQueries *database.Queries, c *common.Client, ver string, encryptionKey []byte, isLastRetry bool) error {

	source, err := dbQueries.GetSourceById(context.Background(), sid)
	if err != nil {
		return err
	}

	return executeSync(context.Background(), dbQueries, source.ID, func() error {
		switch source.Network {
		case "Bluesky":
			return sources.FetchBlueskyPosts(dbQueries, c, source.UserID, source.ID)

		case "Instagram":
			if err := sources.FetchInstagramTags(dbQueries, c, source.ID, ver, encryptionKey); err != nil {
				return err
			}
			return sources.FetchInstagramPosts(dbQueries, c, source.ID, ver, encryptionKey)

		case "Murrtube":
			return sources.FetchMurrtubePosts(source.UserID, dbQueries, c, source.ID)

		case "BadPups":
			return sources.FetchBadpupsPosts(source.UserID, dbQueries, c, source.ID)

		case "TikTok":
			return sources.FetchTikTokPosts(dbQueries, c, source.UserID, source.ID)

		case "Mastodon":
			return sources.FetchMastodonPosts(dbQueries, c, source.UserID, source.ID)

		case "Telegram":
			return sources.FetchTelegramPosts(dbQueries, encryptionKey, source.ID, c)

		case "Google Analytics":
			return sources.FetchGoogleAnalyticsStats(dbQueries, source.ID, encryptionKey)

		case "YouTube":
			return sources.FetchYouTubePosts(dbQueries, source.ID, encryptionKey)

		case "FurTrack":
			return sources.FetchFurTrackPosts(dbQueries, c, source.UserID, source.ID)

		case "Discord":
			return sources.FetchDiscordPosts(dbQueries, encryptionKey, source.ID, c)

		default:
			return nil
		}
	}, isLastRetry)
}

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
			if err := sources.FetchInstagramCollabs(dbQueries, c, source.ID, ver, encryptionKey); err != nil {
				return err
			}
			return sources.FetchInstagramPosts(dbQueries, c, source.ID, ver, encryptionKey)

		case "Murrtube":
			return sources.FetchMurrtubePosts(dbQueries, c, source.ID, encryptionKey)

		case "BadPups":
			return sources.FetchBadpupsPosts(dbQueries, c, source.ID, encryptionKey)

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

		case "Reddit":
			return sources.FetchRedditPosts(dbQueries, encryptionKey, source.ID, c)

		case "Twitch":
			return sources.FetchTwitchPosts(dbQueries, encryptionKey, source.ID, c)

		case "FurAffinity":
			return sources.FetchFurAffinityPosts(dbQueries, c, source.UserID, source.ID)

		case "e621":
			return sources.FetchE621Posts(dbQueries, encryptionKey, source.ID, c)

		case "DeviantArt":
			return sources.FetchDeviantArtPosts(dbQueries, encryptionKey, source.ID, c)

		case "Weasyl":
			return sources.FetchWeasylPosts(dbQueries, encryptionKey, source.ID, c)

		case "Google Search Console":
			return sources.FetchGoogleSearchConsoleStats(dbQueries, source.ID, encryptionKey)

		case "Threads":
			return sources.FetchThreadsPosts(dbQueries, encryptionKey, source.ID, c)

		case "Twitter":
			return sources.FetchTwitterPosts(dbQueries, c, source.UserName, source.ID, encryptionKey)

		default:
			return nil
		}
	}, isLastRetry)
}

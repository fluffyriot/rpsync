package fetcher

import (
	"context"
	"database/sql"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/google/uuid"
)

func executeSync(
	ctx context.Context,
	dbQueries *database.Queries,
	sourceID uuid.UUID,
	syncFunc func() error,
	isLastRetry bool,
) error {
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
	_, err = dbQueries.UpdateSourceSyncStatusById(ctx, database.UpdateSourceSyncStatusByIdParams{
		ID:           sourceID,
		SyncStatus:   "Synced",
		StatusReason: sql.NullString{},
		LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
	})

	return err
}

func SyncBySource(sid uuid.UUID, dbQueries *database.Queries, c *Client, ver string, encryptionKey []byte, isLastRetry bool) error {

	source, err := dbQueries.GetSourceById(context.Background(), sid)
	if err != nil {
		return err
	}

	return executeSync(context.Background(), dbQueries, source.ID, func() error {
		switch source.Network {
		case "Bluesky":
			return FetchBlueskyPosts(dbQueries, c, source.UserID, source.ID)

		case "Instagram":
			if err := FetchInstagramPosts(dbQueries, c, source.ID, ver, encryptionKey); err != nil {
				return err
			}
			return FetchInstagramTags(dbQueries, c, source.ID, ver, encryptionKey)

		case "Murrtube":
			return FetchMurrtubePosts(source.UserID, dbQueries, c, source.ID)

		case "BadPups":
			return FetchBadpupsPosts(source.UserID, dbQueries, c, source.ID)

		case "TikTok":
			return FetchTikTokPosts(dbQueries, c, source.UserID, source.ID)

		case "Mastodon":
			return FetchMastodonPosts(dbQueries, c, source.UserID, source.ID)

		case "Telegram":
			return FetchTelegramPosts(dbQueries, encryptionKey, source.ID, c)

		case "Google Analytics":
			return FetchGoogleAnalyticsStats(dbQueries, source.ID, encryptionKey)

		case "YouTube":
			return FetchYouTubePosts(dbQueries, source.ID, encryptionKey)

		case "FurTrack":
			return FetchFurTrackPosts(dbQueries, c, source.UserID, source.ID)

		default:
			return nil
		}
	}, isLastRetry)
}

package fetcher

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: http.Client{
			Timeout: timeout,
		},
	}
}

func SyncBySource(sid uuid.UUID, dbQueries *database.Queries, c *Client, ver string, encryptionKey []byte) error {

	source, err := dbQueries.GetSourceById(context.Background(), sid)
	if err != nil {
		return err
	}

	_, err = dbQueries.UpdateSourceSyncStatusById(context.Background(), database.UpdateSourceSyncStatusByIdParams{
		ID:         source.ID,
		SyncStatus: "Syncing",
	})
	if err != nil {
		return err
	}

	switch source.Network {
	case "Bluesky":
		err = FetchBlueskyPosts(dbQueries, c, source.UserID, source.ID)
		if err != nil {
			_, err = dbQueries.UpdateSourceSyncStatusById(context.Background(), database.UpdateSourceSyncStatusByIdParams{
				ID:           source.ID,
				SyncStatus:   "Failed",
				StatusReason: sql.NullString{String: err.Error(), Valid: true},
				LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
			})
			if err != nil {
				return err
			}
			return err
		}
	case "Instagram":
		err = FetchInstagramPosts(dbQueries, c, source.ID, ver, encryptionKey)
		if err != nil {
			_, err = dbQueries.UpdateSourceSyncStatusById(context.Background(), database.UpdateSourceSyncStatusByIdParams{
				ID:           source.ID,
				SyncStatus:   "Failed",
				StatusReason: sql.NullString{String: err.Error(), Valid: true},
				LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
			})
			if err != nil {
				return err
			}
			return err
		}
	case "Murrtube":
		err = FetchMurrtubePosts(source.UserID, dbQueries, c, source.ID)
		if err != nil {
			_, err = dbQueries.UpdateSourceSyncStatusById(context.Background(), database.UpdateSourceSyncStatusByIdParams{
				ID:           source.ID,
				SyncStatus:   "Failed",
				StatusReason: sql.NullString{String: err.Error(), Valid: true},
				LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
			})
			if err != nil {
				return err
			}
			return err
		}
	case "BadPups":
		err = FetchBadpupsPosts(source.UserID, dbQueries, c, source.ID)
		if err != nil {
			_, err = dbQueries.UpdateSourceSyncStatusById(context.Background(), database.UpdateSourceSyncStatusByIdParams{
				ID:           source.ID,
				SyncStatus:   "Failed",
				StatusReason: sql.NullString{String: err.Error(), Valid: true},
				LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
			})
			if err != nil {
				return err
			}
			return err
		}
	}

	_, err = dbQueries.UpdateSourceSyncStatusById(context.Background(), database.UpdateSourceSyncStatusByIdParams{
		ID:           source.ID,
		SyncStatus:   "Synced",
		StatusReason: sql.NullString{},
		LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
	})

	return nil
}

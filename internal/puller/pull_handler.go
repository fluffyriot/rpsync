package puller

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

func PullByTarget(tid uuid.UUID, dbQueries *database.Queries, c *Client, encryptionKey []byte) error {

	target, err := dbQueries.GetTargetById(context.Background(), tid)
	if err != nil {
		return err
	}

	_, err = dbQueries.UpdateTargetSyncStatusById(context.Background(), database.UpdateTargetSyncStatusByIdParams{
		ID:         target.ID,
		SyncStatus: "Syncing",
	})
	if err != nil {
		return err
	}

	switch target.TargetType {

	case "NocoDB", "Notion":

		err = startDbSync(dbQueries, c, target.ID, encryptionKey, target)
		if err != nil {
			_, err = dbQueries.UpdateTargetSyncStatusById(context.Background(), database.UpdateTargetSyncStatusByIdParams{
				ID:           target.ID,
				SyncStatus:   "Failed",
				StatusReason: sql.NullString{String: err.Error(), Valid: true},
				LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
			})
			if err != nil {
				return err
			}
			return err
		}

	case "CSV":

		err = startFileSync(dbQueries, target.ID)
		if err != nil {
			_, err = dbQueries.UpdateSourceSyncStatusById(context.Background(), database.UpdateSourceSyncStatusByIdParams{
				ID:           target.ID,
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

	_, err = dbQueries.UpdateTargetSyncStatusById(context.Background(), database.UpdateTargetSyncStatusByIdParams{
		ID:           target.ID,
		SyncStatus:   "Synced",
		StatusReason: sql.NullString{},
		LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
	})

	return nil
}

func startDbSync(dbQueries *database.Queries, c *Client, targetId uuid.UUID, encryptionKey []byte, target database.Target) error {
	if target.TargetType == "Notion" {
		return fmt.Errorf("not implemented yet")
	}

	err := InitializeNoco(targetId, dbQueries, c, encryptionKey, target)
	return err

}

func startFileSync(dbQueries *database.Queries, targetId uuid.UUID) error {
	return fmt.Errorf("not implemented yet")
}

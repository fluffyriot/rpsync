package puller

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/fluffyriot/commission-tracker/internal/exports"
	"github.com/google/uuid"
)

func RemoveByTarget(tid, sid uuid.UUID, dbQueries *database.Queries, c *Client, encryptionKey []byte) error {

	target, err := dbQueries.GetTargetById(context.Background(), tid)
	if err != nil {
		return err
	}

	source, err := dbQueries.GetSourceById(context.Background(), sid)
	if err != nil {
		return err
	}

	err = startDbRemoval(dbQueries, c, target.ID, encryptionKey, target, source)
	if err != nil {
		return err
	}

	return nil
}

func PullByTarget(tid uuid.UUID, dbQueries *database.Queries, c *Client, encryptionKey []byte) error {

	target, err := dbQueries.GetTargetById(context.Background(), tid)
	if err != nil {
		return err
	}

	export, err := exports.CreateLogAutoExport(target.UserID, dbQueries, target.TargetType, target.ID)
	if err != nil {
		log.Println(err)
	}

	_, err = dbQueries.UpdateTargetSyncStatusById(context.Background(), database.UpdateTargetSyncStatusByIdParams{
		ID:         target.ID,
		SyncStatus: "Syncing",
	})
	if err != nil {
		return err
	}

	var filename string

	switch target.TargetType {

	case "NocoDB", "Notion":

		err = startDbSync(dbQueries, c, encryptionKey, target)
		if err != nil {

			exports.UpdateLogAutoExport(export, dbQueries, "Failed", err.Error(), filename)

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

		filename, err = startCsvSync(dbQueries, target, export)
		if err != nil {

			exports.UpdateLogAutoExport(export, dbQueries, "Failed", err.Error(), filename)

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

	}

	_, err = dbQueries.UpdateTargetSyncStatusById(context.Background(), database.UpdateTargetSyncStatusByIdParams{
		ID:           target.ID,
		SyncStatus:   "Synced",
		StatusReason: sql.NullString{},
		LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
	})

	exports.UpdateLogAutoExport(export, dbQueries, "Completed", "", filename)

	return nil
}

func startDbSync(dbQueries *database.Queries, c *Client, encryptionKey []byte, target database.Target) error {

	if target.TargetType == "Notion" {
		return fmt.Errorf("not implemented yet")
	}

	_, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "Sources",
	})
	if err != nil {
		err := InitializeNoco(dbQueries, c, encryptionKey, target)
		if err != nil {
			return err
		}
	}

	err = SyncNoco(dbQueries, c, encryptionKey, target)
	return err

}

func startDbRemoval(dbQueries *database.Queries, c *Client, targetId uuid.UUID, encryptionKey []byte, target database.Target, source database.Source) error {
	if target.TargetType == "Notion" || target.TargetType == "CSV" {
		return nil
	}

	err := DeletePostsAndSourceNoco(dbQueries, c, encryptionKey, target, source)
	return err
}

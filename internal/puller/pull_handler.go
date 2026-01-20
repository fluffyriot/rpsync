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

	_, err = dbQueries.UpdateTargetSyncStatusById(context.Background(), database.UpdateTargetSyncStatusByIdParams{
		ID:         target.ID,
		SyncStatus: "Syncing",
	})
	if err != nil {
		return err
	}

	var finalErr error

	switch target.TargetType {

	case "NocoDB", "Notion":

		export, err := exports.CreateLogAutoExport(target.UserID, dbQueries, target.TargetType, target.ID)
		if err != nil {
			log.Println("Error creating export log:", err)
		}

		err = startDbSync(dbQueries, c, encryptionKey, target)
		if err != nil {
			exports.UpdateLogAutoExport(export, dbQueries, "Failed", err.Error(), "")
			finalErr = err
		} else {
			exports.UpdateLogAutoExport(export, dbQueries, "Completed", "", "")
		}

	case "CSV":

		hasPosts, err := HasPosts(dbQueries, target.UserID)
		if err != nil {
			finalErr = err
		} else if hasPosts {
			exportPosts, err := exports.CreateLogAutoExport(target.UserID, dbQueries, "CSV - Posts", target.ID)
			if err != nil {
				log.Println("Error creating posts export log:", err)
			} else {
				filename, err := GeneratePostsCsv(dbQueries, target, exportPosts)
				if err != nil {
					exports.UpdateLogAutoExport(exportPosts, dbQueries, "Failed", err.Error(), filename)
					finalErr = err // Track error but continue to next export?
				} else {
					exports.UpdateLogAutoExport(exportPosts, dbQueries, "Completed", "", filename)
				}
			}
		}

		hasAnalytics, err := HasAnalytics(dbQueries, target.UserID)
		if err != nil {
			if finalErr == nil {
				finalErr = err
			}
		} else if hasAnalytics {
			exportWeb, err := exports.CreateLogAutoExport(target.UserID, dbQueries, "CSV - Website", target.ID)
			if err != nil {
				log.Println("Error creating website export log:", err)
			} else {
				filename, err := GenerateWebsiteCsv(dbQueries, target, exportWeb)
				if err != nil {
					exports.UpdateLogAutoExport(exportWeb, dbQueries, "Failed", err.Error(), filename)
					if finalErr == nil {
						finalErr = err
					}
				} else {
					exports.UpdateLogAutoExport(exportWeb, dbQueries, "Completed", "", filename)
				}
			}
		}
	}

	status := "Synced"
	var reason sql.NullString
	if finalErr != nil {
		status = "Failed"
		reason = sql.NullString{String: finalErr.Error(), Valid: true}
	}

	_, err = dbQueries.UpdateTargetSyncStatusById(context.Background(), database.UpdateTargetSyncStatusByIdParams{
		ID:           target.ID,
		SyncStatus:   status,
		StatusReason: reason,
		LastSynced:   sql.NullTime{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return err
	}

	return finalErr
}

func startDbSync(dbQueries *database.Queries, c *Client, encryptionKey []byte, target database.Target) error {

	if target.TargetType == "Notion" {
		return fmt.Errorf("not implemented yet")
	}

	_, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "Analytics_Page_Stats",
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

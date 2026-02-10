// SPDX-License-Identifier: AGPL-3.0-only
package noco

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/google/uuid"
)

func syncNocoSourcesStats(dbQueries *database.Queries, c *common.Client, encryptionKey []byte, target database.Target) error {
	const batchSize = 10

	tableMapping, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "sources_stats",
	})
	if err != nil {
		return nil
	}

	sourcesTableMapping, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "sources",
	})
	if err != nil {
		return fmt.Errorf("failed to get sources table mapping: %w", err)
	}

	sources, err := dbQueries.GetUserSources(context.Background(), target.UserID)
	if err != nil {
		return err
	}

	dateThreshold := time.Now().AddDate(0, 0, -2)

	for _, source := range sources {
		sourceMapping, err := dbQueries.GetTargetSourceBySource(context.Background(), database.GetTargetSourceBySourceParams{
			TargetID: target.ID,
			SourceID: source.ID,
		})
		if err != nil {
			continue
		}

		syncedStats, err := dbQueries.GetSyncedSourcesStatsForUpdate(context.Background(), database.GetSyncedSourcesStatsForUpdateParams{
			TargetID: target.ID,
			SourceID: source.ID,
			Date:     dateThreshold,
		})
		if err != nil {
			return err
		}

		var updateRecords []NocoTableRecord

		flushUpdate := func() error {
			if len(updateRecords) == 0 {
				return nil
			}
			if err := updateNocoRecords(c, dbQueries, encryptionKey, target, tableMapping.TargetTableCode.String, updateRecords); err != nil {
				return err
			}
			updateRecords = updateRecords[:0]
			return nil
		}

		for _, stat := range syncedStats {
			targetIDVal, _ := strconv.Atoi(stat.TargetRecordID)
			safeTargetID := targetIDVal

			fieldMap := NocoRecordFields{
				ID:             stat.ID.String(),
				Date:           stat.Date,
				FollowersCount: int(stat.FollowersCount.Int64),
				FollowingCount: int(stat.FollowingCount.Int64),
				PostsCount:     int(stat.PostsCount.Int64),
				AverageLikes:   stat.AverageLikes.Float64,
				AverageReposts: stat.AverageReposts.Float64,
				AverageViews:   stat.AverageViews.Float64,
			}
			updateRecords = append(updateRecords, NocoTableRecord{
				Id:     safeTargetID,
				Fields: fieldMap,
			})
			if len(updateRecords) == batchSize {
				if err := flushUpdate(); err != nil {
					return err
				}
			}
		}
		if err := flushUpdate(); err != nil {
			return err
		}

		// Step 2: Create unsynced stats (all dates)
		unsyncedStats, err := dbQueries.GetUnsyncedSourcesStatsForTarget(context.Background(), database.GetUnsyncedSourcesStatsForTargetParams{
			SourceID: source.ID,
			TargetID: target.ID,
		})
		if err != nil {
			return err
		}

		var records []NocoTableRecord
		var currentBatch []database.SourcesStat

		flushCreate := func() error {
			if len(records) == 0 {
				return nil
			}
			createdRecords, err := createNocoRecords(c, dbQueries, encryptionKey, target, tableMapping.TargetTableCode.String, records)
			if err != nil {
				return err
			}

			var createdIds []int

			for i, rec := range createdRecords {
				var id float64
				if val, ok := rec["Id"].(float64); ok {
					id = val
				} else if val, ok := rec["id"].(float64); ok {
					id = val
				} else {
					continue
				}

				originalStat := currentBatch[i]

				_, err = dbQueries.AddSourcesStatToTarget(context.Background(), database.AddSourcesStatToTargetParams{
					ID:             uuid.New(),
					SyncedAt:       time.Now(),
					StatID:         originalStat.ID,
					TargetID:       target.ID,
					TargetRecordID: fmt.Sprintf("%.0f", id),
				})
				if err != nil {
					return fmt.Errorf("failed to map sources stat: %w", err)
				}

				createdIds = append(createdIds, int(id))
			}

			sourceNocoId, _ := strconv.Atoi(sourceMapping.TargetSourceID)
			safeSourceNocoId := sourceNocoId

			if err := linkChildrenToParent(c, dbQueries, encryptionKey, target, sourcesTableMapping, "sources_stats", safeSourceNocoId, createdIds); err != nil {
				log.Printf("Failed to link sources stats to source: %v", err)
			}

			records = records[:0]
			currentBatch = currentBatch[:0]
			return nil
		}

		for _, stat := range unsyncedStats {
			fieldMap := NocoRecordFields{
				ID:             stat.ID.String(),
				Date:           stat.Date,
				FollowersCount: int(stat.FollowersCount.Int64),
				FollowingCount: int(stat.FollowingCount.Int64),
				PostsCount:     int(stat.PostsCount.Int64),
				AverageLikes:   stat.AverageLikes.Float64,
				AverageReposts: stat.AverageReposts.Float64,
				AverageViews:   stat.AverageViews.Float64,
			}

			records = append(records, NocoTableRecord{
				Fields: fieldMap,
			})
			currentBatch = append(currentBatch, stat)

			if len(records) == batchSize {
				if err := flushCreate(); err != nil {
					return err
				}
			}
		}
		if err := flushCreate(); err != nil {
			return err
		}

	}
	return nil
}

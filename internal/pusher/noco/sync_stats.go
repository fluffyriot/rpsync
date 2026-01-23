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

	for _, source := range sources {
		unsyncedStats, err := dbQueries.GetUnsyncedSourcesStatsForTarget(context.Background(), database.GetUnsyncedSourcesStatsForTargetParams{
			SourceID: source.ID,
			TargetID: target.ID,
		})

		if err != nil {
			return err
		}

		sourceMapping, err := dbQueries.GetTargetSourceBySource(context.Background(), database.GetTargetSourceBySourceParams{
			TargetID: target.ID,
			SourceID: source.ID,
		})
		if err != nil {
			continue
		}

		var records []NocoTableRecord
		var currentBatch []database.SourcesStat

		flush := func() error {
			if len(records) == 0 {
				return nil
			}
			createdRecords, err := createNocoRecords(c, dbQueries, encryptionKey, target, tableMapping.TargetTableCode.String, records)
			if err != nil {
				return err
			}

			var createdIds []int32

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

				createdIds = append(createdIds, int32(id))
			}

			sourceNocoId, _ := strconv.Atoi(sourceMapping.TargetSourceID)

			if err := linkChildrenToParent(c, dbQueries, encryptionKey, target, sourcesTableMapping, "sources_stats", int32(sourceNocoId), createdIds); err != nil {
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
				FollowersCount: int32(stat.FollowersCount.Int32),
				FollowingCount: int32(stat.FollowingCount.Int32),
				PostsCount:     int32(stat.PostsCount.Int32),
				AverageLikes:   stat.AverageLikes.Float64,
				AverageReposts: stat.AverageReposts.Float64,
				AverageViews:   stat.AverageViews.Float64,
			}

			records = append(records, NocoTableRecord{
				Fields: fieldMap,
			})
			currentBatch = append(currentBatch, stat)

			if len(records) == batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
		if err := flush(); err != nil {
			return err
		}
	}
	return nil
}

package noco

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/fluffyriot/commission-tracker/internal/pusher/common"
	"github.com/google/uuid"
)

func syncNocoAnalyticsSiteStats(dbQueries *database.Queries, c *common.Client, encryptionKey []byte, target database.Target) error {
	const batchSize = 10

	tableMapping, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "analytics_site_stats",
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
		unsyncedStats, err := dbQueries.GetUnsyncedSiteStatsForTarget(context.Background(), database.GetUnsyncedSiteStatsForTargetParams{
			TargetID: target.ID,
			SourceID: source.ID,
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
		var currentBatch []database.AnalyticsSiteStat

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

				_, err = dbQueries.AddAnalyticsSiteStatToTarget(context.Background(), database.AddAnalyticsSiteStatToTargetParams{
					ID:             uuid.New(),
					SyncedAt:       time.Now(),
					StatID:         originalStat.ID,
					TargetID:       target.ID,
					TargetRecordID: fmt.Sprintf("%.0f", id),
				})
				if err != nil {
					return fmt.Errorf("failed to map site stat: %w", err)
				}

				createdIds = append(createdIds, int32(id))
			}

			sourceNocoId, _ := strconv.Atoi(sourceMapping.TargetSourceID)

			if err := linkChildrenToParent(c, dbQueries, encryptionKey, target, sourcesTableMapping, "site_stats", int32(sourceNocoId), createdIds); err != nil {
				log.Printf("Failed to link site stats to source: %v", err)
			}

			records = records[:0]
			currentBatch = currentBatch[:0]
			return nil
		}

		for _, stat := range unsyncedStats {
			fieldMap := NocoRecordFields{
				ID:                 stat.ID.String(),
				Date:               stat.Date,
				Visitors:           stat.Visitors,
				AvgSessionDuration: stat.AvgSessionDuration,
			}

			records = append(records, NocoTableRecord{
				Fields: fieldMap,
			})
			currentBatch = append(currentBatch, database.AnalyticsSiteStat{
				ID:                 stat.ID,
				Date:               stat.Date,
				Visitors:           stat.Visitors,
				AvgSessionDuration: stat.AvgSessionDuration,
				SourceID:           stat.SourceID,
			})

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

func syncNocoAnalyticsPageStats(dbQueries *database.Queries, c *common.Client, encryptionKey []byte, target database.Target) error {
	const batchSize = 10

	tableMapping, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "analytics_page_stats",
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
		unsyncedStats, err := dbQueries.GetUnsyncedPageStatsForTarget(context.Background(), database.GetUnsyncedPageStatsForTargetParams{
			TargetID: target.ID,
			SourceID: source.ID,
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
		var currentBatch []database.AnalyticsPageStat

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

				_, err = dbQueries.AddAnalyticsPageStatToTarget(context.Background(), database.AddAnalyticsPageStatToTargetParams{
					ID:             uuid.New(),
					SyncedAt:       time.Now(),
					StatID:         originalStat.ID,
					TargetID:       target.ID,
					TargetRecordID: fmt.Sprintf("%.0f", id),
				})
				if err != nil {
					return fmt.Errorf("failed to map page stat: %w", err)
				}

				createdIds = append(createdIds, int32(id))
			}

			sourceNocoId, _ := strconv.Atoi(sourceMapping.TargetSourceID)

			if err := linkChildrenToParent(c, dbQueries, encryptionKey, target, sourcesTableMapping, "page_stats", int32(sourceNocoId), createdIds); err != nil {
				log.Printf("Failed to link page stats to source: %v", err)
			}

			records = records[:0]
			currentBatch = currentBatch[:0]
			return nil
		}

		for _, stat := range unsyncedStats {
			fieldMap := NocoRecordFields{
				ID:       stat.ID.String(),
				Date:     stat.Date,
				PagePath: stat.UrlPath,
				Views:    stat.Views,
			}

			records = append(records, NocoTableRecord{
				Fields: fieldMap,
			})
			currentBatch = append(currentBatch, database.AnalyticsPageStat{
				ID:       stat.ID,
				Date:     stat.Date,
				UrlPath:  stat.UrlPath,
				Views:    stat.Views,
				SourceID: stat.SourceID,
			})

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

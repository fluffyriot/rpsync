package noco

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
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

	dateThreshold := time.Now().AddDate(0, 0, -9)

	for _, source := range sources {
		sourceMapping, err := dbQueries.GetTargetSourceBySource(context.Background(), database.GetTargetSourceBySourceParams{
			TargetID: target.ID,
			SourceID: source.ID,
		})
		if err != nil {
			continue
		}

		syncedStats, err := dbQueries.GetSyncedSiteStatsForUpdate(context.Background(), database.GetSyncedSiteStatsForUpdateParams{
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
			safeTargetID, err := helpers.ToInt32(targetIDVal)
			if err != nil {
				continue
			}

			fieldMap := NocoRecordFields{
				ID:                 stat.ID.String(),
				Date:               stat.Date,
				Visitors:           stat.Visitors,
				AvgSessionDuration: stat.AvgSessionDuration,
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

		unsyncedStats, err := dbQueries.GetUnsyncedSiteStatsForTarget(context.Background(), database.GetUnsyncedSiteStatsForTargetParams{
			TargetID: target.ID,
			SourceID: source.ID,
		})
		if err != nil {
			return err
		}

		var records []NocoTableRecord
		var currentBatch []database.AnalyticsSiteStat

		flushCreate := func() error {
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
					StatID:         uuid.NullUUID{UUID: originalStat.ID, Valid: true},
					TargetID:       target.ID,
					TargetRecordID: fmt.Sprintf("%.0f", id),
				})
				if err != nil {
					return fmt.Errorf("failed to map site stat: %w", err)
				}

				createdIds = append(createdIds, int32(id))
			}

			sourceNocoId, _ := strconv.Atoi(sourceMapping.TargetSourceID)
			safeSourceNocoId, err := helpers.ToInt32(sourceNocoId)
			if err != nil {
				log.Printf("Invalid source Noco ID: %v", err)
				return err
			}

			if err := linkChildrenToParent(c, dbQueries, encryptionKey, target, sourcesTableMapping, "site_stats", safeSourceNocoId, createdIds); err != nil {
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

	mappings, err := dbQueries.GetPageStatsOnTarget(context.Background(), target.ID)
	if err != nil {
		return err
	}
	mappingMap := make(map[string]database.AnalyticsPageStatsOnTarget)
	for _, m := range mappings {
		if m.StatID.Valid {
			mappingMap[m.StatID.UUID.String()] = m
		}
	}

	dateThreshold := time.Now().AddDate(0, 0, -9)

	for _, source := range sources {
		sourceMapping, err := dbQueries.GetTargetSourceBySource(context.Background(), database.GetTargetSourceBySourceParams{
			TargetID: target.ID,
			SourceID: source.ID,
		})
		if err != nil {
			continue
		}

		// Step 1: Update synced stats from last 2 days
		syncedStats, err := dbQueries.GetSyncedPageStatsForUpdate(context.Background(), database.GetSyncedPageStatsForUpdateParams{
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
			safeTargetID, err := helpers.ToInt32(targetIDVal)
			if err != nil {
				continue
			}

			fieldMap := NocoRecordFields{
				ID:       stat.ID.String(),
				Date:     stat.Date,
				PagePath: stat.UrlPath,
				Views:    stat.Views,
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
		unsyncedStats, err := dbQueries.GetUnsyncedPageStatsForTarget(context.Background(), database.GetUnsyncedPageStatsForTargetParams{
			TargetID: target.ID,
			SourceID: source.ID,
		})
		if err != nil {
			return err
		}

		var records []NocoTableRecord
		var currentBatch []database.AnalyticsPageStat

		flushCreate := func() error {
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
					StatID:         uuid.NullUUID{UUID: originalStat.ID, Valid: true},
					TargetID:       target.ID,
					TargetRecordID: fmt.Sprintf("%.0f", id),
				})
				if err != nil {
					return fmt.Errorf("failed to map page stat: %w", err)
				}

				createdIds = append(createdIds, int32(id))
			}

			sourceNocoId, _ := strconv.Atoi(sourceMapping.TargetSourceID)
			safeSourceNocoId, err := helpers.ToInt32(sourceNocoId)
			if err != nil {
				log.Printf("Invalid source Noco ID: %v", err)
				return err
			}

			if err := linkChildrenToParent(c, dbQueries, encryptionKey, target, sourcesTableMapping, "page_stats", safeSourceNocoId, createdIds); err != nil {
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

	var deleteMappings []database.AnalyticsPageStatsOnTarget
	for _, m := range mappings {
		if !m.StatID.Valid {
			deleteMappings = append(deleteMappings, m)
		}
	}

	var deleteRecords []NocoDeleteRecord
	flushDelete := func() error {
		if len(deleteRecords) == 0 {
			return nil
		}
		if err := deleteNocoRecords(c, dbQueries, encryptionKey, target, tableMapping.TargetTableCode.String, deleteRecords); err != nil {
			return err
		}
		deleteRecords = deleteRecords[:0]
		return nil
	}

	for _, m := range deleteMappings {
		targetIDVal, _ := strconv.Atoi(m.TargetRecordID)
		safeTargetID, err := helpers.ToInt32(targetIDVal)
		if err != nil {
			log.Printf("Invalid target ID: %v", err)
			continue
		}

		deleteRecords = append(deleteRecords, NocoDeleteRecord{
			ID: safeTargetID,
		})

		if len(deleteRecords) == batchSize {
			if err := flushDelete(); err != nil {
				return err
			}
		}
		if err := dbQueries.DeleteAnalyticsPageStatOnTarget(context.Background(), m.ID); err != nil {
			log.Printf("Warning: failed to delete mapping %s: %v", m.ID, err)
		}
	}
	if err := flushDelete(); err != nil {
		return err
	}

	return nil
}

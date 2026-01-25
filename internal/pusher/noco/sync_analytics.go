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

	for _, source := range sources {
		localStats, err := dbQueries.GetAllPageStatsWithTargetInfo(context.Background(), database.GetAllPageStatsWithTargetInfoParams{
			TargetID: target.ID,
			SourceID: source.ID,
		})
		if err != nil {
			return err
		}

		var createStats []database.GetAllPageStatsWithTargetInfoRow
		var updateStats []database.GetAllPageStatsWithTargetInfoRow

		for _, stat := range localStats {
			if !stat.TargetRecordID.Valid {
				createStats = append(createStats, stat)
			} else {
				updateStats = append(updateStats, stat)
			}
		}

		sourceMapping, err := dbQueries.GetTargetSourceBySource(context.Background(), database.GetTargetSourceBySourceParams{
			TargetID: target.ID,
			SourceID: source.ID,
		})
		if err != nil {
			continue
		}

		var records []NocoTableRecord
		var currentBatch []database.GetAllPageStatsWithTargetInfoRow

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

			if err := linkChildrenToParent(c, dbQueries, encryptionKey, target, sourcesTableMapping, "page_stats", int32(sourceNocoId), createdIds); err != nil {
				log.Printf("Failed to link page stats to source: %v", err)
			}

			records = records[:0]
			currentBatch = currentBatch[:0]
			return nil
		}

		for _, stat := range createStats {
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

		for _, stat := range updateStats {
			targetIDVal, _ := strconv.Atoi(stat.TargetRecordID.String)
			fieldMap := NocoRecordFields{
				ID:       stat.ID.String(),
				Date:     stat.Date,
				PagePath: stat.UrlPath,
				Views:    stat.Views,
			}
			updateRecords = append(updateRecords, NocoTableRecord{
				Id:     int32(targetIDVal),
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
		deleteRecords = append(deleteRecords, NocoDeleteRecord{
			ID: int32(targetIDVal),
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

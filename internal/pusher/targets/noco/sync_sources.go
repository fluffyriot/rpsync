package noco

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/google/uuid"
)

func syncNocoSources(c *common.Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableId string) error {

	var createSources []database.Source
	var removeSources []database.SourcesOnTarget

	userSources, err := dbQueries.GetUserSources(context.Background(), target.UserID)
	if err != nil {
		return fmt.Errorf("error fetching user sources: %w", err)
	}

	mappedSources, err := dbQueries.GetTargetSources(context.Background(), target.ID)
	if err != nil {
		return fmt.Errorf("error fetching user sources: %w", err)
	}

	internMap := make(map[string]database.Source, len(userSources))
	mappedMap := make(map[string]database.SourcesOnTarget, len(mappedSources))

	for _, uSource := range userSources {
		internMap[uSource.ID.String()] = uSource
	}

	for _, mSource := range mappedSources {
		mappedMap[mSource.SourceID.String()] = mSource
	}

	for id, uSource := range internMap {
		if _, ok := mappedMap[id]; !ok {
			createSources = append(createSources, uSource)
		}
	}

	for id, mSource := range mappedMap {
		if _, ok := internMap[id]; !ok {
			removeSources = append(removeSources, mSource)
		}
	}

	const batchSize = 10

	var recordsCreate []NocoTableRecord

	flush := func() error {
		if len(recordsCreate) == 0 {
			return nil
		}

		createdRecords, err := createNocoRecords(
			c,
			dbQueries,
			encryptionKey,
			target,
			tableId,
			recordsCreate,
		)
		if err != nil {
			return err
		}

		for _, rec := range createdRecords {
			var id float64
			if val, ok := rec["Id"].(float64); ok {
				id = val
			} else if val, ok := rec["id"].(float64); ok {
				id = val
			} else {
				continue
			}

			fields, ok := rec["fields"].(map[string]any)
			if !ok {
				continue
			}

			ctId, ok := fields["ct_id"].(string)
			if !ok {
				continue
			}

			var source *database.Source
			for _, s := range createSources {
				if s.ID.String() == ctId {
					source = &s
					break
				}
			}

			if source == nil {
				continue
			}

			_, err := dbQueries.AddSourceToTarget(context.Background(), database.AddSourceToTargetParams{
				ID:             uuid.New(),
				SourceID:       source.ID,
				TargetID:       target.ID,
				TargetSourceID: fmt.Sprintf("%.0f", id),
			})

			if err != nil {
				return err
			}
		}

		recordsCreate = recordsCreate[:0]
		return nil
	}

	for _, source := range createSources {
		url, _ := helpers.ConvNetworkToURL(source.Network, source.UserName)

		fieldMap := NocoRecordFields{
			ID:         source.ID.String(),
			LastSynced: source.LastSynced.Time,
			Network:    source.Network,
			Username:   source.UserName,
			URL:        url,
		}

		recordsCreate = append(recordsCreate, NocoTableRecord{
			Fields: fieldMap,
		})

		if len(recordsCreate) == batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	if err := flush(); err != nil {
		return err
	}

	var recordsDelete []NocoDeleteRecord

	flushDelete := func() error {
		if len(recordsDelete) == 0 {
			return nil
		}

		if err := deleteNocoRecords(
			c,
			dbQueries,
			encryptionKey,
			target,
			tableId,
			recordsDelete,
		); err != nil {
			return err
		}

		recordsDelete = recordsDelete[:0]
		return nil
	}

	for _, source := range removeSources {
		v, _ := strconv.Atoi(source.TargetSourceID)
		intId, err := helpers.ToInt32(v)
		if err != nil {
			continue
		}

		recordsDelete = append(recordsDelete, NocoDeleteRecord{
			ID: intId,
		})

		if len(recordsDelete) == batchSize {
			if err := flushDelete(); err != nil {
				return err
			}
		}
	}

	if err := flushDelete(); err != nil {
		return err
	}

	for _, source := range removeSources {
		err := dbQueries.DeleteSourceTarget(context.Background(), database.DeleteSourceTargetParams{
			TargetID: target.ID,
			SourceID: source.SourceID,
		})
		if err != nil {
			return fmt.Errorf("failed to delete source target mapping: %w", err)
		}
	}

	tm, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "sources",
	})
	if err == nil {
		colMapping, err := dbQueries.GetColumnMappingsByTableAndName(context.Background(), database.GetColumnMappingsByTableAndNameParams{
			TableMappingID:   tm.ID,
			TargetColumnName: "network",
		})
		if err == nil && colMapping.TargetColumnCode.Valid {
			var choices []NocoColumnTypeOptions
			for _, network := range helpers.AvailableSources {
				choices = append(choices, NocoColumnTypeOptions{Title: network.Name, Color: network.Color})
			}

			err = updateNocoColumn(c, dbQueries, encryptionKey, target, tableId, colMapping.TargetColumnCode.String, NocoColumn{
				Title: "network",
				Type:  "SingleSelect",
				Options: NocoColumnTypeSelectOptions{
					Choices: choices,
				},
			})
			if err != nil {
				log.Printf("Failed to update network column options: %v", err)
			} else {
				log.Println("Updated network column options")
			}
		}
	}

	return nil
}

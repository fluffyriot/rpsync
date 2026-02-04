package noco

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/google/uuid"
)

func InitializeNoco(dbQueries *database.Queries, c *common.Client, encryptionKey []byte, target database.Target) error {
	log.Println("InitializeNoco started for target", target.ID)

	nocoURL := target.HostUrl.String +
		"/api/v3/meta/bases/" +
		target.DbID.String +
		"/tables"

	_, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "posts",
	})
	var postsRespID string
	if err != nil {
		postsTable := NocoTable{
			Title:       "posts",
			Description: "Posts from your social networks",
			Fields: []NocoColumn{
				{Title: "ct_id", Type: "SingleLineText", Unique: true},
				{Title: "created_at", Type: "DateTime"},
				{Title: "last_synced", Type: "DateTime"},
				{Title: "is_archived", Type: "Checkbox"},
				{Title: "network_internal_id", Type: "SingleLineText"},
				{Title: "post_type", Type: "SingleLineText"},
				{Title: "author", Type: "SingleLineText"},
				{Title: "content", Type: "LongText"},
				{Title: "likes", Type: "Number"},
				{Title: "views", Type: "Number"},
				{Title: "reposts", Type: "Number"},
				{Title: "URL", Type: "URL"},
			},
		}

		postsResp, err := createNocoTable(c, dbQueries, encryptionKey, target.ID, nocoURL, postsTable)
		if err != nil {
			return err
		}
		postsRespID = postsResp.ID

		postsMapping, err := dbQueries.CreateMappingForTable(context.Background(), database.CreateMappingForTableParams{
			ID:              uuid.New(),
			CreatedAt:       time.Now(),
			SourceTableName: "posts",
			TargetTableName: postsResp.Title,
			TargetTableCode: sql.NullString{String: postsResp.ID, Valid: true},
			TargetID:        target.ID,
		})
		if err != nil {
			return fmt.Errorf("create posts table mapping: %w", err)
		}

		for _, field := range postsResp.Fields {
			_, err := dbQueries.CreateMappingForColumn(context.Background(), database.CreateMappingForColumnParams{
				ID:               uuid.New(),
				CreatedAt:        time.Now(),
				TableMappingID:   postsMapping.ID,
				SourceColumnName: field.Title,
				TargetColumnName: field.Title,
				TargetColumnCode: sql.NullString{String: field.ID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create posts column mapping %s: %w", field.Title, err)
			}
		}
	} else {
		tm, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
			TargetID:        target.ID,
			TargetTableName: "posts",
		})
		if err == nil {
			postsRespID = tm.TargetTableCode.String
		}
	}

	_, err = dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "analytics_site_stats",
	})
	var siteStatsRespID string
	if err != nil {
		analyticsSiteStatsTable := NocoTable{
			Title:       "analytics_site_stats",
			Description: "Daily website analytics (visitors, session duration)",
			Fields: []NocoColumn{
				{Title: "ct_id", Type: "SingleLineText", Unique: true},
				{Title: "date", Type: "Date"},
				{Title: "visitors", Type: "Number"},
				{Title: "avg_session_duration", Type: "Decimal"},
			},
		}

		siteStatsResp, err := createNocoTable(c, dbQueries, encryptionKey, target.ID, nocoURL, analyticsSiteStatsTable)
		if err != nil {
			return fmt.Errorf("create analytics site stats table: %w", err)
		}
		siteStatsRespID = siteStatsResp.ID

		siteStatsMapping, err := dbQueries.CreateMappingForTable(context.Background(), database.CreateMappingForTableParams{
			ID:              uuid.New(),
			CreatedAt:       time.Now(),
			SourceTableName: "analytics_site_stats",
			TargetTableName: siteStatsResp.Title,
			TargetTableCode: sql.NullString{String: siteStatsResp.ID, Valid: true},
			TargetID:        target.ID,
		})
		if err != nil {
			return fmt.Errorf("create analytics site stats table mapping: %w", err)
		}

		for _, field := range siteStatsResp.Fields {
			_, err := dbQueries.CreateMappingForColumn(context.Background(), database.CreateMappingForColumnParams{
				ID:               uuid.New(),
				CreatedAt:        time.Now(),
				TableMappingID:   siteStatsMapping.ID,
				SourceColumnName: field.Title,
				TargetColumnName: field.Title,
				TargetColumnCode: sql.NullString{String: field.ID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create analytics site stats column mapping %s: %w", field.Title, err)
			}
		}
	} else {
		tm, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
			TargetID:        target.ID,
			TargetTableName: "analytics_site_stats",
		})
		if err == nil {
			siteStatsRespID = tm.TargetTableCode.String
		}
	}

	_, err = dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "analytics_page_stats",
	})
	var pageStatsRespID string
	if err != nil {
		analyticsPageStatsTable := NocoTable{
			Title:       "analytics_page_stats",
			Description: "Daily page view analytics",
			Fields: []NocoColumn{
				{Title: "ct_id", Type: "SingleLineText", Unique: true},
				{Title: "date", Type: "Date"},
				{Title: "page_path", Type: "SingleLineText"},
				{Title: "views", Type: "Number"},
			},
		}

		pageStatsResp, err := createNocoTable(c, dbQueries, encryptionKey, target.ID, nocoURL, analyticsPageStatsTable)
		if err != nil {
			return fmt.Errorf("create analytics page stats table: %w", err)
		}
		pageStatsRespID = pageStatsResp.ID

		pageStatsMapping, err := dbQueries.CreateMappingForTable(context.Background(), database.CreateMappingForTableParams{
			ID:              uuid.New(),
			CreatedAt:       time.Now(),
			SourceTableName: "analytics_page_stats",
			TargetTableName: pageStatsResp.Title,
			TargetTableCode: sql.NullString{String: pageStatsResp.ID, Valid: true},
			TargetID:        target.ID,
		})
		if err != nil {
			return fmt.Errorf("create analytics page stats table mapping: %w", err)
		}

		for _, field := range pageStatsResp.Fields {
			_, err := dbQueries.CreateMappingForColumn(context.Background(), database.CreateMappingForColumnParams{
				ID:               uuid.New(),
				CreatedAt:        time.Now(),
				TableMappingID:   pageStatsMapping.ID,
				SourceColumnName: field.Title,
				TargetColumnName: field.Title,
				TargetColumnCode: sql.NullString{String: field.ID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create analytics page stats column mapping %s: %w", field.Title, err)
			}
		}
	} else {
		tm, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
			TargetID:        target.ID,
			TargetTableName: "analytics_page_stats",
		})
		if err == nil {
			pageStatsRespID = tm.TargetTableCode.String
		}
	}

	_, err = dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "sources_stats",
	})
	var sourcesStatsRespID string
	if err != nil {
		sourcesStatsTable := NocoTable{
			Title:       "sources_stats",
			Description: "Daily profile statistics (followers, following, averages)",
			Fields: []NocoColumn{
				{Title: "ct_id", Type: "SingleLineText", Unique: true},
				{Title: "date", Type: "Date"},
				{Title: "followers_count", Type: "Number"},
				{Title: "following_count", Type: "Number"},
				{Title: "posts_count", Type: "Number"},
				{Title: "average_likes", Type: "Decimal"},
				{Title: "average_reposts", Type: "Decimal"},
				{Title: "average_views", Type: "Decimal"},
			},
		}

		sourcesStatsResp, err := createNocoTable(c, dbQueries, encryptionKey, target.ID, nocoURL, sourcesStatsTable)
		if err != nil {
			return fmt.Errorf("create sources stats table: %w", err)
		}
		sourcesStatsRespID = sourcesStatsResp.ID

		sourcesStatsMapping, err := dbQueries.CreateMappingForTable(context.Background(), database.CreateMappingForTableParams{
			ID:              uuid.New(),
			CreatedAt:       time.Now(),
			SourceTableName: "sources_stats",
			TargetTableName: sourcesStatsResp.Title,
			TargetTableCode: sql.NullString{String: sourcesStatsResp.ID, Valid: true},
			TargetID:        target.ID,
		})
		if err != nil {
			return fmt.Errorf("create sources stats table mapping: %w", err)
		}

		for _, field := range sourcesStatsResp.Fields {
			_, err := dbQueries.CreateMappingForColumn(context.Background(), database.CreateMappingForColumnParams{
				ID:               uuid.New(),
				CreatedAt:        time.Now(),
				TableMappingID:   sourcesStatsMapping.ID,
				SourceColumnName: field.Title,
				TargetColumnName: field.Title,
				TargetColumnCode: sql.NullString{String: field.ID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create sources stats column mapping %s: %w", field.Title, err)
			}
		}
	} else {
		tm, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
			TargetID:        target.ID,
			TargetTableName: "sources_stats",
		})
		if err == nil {
			sourcesStatsRespID = tm.TargetTableCode.String
		}
	}

	tmSources, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "sources",
	})

	var sourcesMapping database.TableMapping
	var sourcesTableID string

	if err != nil {
		var choices []NocoColumnTypeOptions
		for _, source := range helpers.AvailableSources {
			choices = append(choices, NocoColumnTypeOptions{Title: source.Name, Color: source.Color})
		}

		sourcesTable := NocoTable{
			Title:       "sources",
			Description: "Social media sources",
			Fields: []NocoColumn{
				{Title: "ct_id", Type: "SingleLineText", Unique: true},
				{
					Title: "network",
					Type:  "SingleSelect",
					Options: NocoColumnTypeSelectOptions{
						Choices: choices,
					},
				},
				{Title: "username", Type: "SingleLineText"},
				{Title: "URL", Type: "URL"},
				{Title: "last_synced", Type: "DateTime"},
			},
		}

		sourcesResp, err := createNocoTable(c, dbQueries, encryptionKey, target.ID, nocoURL, sourcesTable)
		if err != nil {
			return err
		}
		sourcesTableID = sourcesResp.ID

		sourcesMapping, err = dbQueries.CreateMappingForTable(context.Background(), database.CreateMappingForTableParams{
			ID:              uuid.New(),
			CreatedAt:       time.Now(),
			SourceTableName: "sources",
			TargetTableName: sourcesResp.Title,
			TargetTableCode: sql.NullString{String: sourcesResp.ID, Valid: true},
			TargetID:        target.ID,
		})
		if err != nil {
			return fmt.Errorf("create sources table mapping: %w", err)
		}

		for _, field := range sourcesResp.Fields {
			_, err := dbQueries.CreateMappingForColumn(context.Background(), database.CreateMappingForColumnParams{
				ID:               uuid.New(),
				CreatedAt:        time.Now(),
				TableMappingID:   sourcesMapping.ID,
				SourceColumnName: field.Title,
				TargetColumnName: field.Title,
				TargetColumnCode: sql.NullString{String: field.ID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create sources column mapping %s: %w", field.Title, err)
			}
		}
	} else {
		sourcesMapping = tmSources
		sourcesTableID = sourcesMapping.TargetTableCode.String
	}

	linkCols := map[string]string{
		"posts":         postsRespID,
		"site_stats":    siteStatsRespID,
		"page_stats":    pageStatsRespID,
		"sources_stats": sourcesStatsRespID,
	}

	for colName, relatedTableID := range linkCols {
		_, err := dbQueries.GetColumnMappingsByTableAndName(context.Background(), database.GetColumnMappingsByTableAndNameParams{
			TableMappingID:   sourcesMapping.ID,
			TargetColumnName: colName,
		})

		if err != nil {

			log.Printf("Sources column mapping missing: %s. Creating in NocoDB...", colName)

			newCol := NocoColumn{
				Title: colName,
				Type:  "Links",
				Options: NocoColumnTypeRelation{
					RelationType:   "hm",
					RelatedTableId: relatedTableID,
				},
			}
			respCol, err := createNocoColumn(c, dbQueries, encryptionKey, target, sourcesTableID, newCol)
			if err != nil {
				return fmt.Errorf("failed to create column %s: %w", colName, err)
			}
			colID := respCol.ID

			_, err = dbQueries.CreateMappingForColumn(context.Background(), database.CreateMappingForColumnParams{
				ID:               uuid.New(),
				CreatedAt:        time.Now(),
				TableMappingID:   sourcesMapping.ID,
				SourceColumnName: colName,
				TargetColumnName: colName,
				TargetColumnCode: sql.NullString{String: colID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to create column mapping for %s: %w", colName, err)
			}
			log.Printf("Created column and mapping for %s", colName)
		}
	}

	return nil
}

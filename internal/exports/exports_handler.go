package exports

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

func InitiateExport(userID uuid.UUID, syncMethod string, dbQueries *database.Queries) (database.Export, error) {

	export, err := dbQueries.CreateExport(context.Background(), database.CreateExportParams{
		ID:           uuid.New(),
		CreatedAt:    time.Now(),
		ExportStatus: "Requested",
		UserID:       userID,
		ExportMethod: syncMethod,
	})

	if err != nil {
		log.Printf("Error creating export record: %v", err)
		return database.Export{}, err
	}

	switch syncMethod {
	case "csv":
		csvExport(userID, dbQueries, export)
		return database.Export{}, fmt.Errorf("Not implemented")
	case "notion":
		notionExport(userID, dbQueries, export)
		return database.Export{}, fmt.Errorf("Not implemented")
	case "none":
		testExport(userID, dbQueries, export)
		return database.Export{}, fmt.Errorf("Not required - testing/dev only")
	default:
		return database.Export{}, errors.New("Unknown sync method")
	}

}

func csvExport(userID uuid.UUID, dbQueries *database.Queries, exp database.Export) error {

	_, err := dbQueries.ChangeExportStatusById(context.Background(), database.ChangeExportStatusByIdParams{
		ID:           exp.ID,
		ExportStatus: "Processing",
	})
	if err != nil {
		log.Printf("Error updating export status: %v", err)
	}

	posts, err := dbQueries.GetAllPostsWithTheLatestInfoForUser(context.Background(), userID)
	if err != nil {
		log.Printf("Error fetching posts for export: %v", err)
		return err
	}

	filename := fmt.Sprintf("outputs/export_%s_posts_%s.csv", userID.String(), time.Now().Format("20060102_150405"))

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{
		"id",
		"created_at",
		"source_id",
		"is_archived",
		"network_internal_id",
		"content",
		"user_id",
		"reactions_synced_at",
		"likes",
		"reposts",
		"views",
	}); err != nil {
		return err
	}

	for _, r := range posts {

		content := ""
		if r.Content.Valid {
			content = r.Content.String
		}

		userID := ""
		if r.UserID.Valid {
			userID = r.UserID.UUID.String()
		}

		reactionsSyncedAt := ""
		if r.ReactionsSyncedAt.Valid {
			reactionsSyncedAt = r.ReactionsSyncedAt.Time.Format(time.RFC3339)
		}

		likes := ""
		if r.Likes.Valid {
			likes = strconv.FormatInt(int64(r.Likes.Int32), 10)
		}

		reposts := ""
		if r.Reposts.Valid {
			reposts = strconv.FormatInt(int64(r.Reposts.Int32), 10)
		}

		views := ""
		if r.Views.Valid {
			views = strconv.FormatInt(int64(r.Views.Int32), 10)
		}

		record := []string{
			r.ID.String(),
			r.CreatedAt.Format(time.RFC3339),
			r.SourceID.String(),
			strconv.FormatBool(r.IsArchived),
			r.NetworkInternalID,
			content,
			userID,
			reactionsSyncedAt,
			likes,
			reposts,
			views,
		}

		if err := writer.Write(record); err != nil {

			dbQueries.ChangeExportStatusById(context.Background(), database.ChangeExportStatusByIdParams{
				ID:            exp.ID,
				ExportStatus:  "Failed",
				StatusMessage: sql.NullString{String: err.Error(), Valid: true},
				CompletedAt:   time.Now(),
			})

			return err
		}
	}

	dbQueries.ChangeExportStatusById(context.Background(), database.ChangeExportStatusByIdParams{
		ID:           exp.ID,
		ExportStatus: "Completed",
		DownloadUrl:  sql.NullString{String: filename, Valid: true},
		CompletedAt:  time.Now(),
	})

	return writer.Error()

}

func notionExport(userID uuid.UUID, dbQueries *database.Queries, exp database.Export) error {
	// Placeholder for Notion export logic
	return nil
}

func testExport(userID uuid.UUID, dbQueries *database.Queries, exp database.Export) error {
	// Placeholder for testing/dev export logic
	return nil
}

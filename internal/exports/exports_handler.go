package exports

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/google/uuid"
)

func DeleteAllExports(userID uuid.UUID, dbQueries *database.Queries) error {

	exports, err := dbQueries.GetAllExportsByUserId(context.Background(), userID)
	if err != nil {
		log.Printf("Error getting all exports records: %v", err)
		return err
	}
	for _, exp := range exports {
		if exp.DownloadUrl.Valid {
			err := os.Remove(exp.DownloadUrl.String)
			if err != nil {
				log.Printf("Error deleting export file %s: %v", exp.DownloadUrl.String, err)
			}
		}
	}

	err = dbQueries.DeleteAllExportsByUserId(context.Background(), userID)
	if err != nil {
		log.Printf("Error deleting all exports records: %v", err)
		return err
	}

	return nil

}

func CreateLogAutoExport(userID uuid.UUID, dbQueries *database.Queries, method string, targetId uuid.UUID) (database.Export, error) {

	export, err := dbQueries.CreateExport(context.Background(), database.CreateExportParams{
		ID:           uuid.New(),
		CreatedAt:    time.Now(),
		ExportStatus: "Requested",
		UserID:       userID,
		ExportMethod: method,
		TargetID:     uuid.NullUUID{UUID: targetId, Valid: true},
	})

	return export, err
}

func UpdateLogAutoExport(export database.Export, dbQueries *database.Queries, status, statusReason, filename string) error {
	var completedDate time.Time
	if status == "Completed" {
		completedDate = time.Now()
	}

	var downloadURL sql.NullString
	if filename != "" {
		downloadURL = sql.NullString{
			String: filename,
			Valid:  true,
		}
	} else {
		downloadURL = sql.NullString{
			Valid: false,
		}
	}

	var statusMessage sql.NullString
	if statusReason != "" {
		statusMessage = sql.NullString{
			String: statusReason,
			Valid:  true,
		}
	} else {
		statusMessage = sql.NullString{
			Valid: false,
		}
	}

	_, err := dbQueries.ChangeExportStatusById(context.Background(), database.ChangeExportStatusByIdParams{
		ID:            export.ID,
		ExportStatus:  status,
		StatusMessage: statusMessage,
		CompletedAt:   completedDate,
		DownloadUrl:   downloadURL,
	})

	return err
}

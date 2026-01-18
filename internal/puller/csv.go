package puller

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
)

func startCsvSync(dbQueries *database.Queries, target database.Target, export database.Export) (string, error) {

	posts, err := dbQueries.GetAllPostsWithTheLatestInfoForUser(context.Background(), target.UserID)
	if err != nil {
		log.Printf("Error fetching posts for export: %v", err)
		return "", err
	}

	filename := fmt.Sprintf("outputs/export_id_%s_%s.csv", export.ID.String(), time.Now().Format("20060102_150405"))

	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{
		"ct_id",
		"posted_at",
		"last_updated",
		"is_archived",
		"network",
		"post_type",
		"post_author",
		"likes",
		"reposts",
		"views",
		"url",
		"content",
	}); err != nil {
		return "", err
	}

	for _, r := range posts {

		content := ""
		if r.Content.Valid {
			content = r.Content.String
		}

		network := ""
		if r.Network.Valid {
			network = r.Network.String
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

		url, _ := ConvPostToURL(network, r.Author, r.NetworkInternalID)

		record := []string{
			r.ID.String(),
			r.CreatedAt.Format(time.RFC3339),
			reactionsSyncedAt,
			strconv.FormatBool(r.IsArchived),
			network,
			r.PostType,
			r.Author,
			likes,
			reposts,
			views,
			url,
			content,
		}

		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	return filename, writer.Error()

}

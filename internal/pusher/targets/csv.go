package targets

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/google/uuid"
)

func HasPosts(dbQueries *database.Queries, userID uuid.UUID) (bool, error) {

	count, err := dbQueries.CheckCountOfPostsForUser(context.Background(), userID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func HasAnalytics(dbQueries *database.Queries, userID uuid.UUID) (bool, error) {
	count, err := dbQueries.CheckCountOfAnalyticsSiteStatsForUser(context.Background(), userID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func GeneratePostsCsv(dbQueries *database.Queries, target database.Target, export database.Export) (string, error) {
	posts, err := dbQueries.GetAllPostsWithTheLatestInfoForUser(context.Background(), target.UserID)
	if err != nil {
		return "", fmt.Errorf("fetching posts: %w", err)
	}

	if len(posts) == 0 {
		return "", nil
	}

	filename := fmt.Sprintf("outputs/export_id_%s_posts_%s.csv", export.ID.String(), time.Now().Format("20060102_150405"))
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
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
			likes = strconv.FormatInt(r.Likes.Int64, 10)
		}
		reposts := ""
		if r.Reposts.Valid {
			reposts = strconv.FormatInt(r.Reposts.Int64, 10)
		}
		views := ""
		if r.Views.Valid {
			views = strconv.FormatInt(r.Views.Int64, 10)
		}

		url, _ := helpers.ConvPostToURL(network, r.Author, r.NetworkInternalID)

		if err := writer.Write([]string{
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
		}); err != nil {
			return "", err
		}
	}

	return filename, nil
}

func GenerateWebsiteCsv(dbQueries *database.Queries, target database.Target, export database.Export) (string, error) {
	stats, err := dbQueries.GetAllAnalyticsSiteStatsForUser(context.Background(), target.UserID)
	if err != nil {
		return "", fmt.Errorf("fetching site stats: %w", err)
	}

	if len(stats) == 0 {
		return "", nil
	}

	filename := fmt.Sprintf("outputs/export_id_%s_website_%s.csv", export.ID.String(), time.Now().Format("20060102_150405"))
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{
		"ct_id",
		"date",
		"visitors",
		"avg_session_duration",
		"source_network",
		"source_username",
	}); err != nil {
		return "", err
	}

	for _, s := range stats {
		if err := writer.Write([]string{
			s.ID.String(),
			s.Date.Format("2006-01-02"),
			strconv.Itoa(s.Visitors),
			fmt.Sprintf("%f", s.AvgSessionDuration),
			s.SourceNetwork,
			s.SourceUserName,
		}); err != nil {
			return "", err
		}
	}

	return filename, nil
}

func GeneratePageViewsCsv(dbQueries *database.Queries, target database.Target, export database.Export) (string, error) {
	stats, err := dbQueries.GetAllAnalyticsPageStatsForUser(context.Background(), target.UserID)
	if err != nil {
		return "", fmt.Errorf("fetching pages stats: %w", err)
	}

	if len(stats) == 0 {
		return "", nil
	}

	filename := fmt.Sprintf("outputs/export_id_%s_webpages_%s.csv", export.ID.String(), time.Now().Format("20060102_150405"))
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{
		"ct_id",
		"date",
		"url_path",
		"views",
		"source_network",
		"source_username",
	}); err != nil {
		return "", err
	}

	for _, s := range stats {
		if err := writer.Write([]string{
			s.ID.String(),
			s.Date.Format("2006-01-02"),
			s.UrlPath,
			strconv.Itoa(s.Views),
			s.SourceNetwork,
			s.SourceUserName,
		}); err != nil {
			return "", err
		}
	}

	return filename, nil
}

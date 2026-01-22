package fetcher

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
	"golang.org/x/net/html"
)

func stripHTMLToText(input string) string {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return ""
	}

	var b strings.Builder

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				b.WriteString(text)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(doc)

	return strings.Join(strings.Fields(html.UnescapeString(b.String())), " ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func saveOrUpdateSourceStats(ctx context.Context, dbQueries *database.Queries, sourceID uuid.UUID, stats *ProfileStats) error {

	today := time.Now().UTC().Truncate(24 * time.Hour)

	existing, err := dbQueries.GetSourceStatsByDate(ctx, database.GetSourceStatsByDateParams{
		SourceID: sourceID,
		Date:     today,
	})

	var followersCount, followingCount, postsCount sql.NullInt32
	var avgLikes, avgReposts, avgViews sql.NullFloat64

	if stats.FollowersCount != nil {
		followersCount = sql.NullInt32{Int32: int32(*stats.FollowersCount), Valid: true}
	}
	if stats.FollowingCount != nil {
		followingCount = sql.NullInt32{Int32: int32(*stats.FollowingCount), Valid: true}
	}
	if stats.PostsCount != nil {
		postsCount = sql.NullInt32{Int32: int32(*stats.PostsCount), Valid: true}
	}
	if stats.AverageLikes != nil {
		avgLikes = sql.NullFloat64{Float64: *stats.AverageLikes, Valid: true}
	}
	if stats.AverageReposts != nil {
		avgReposts = sql.NullFloat64{Float64: *stats.AverageReposts, Valid: true}
	}
	if stats.AverageViews != nil {
		avgViews = sql.NullFloat64{Float64: *stats.AverageViews, Valid: true}
	}

	if err != nil {

		_, err = dbQueries.CreateSourceStat(ctx, database.CreateSourceStatParams{
			ID:             uuid.New(),
			Date:           today,
			SourceID:       sourceID,
			FollowersCount: followersCount,
			FollowingCount: followingCount,
			PostsCount:     postsCount,
			AverageLikes:   avgLikes,
			AverageReposts: avgReposts,
			AverageViews:   avgViews,
		})
		return err
	}

	_, err = dbQueries.UpdateSourceDayStats(ctx, database.UpdateSourceDayStatsParams{
		FollowersCount: followersCount,
		FollowingCount: followingCount,
		PostsCount:     postsCount,
		AverageLikes:   avgLikes,
		AverageReposts: avgReposts,
		AverageViews:   avgViews,
		SourceID:       sourceID,
		Date:           today,
	})

	if err == nil && existing.ID != uuid.Nil {
		log.Printf("Updated stats for source %s (date: %s)", sourceID, today.Format("2006-01-02"))
	}

	return err
}

func calculateAverageStats(ctx context.Context, dbQueries *database.Queries, sourceID uuid.UUID) (*ProfileStats, error) {
	totals, err := dbQueries.GetSourceTotals(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	stats := &ProfileStats{}

	if totals.TotalPosts > 0 {
		postsCount := int(totals.TotalPosts)
		stats.PostsCount = &postsCount

		avgLikes := float64(totals.TotalLikes) / float64(totals.TotalPosts)
		stats.AverageLikes = &avgLikes

		avgReposts := float64(totals.TotalReposts) / float64(totals.TotalPosts)
		stats.AverageReposts = &avgReposts

		avgViews := float64(totals.TotalViews) / float64(totals.TotalPosts)
		stats.AverageViews = &avgViews
	}

	return stats, nil
}

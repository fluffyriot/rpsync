// SPDX-License-Identifier: AGPL-3.0-only
package common

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/google/uuid"
	"golang.org/x/net/html"
)

const (
	APIRateLimit     = 50 * time.Millisecond
	ScraperRateLimit = 3 * time.Second
	RateLimitWait    = 30 * time.Second

	UserAgentBingbot = "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)"
	UserAgentChrome  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.7727.102 Safari/537.36"
)

func StripHTMLToText(input string) string {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return ""
	}

	var b strings.Builder

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			switch n.Data {
			case "p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li":
				if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") && !strings.HasSuffix(b.String(), "\n") {
					b.WriteString(" ")
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "br":
				if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") && !strings.HasSuffix(b.String(), "\n") {
					b.WriteString(" ")
				}
			}
		}
	}

	walk(doc)

	text := html.UnescapeString(b.String())
	return strings.Join(strings.Fields(text), " ")
}

func SaveOrUpdateSourceStats(ctx context.Context, dbQueries *database.Queries, sourceID uuid.UUID, stats *ProfileStats) error {

	today := time.Now().UTC().Truncate(24 * time.Hour)

	existing, err := dbQueries.GetSourceStatsByDate(ctx, database.GetSourceStatsByDateParams{
		SourceID: sourceID,
		Date:     today,
	})

	var followersCount, followingCount, postsCount sql.NullInt64
	var avgLikes, avgReposts, avgViews sql.NullFloat64

	if stats.FollowersCount != nil {
		followersCount = sql.NullInt64{Int64: int64(*stats.FollowersCount), Valid: true}
	}
	if stats.FollowingCount != nil {
		followingCount = sql.NullInt64{Int64: int64(*stats.FollowingCount), Valid: true}
	}
	if stats.PostsCount != nil {
		postsCount = sql.NullInt64{Int64: int64(*stats.PostsCount), Valid: true}
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

func CalculateAverageStats(ctx context.Context, dbQueries *database.Queries, sourceID uuid.UUID) (*ProfileStats, error) {
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

func sanitizeText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, s)
}

func CreateOrUpdatePost(
	ctx context.Context,
	dbQueries *database.Queries,
	sourceID uuid.UUID,
	networkInternalID string,
	network string,
	createdAt time.Time,
	postType string,
	author string,
	content string,
) (uuid.UUID, error) {
	content = sanitizeText(html.UnescapeString(content))

	post, err := dbQueries.GetPostBySourceAndNetworkId(ctx, database.GetPostBySourceAndNetworkIdParams{
		NetworkInternalID: networkInternalID,
		SourceID:          sourceID,
	})

	if err != nil {
		newPost, err := dbQueries.CreatePost(ctx, database.CreatePostParams{
			ID:                uuid.New(),
			CreatedAt:         createdAt,
			LastSyncedAt:      time.Now(),
			SourceID:          sourceID,
			IsArchived:        false,
			Author:            author,
			PostType:          postType,
			NetworkInternalID: networkInternalID,
			Content: sql.NullString{
				String: content,
				Valid:  content != "",
			},
		})
		if err != nil {
			log.Printf("CreatePost failed: sourceID=%s networkInternalID=%q author=%q postType=%q contentLen=%d err=%v", sourceID, networkInternalID, author, postType, len(content), err)
			return uuid.Nil, err
		}
		return newPost.ID, nil
	}

	_, err = dbQueries.UpdatePost(ctx, database.UpdatePostParams{
		ID:           post.ID,
		LastSyncedAt: time.Now(),
		IsArchived:   false,
		Content: sql.NullString{
			String: content,
			Valid:  content != "",
		},
		PostType: postType,
		Author:   author,
	})
	if err != nil {
		log.Printf("UpdatePost failed: sourceID=%s networkInternalID=%q author=%q postType=%q contentLen=%d err=%v", sourceID, networkInternalID, author, postType, len(content), err)
		return uuid.Nil, err
	}

	return post.ID, nil
}

func ProcessScrapedPost(
	ctx context.Context,
	dbQueries *database.Queries,
	sourceID uuid.UUID,
	networkInternalID string,
	network string,
	createdAt time.Time,
	postType string,
	author string,
	content string,
	likes, reposts, views sql.NullInt64,
) error {
	postID, err := CreateOrUpdatePost(
		ctx,
		dbQueries,
		sourceID,
		networkInternalID,
		network,
		createdAt,
		postType,
		author,
		content,
	)
	if err != nil {
		return err
	}

	_, err = dbQueries.SyncReactions(ctx, database.SyncReactionsParams{
		ID:       uuid.New(),
		SyncedAt: time.Now(),
		PostID:   postID,
		Views:    views,
		Likes:    likes,
		Reposts:  reposts,
	})
	return err
}

func UpdateSourceStats(
	ctx context.Context,
	dbQueries *database.Queries,
	sourceID uuid.UUID,
	updateFn func(*ProfileStats),
) error {
	stats, err := CalculateAverageStats(ctx, dbQueries, sourceID)
	if err != nil {
		return err
	}

	if updateFn != nil {
		updateFn(stats)
	}

	return SaveOrUpdateSourceStats(ctx, dbQueries, sourceID, stats)
}

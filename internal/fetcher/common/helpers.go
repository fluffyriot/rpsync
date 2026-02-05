package common

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/google/uuid"
	"golang.org/x/net/html"
)

func StripHTMLToText(input string) string {
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

func SaveOrUpdateSourceStats(ctx context.Context, dbQueries *database.Queries, sourceID uuid.UUID, stats *ProfileStats) error {

	today := time.Now().UTC().Truncate(24 * time.Hour)

	existing, err := dbQueries.GetSourceStatsByDate(ctx, database.GetSourceStatsByDateParams{
		SourceID: sourceID,
		Date:     today,
	})

	var followersCount, followingCount, postsCount sql.NullInt32
	var avgLikes, avgReposts, avgViews sql.NullFloat64

	if stats.FollowersCount != nil {
		followersCount = sql.NullInt32{Int32: helpers.ClampToInt32(*stats.FollowersCount), Valid: true}
	}
	if stats.FollowingCount != nil {
		followingCount = sql.NullInt32{Int32: helpers.ClampToInt32(*stats.FollowingCount), Valid: true}
	}
	if stats.PostsCount != nil {
		postsCount = sql.NullInt32{Int32: helpers.ClampToInt32(*stats.PostsCount), Valid: true}
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
	post, err := dbQueries.GetPostByNetworkAndId(ctx, database.GetPostByNetworkAndIdParams{
		NetworkInternalID: networkInternalID,
		Network:           network,
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
	likes, reposts, views sql.NullInt32,
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

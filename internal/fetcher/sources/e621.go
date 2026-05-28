// SPDX-License-Identifier: AGPL-3.0-only
package sources

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"
)

type E621Post struct {
	ID          int    `json:"id"`
	CreatedAt   string `json:"created_at"`
	Description string `json:"description"`
	Score       struct {
		Total int `json:"total"`
	} `json:"score"`
	FavCount int `json:"fav_count"`
	File     struct {
		URL string `json:"url"`
	} `json:"file"`
}

type E621Response struct {
	Posts []E621Post `json:"posts"`
}

func FetchE621Posts(dbQueries *database.Queries, encryptionKey []byte, sourceId uuid.UUID, c *common.Client) error {
	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	userSource, err := dbQueries.GetSourceById(context.Background(), sourceId)
	if err != nil {
		return err
	}
	syncUsername := userSource.UserName

	apiKey, apiTokenUsername, _, _, err := authhelp.GetSourceToken(context.Background(), dbQueries, encryptionKey, sourceId)
	if err != nil {
		return fmt.Errorf("failed to get e621 credentials: %w", err)
	}

	userAgent := fmt.Sprintf("RPSync/%s (by riotphotos)", config.AppVersion)

	processedPosts := make(map[string]struct{})
	page := 1
	const maxPages = 500

	for page <= maxPages {
		time.Sleep(common.ScraperRateLimit)

		url := fmt.Sprintf("https://e621.net/posts.json?tags=user:%s&page=%d", syncUsername, page)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		req.Header.Set("User-Agent", userAgent)
		req.SetBasicAuth(apiTokenUsername, apiKey)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 503 {
			log.Printf("e621: Rate limited (503), waiting longer...")
			time.Sleep(common.RateLimitWait)
			continue
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("e621 API returned status: %d. Body: %s", resp.StatusCode, string(data))
		}

		var response E621Response
		if err := json.Unmarshal(data, &response); err != nil {
			return fmt.Errorf("failed to decode e621 response: %w", err)
		}

		if len(response.Posts) == 0 {
			break
		}

		for _, post := range response.Posts {
			postID := strconv.Itoa(post.ID)

			if _, exists := processedPosts[postID]; exists {
				continue
			}
			processedPosts[postID] = struct{}{}

			if exclusionMap[postID] {
				continue
			}

			postedAt, err := time.Parse(time.RFC3339, post.CreatedAt)
			if err != nil {
				log.Printf("e621: Failed to parse time for post %d: %v", post.ID, err)
				postedAt = time.Now()
			}

			internalID, err := common.CreateOrUpdatePost(
				context.Background(),
				dbQueries,
				sourceId,
				postID,
				"e621",
				postedAt,
				"post",
				syncUsername,
				post.Description,
			)
			if err != nil {
				log.Printf("e621: Failed to save post %s: %v", postID, err)
				continue
			}

			likes := post.Score.Total + post.FavCount

			_, err = dbQueries.SyncReactions(context.Background(), database.SyncReactionsParams{
				ID:       uuid.New(),
				SyncedAt: time.Now(),
				PostID:   internalID,
				Likes: sql.NullInt64{
					Int64: int64(likes),
					Valid: true,
				},
				Reposts: sql.NullInt64{Valid: false},
				Views:   sql.NullInt64{Valid: false},
			})
			if err != nil {
				log.Printf("e621: Failed to sync reactions for post %s: %v", postID, err)
			}
		}

		page++
	}

	if len(processedPosts) == 0 {
		return fmt.Errorf("no content found")
	}

	avgStats, err := common.CalculateAverageStats(context.Background(), dbQueries, sourceId)
	if err != nil {
		log.Printf("e621: Failed to calculate average stats: %v", err)
	} else {
		avgStats.FollowersCount = nil
		avgStats.FollowingCount = nil

		if err := common.SaveOrUpdateSourceStats(context.Background(), dbQueries, sourceId, avgStats); err != nil {
			log.Printf("e621: Failed to save stats: %v", err)
		}
	}

	return nil
}

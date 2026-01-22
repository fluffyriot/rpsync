package fetcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"

	_ "github.com/lib/pq"
)

type bskyFeed struct {
	Feed []struct {
		Post struct {
			URI    string `json:"uri"`
			Author struct {
				Handle string `json:"handle"`
			} `json:"author"`
			Record struct {
				Type      string    `json:"$type"`
				CreatedAt time.Time `json:"createdAt"`
				Embed     struct {
					Type  string `json:"$type"`
					Media struct {
						Type string `json:"$type"`
					} `json:"media"`
				} `json:"embed"`
				Text string `json:"text"`
			} `json:"record"`
			BookmarkCount int `json:"bookmarkCount"`
			ReplyCount    int `json:"replyCount"`
			RepostCount   int `json:"repostCount"`
			LikeCount     int `json:"likeCount"`
			QuoteCount    int `json:"quoteCount"`
		} `json:"post"`
		Reason struct {
			Type string `json:"$type"`
			By   struct {
				Handle string `json:"handle"`
			} `json:"by"`
		} `json:"reason,omitempty"`
	} `json:"feed"`
	Cursor string `json:"cursor,omitempty"`
}

type bskyProfile struct {
	FollowersCount int `json:"followersCount"`
	FollowsCount   int `json:"followsCount"`
	PostsCount     int `json:"postsCount"`
}

func getBskyApiString(dbQueries *database.Queries, uid uuid.UUID, cursor string) (string, string, error) {

	username, err := dbQueries.GetUserActiveSourceByName(
		context.Background(),
		database.GetUserActiveSourceByNameParams{
			UserID:  uid,
			Network: "Bluesky",
		},
	)

	if err != nil {
		return "", "", err
	}

	apiString := fmt.Sprintf(
		"https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed?actor=%s&limit=100&filter=posts_no_replies",
		username.UserName,
	)

	if cursor != "" {
		apiString += "&cursor=" + cursor
	}

	return apiString, username.UserName, nil

}

func fetchBlueskyProfile(username string, c *Client) (*bskyProfile, error) {

	url := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=%s", username)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get profile: %v %v", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var profile bskyProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

func FetchBlueskyPosts(dbQueries *database.Queries, c *Client, uid uuid.UUID, sourceId uuid.UUID) error {

	exclusionMap, err := loadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	processedLinks := make(map[string]struct{})

	const maxPages = 500

	var cursor string
	var username string
	var url string

	for page := 0; page < maxPages; page++ {

		url, username, err = getBskyApiString(dbQueries, uid, cursor)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("Failed to get a successfull response. %v: %v", resp.StatusCode, resp.Status)
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		var feed bskyFeed
		if err := json.Unmarshal(data, &feed); err != nil {
			return err
		}

		for _, item := range feed.Feed {

			uriSplit := strings.Split(item.Post.URI, "/")
			interNetId := string(uriSplit[len(uriSplit)-1])

			if _, exists := processedLinks[interNetId]; exists {
				continue
			}

			processedLinks[interNetId] = struct{}{}

			if exclusionMap[interNetId] {
				continue
			}

			post_type := "post"

			if item.Post.Record.Embed.Type == "app.bsky.embed.record" || item.Post.Record.Embed.Type == "app.bsky.embed.recordWithMedia" {
				post_type = "quote"
			}

			if item.Reason.Type == "app.bsky.feed.defs#reasonRepost" && item.Reason.By.Handle != item.Post.Author.Handle {
				post_type = "repost"
			}

			var intId uuid.UUID

			post, err := dbQueries.GetPostByNetworkAndId(context.Background(), database.GetPostByNetworkAndIdParams{
				NetworkInternalID: interNetId,
				Network:           "Bluesky",
			})

			if err != nil {
				newPost, errN := dbQueries.CreatePost(context.Background(), database.CreatePostParams{
					ID:                uuid.New(),
					CreatedAt:         item.Post.Record.CreatedAt,
					LastSyncedAt:      time.Now(),
					SourceID:          sourceId,
					IsArchived:        false,
					Author:            item.Post.Author.Handle,
					PostType:          post_type,
					NetworkInternalID: interNetId,
					Content: sql.NullString{
						String: item.Post.Record.Text,
						Valid:  true,
					},
				})
				if errN != nil {
					return errN
				}
				intId = newPost.ID
			} else {
				intId = post.ID
			}

			_, err = dbQueries.SyncReactions(context.Background(), database.SyncReactionsParams{
				ID:       uuid.New(),
				SyncedAt: time.Now(),
				PostID:   intId,
				Likes: sql.NullInt32{
					Int32: int32(item.Post.LikeCount),
					Valid: true,
				},
				Reposts: sql.NullInt32{
					Int32: int32(item.Post.RepostCount) + int32(item.Post.QuoteCount),
					Valid: true,
				},
				Views: sql.NullInt32{
					Int32: 0,
					Valid: true,
				},
			})
		}

		if feed.Cursor == "" {
			break
		}

		cursor = feed.Cursor
	}

	if len(processedLinks) == 0 {
		return errors.New("No content found")
	}

	stats, err := calculateAverageStats(context.Background(), dbQueries, sourceId)
	if err != nil {
		log.Printf("Bluesky: Failed to calculate stats for source %s: %v", sourceId, err)
	} else {

		profile, err := fetchBlueskyProfile(username, c)
		if err != nil {
			log.Printf("Bluesky: Failed to fetch profile for source %s: %v", sourceId, err)
		} else {
			stats.FollowersCount = &profile.FollowersCount
			stats.FollowingCount = &profile.FollowsCount
		}

		if err := saveOrUpdateSourceStats(context.Background(), dbQueries, sourceId, stats); err != nil {
			log.Printf("Bluesky: Failed to save stats for source %s: %v", sourceId, err)
		}
	}

	return nil

}

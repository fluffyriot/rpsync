package fetcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
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
		} `json:"reason,omitempty"`
	} `json:"feed"`
	Cursor string `json:"cursor,omitempty"`
}

func getBskyApiString(
	dbQueries *database.Queries,
	uid uuid.UUID,
	cursor string,
) (string, error) {

	username, err := dbQueries.GetUserActiveSourceByName(
		context.Background(),
		database.GetUserActiveSourceByNameParams{
			UserID:  uid,
			Network: "Bluesky",
		},
	)
	if err != nil {
		return "", err
	}

	apiString := fmt.Sprintf(
		"https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed?actor=%s&limit=100&filter=posts_no_replies",
		username.UserName,
	)

	if cursor != "" {
		apiString += "&cursor=" + cursor
	}

	return apiString, nil
}

func FetchBlueskyPosts(
	dbQueries *database.Queries,
	c *Client,
	uid uuid.UUID,
	sourceId uuid.UUID,
) error {

	processedLinks := make(map[string]struct{})

	var cursor string

	const maxPages = 500

	for page := 0; page < maxPages; page++ {

		url, err := getBskyApiString(dbQueries, uid, cursor)
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

	return nil
}

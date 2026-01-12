package fetcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/auth"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"

	_ "github.com/lib/pq"
)

type instagramFeed struct {
	Data []struct {
		ID        string `json:"id"`
		Caption   string `json:"caption"`
		Shortcode string `json:"shortcode"`
		LikeCount int    `json:"like_count"`
		Timestamp string `json:"timestamp"`
		Insights  struct {
			Data []struct {
				Values []struct {
					Value int `json:"value"`
				} `json:"values"`
			} `json:"data"`
		} `json:"insights,omitempty"`
	} `json:"data"`
	Paging struct {
		Next string `json:"next,omitempty"`
	} `json:"paging"`
}

func getInstagramApiString(
	dbQueries *database.Queries,
	sid uuid.UUID,
	next string,
	version string,
	encryptionKey []byte,
) (string, error) {

	token, err := auth.GetToken(context.Background(), dbQueries, encryptionKey, sid)
	if err != nil {
		return "", err
	}

	apiString := fmt.Sprintf("https://graph.instagram.com/v%v/me/media?fields=id,caption,shortcode,like_count,timestamp,insights.metric(views)&access_token=%v&limit=25", version, token)

	if next != "" {
		apiString = next
	}

	return apiString, nil
}

func FetchInstagramPosts(
	dbQueries *database.Queries,
	c *Client,
	sourceId uuid.UUID,
	version string,
	encryptionKey []byte,
) error {

	processedLinks := make(map[string]struct{})

	var next string

	const maxPages = 500

	for page := 0; page < maxPages; page++ {

		url, err := getInstagramApiString(dbQueries, sourceId, next, version, encryptionKey)
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

		var feed instagramFeed
		if err := json.Unmarshal(data, &feed); err != nil {
			return err
		}

		if len(feed.Data) == 0 {
			return nil
		}

		for _, item := range feed.Data {
			var intId uuid.UUID

			if _, exists := processedLinks[item.Shortcode]; exists {
				continue
			}

			processedLinks[item.Shortcode] = struct{}{}

			timeParse, _ := time.Parse("2006-01-02T15:04:05-0700", item.Timestamp)

			post, err := dbQueries.GetPostByNetworkAndId(context.Background(), database.GetPostByNetworkAndIdParams{
				NetworkInternalID: item.Shortcode,
				Network:           "Instagram",
			})

			if err != nil {
				newPost, errN := dbQueries.CreatePost(context.Background(), database.CreatePostParams{
					ID:                uuid.New(),
					CreatedAt:         timeParse,
					LastSyncedAt:      time.Now(),
					SourceID:          sourceId,
					IsArchived:        false,
					NetworkInternalID: item.Shortcode,
					Content: sql.NullString{
						String: item.Caption,
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

			views := 0

			if len(item.Insights.Data) != 0 {
				insight := item.Insights.Data[0]
				if len(insight.Values) != 0 {
					views = insight.Values[0].Value
				}
			}

			_, err = dbQueries.SyncReactions(context.Background(), database.SyncReactionsParams{
				ID:       uuid.New(),
				SyncedAt: time.Now(),
				PostID:   intId,
				Likes: sql.NullInt32{
					Int32: int32(item.LikeCount),
					Valid: true,
				},
				Reposts: sql.NullInt32{
					Int32: 0,
					Valid: true,
				},
				Views: sql.NullInt32{
					Int32: int32(views),
					Valid: true,
				},
			})
			if err != nil {
				log.Println(err)
			}
		}

		if feed.Paging.Next == "" {
			break
		}

		next = feed.Paging.Next
	}

	return nil
}

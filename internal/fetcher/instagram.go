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

	"github.com/fluffyriot/commission-tracker/internal/authhelp"
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
		MediaType string `json:"media_type"`
		Username  string `json:"username"`
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

type instagramTagsFeed struct {
	Data []struct {
		ID        string `json:"id"`
		Caption   string `json:"caption"`
		LikeCount int    `json:"like_count"`
		Timestamp string `json:"timestamp"`
		MediaType string `json:"media_type"`
		Username  string `json:"username"`
		Permalink string `json:"permalink"`
	} `json:"data"`
	Paging struct {
		Next string `json:"next,omitempty"`
	} `json:"paging"`
}

type instagramProfile struct {
	FollowsCount   int `json:"follows_count"`
	FollowersCount int `json:"followers_count"`
}

func getInstagramApiString(dbQueries *database.Queries, sid uuid.UUID, next string, version string, encryptionKey []byte) (string, string, string, string, error) {

	token, pid, _, err := authhelp.GetSourceToken(context.Background(), dbQueries, encryptionKey, sid)
	if err != nil {
		return "", "", "", "", err
	}

	apiString := fmt.Sprintf("https://graph.facebook.com/v%v/%v/media?fields=id,caption,shortcode,like_count,timestamp,media_type,username,insights.metric(views)&access_token=%v&limit=25", version, pid, token)

	if next != "" {
		apiString = next
	}

	return apiString, token, pid, version, nil

}

func getInstagramTagstring(dbQueries *database.Queries, sid uuid.UUID, next string, version string, encryptionKey []byte) (string, error) {

	token, pid, _, err := authhelp.GetSourceToken(context.Background(), dbQueries, encryptionKey, sid)
	if err != nil {
		return "", err
	}

	apiString := fmt.Sprintf("https://graph.facebook.com/v%v/%v/tags?fields=id,caption,like_count,timestamp,media_type,username,permalink&access_token=%v&limit=25", version, pid, token)

	if next != "" {
		apiString = next
	}

	return apiString, nil

}

func fetchInstagramProfile(token, pid, version string, c *Client) (*instagramProfile, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v%s/%s?fields=follows_count,followers_count&access_token=%s", version, pid, token)

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

	var profile instagramProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

func FetchInstagramPosts(dbQueries *database.Queries, c *Client, sourceId uuid.UUID, version string, encryptionKey []byte) error {

	exclusionMap, err := loadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	processedLinks := make(map[string]struct{})

	const maxPages = 500

	var next string
	var token, pid, ver string
	var url string

	for page := 0; page < maxPages; page++ {

		url, token, pid, ver, err = getInstagramApiString(dbQueries, sourceId, next, version, encryptionKey)
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
			return fmt.Errorf("Failed to get a successfull response. Code: %v. Status: %v", resp.StatusCode, resp.Status)
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

			if exclusionMap[item.Shortcode] {
				continue
			}

			timeParse, _ := time.Parse("2006-01-02T15:04:05-0700", item.Timestamp)
			post_type := strings.ToLower(item.MediaType)

			if item.MediaType == "CAROUSEL_ALBUM" {
				post_type = "image"
			}

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
					Author:            item.Username,
					PostType:          post_type,
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

		time.Sleep(300 * time.Millisecond)
	}

	log.Printf("Instagram: Processed %d posts", len(processedLinks))

	if len(processedLinks) == 0 {
		return errors.New("No content found")
	}

	stats, err := calculateAverageStats(context.Background(), dbQueries, sourceId)
	if err != nil {
		log.Printf("Instagram: Failed to calculate stats for source %s: %v", sourceId, err)
	} else {

		profile, err := fetchInstagramProfile(token, pid, ver, c)
		if err != nil {
			log.Printf("Instagram: Failed to fetch profile for source %s: %v", sourceId, err)
		} else {
			stats.FollowersCount = &profile.FollowersCount
			stats.FollowingCount = &profile.FollowsCount
		}

		if err := saveOrUpdateSourceStats(context.Background(), dbQueries, sourceId, stats); err != nil {
			log.Printf("Instagram: Failed to save stats for source %s: %v", sourceId, err)
		}
	}

	return nil

}

func FetchInstagramTags(dbQueries *database.Queries, c *Client, sourceId uuid.UUID, version string, encryptionKey []byte) error {

	exclusionMap, err := loadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	processedLinks := make(map[string]struct{})

	var next string

	const maxPages = 500

	for page := 0; page < maxPages; page++ {

		url, err := getInstagramTagstring(dbQueries, sourceId, next, version, encryptionKey)
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
			return fmt.Errorf("Failed to get a successfull response. %v: %v", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		var feed instagramTagsFeed
		if err := json.Unmarshal(data, &feed); err != nil {
			return err
		}

		if len(feed.Data) == 0 {
			return nil
		}

		for _, item := range feed.Data {
			var intId uuid.UUID

			shortcode := strings.TrimPrefix(item.Permalink, "https://www.instagram.com/p/")
			shortcode = strings.TrimSuffix(shortcode, "/")

			if _, exists := processedLinks[shortcode]; exists {
				continue
			}

			processedLinks[shortcode] = struct{}{}

			if exclusionMap[shortcode] {
				continue
			}

			timeParse, _ := time.Parse("2006-01-02T15:04:05-0700", item.Timestamp)

			post, err := dbQueries.GetPostByNetworkAndId(context.Background(), database.GetPostByNetworkAndIdParams{
				NetworkInternalID: shortcode,
				Network:           "Instagram",
			})

			if err != nil {
				newPost, errN := dbQueries.CreatePost(context.Background(), database.CreatePostParams{
					ID:                uuid.New(),
					CreatedAt:         timeParse,
					LastSyncedAt:      time.Now(),
					SourceID:          sourceId,
					IsArchived:        false,
					Author:            item.Username,
					PostType:          "tag",
					NetworkInternalID: shortcode,
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
					Int32: 0,
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

		time.Sleep(300 * time.Millisecond)
	}

	if len(processedLinks) == 0 {
		return errors.New("No content found")
	}

	return nil

}

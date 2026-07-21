// SPDX-License-Identifier: AGPL-3.0-only
package sources

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

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"

	_ "github.com/lib/pq"
)

type facebookAPIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
		Subcode int    `json:"error_subcode"`
	} `json:"error"`
}

func isFacebookSessionExpired(body []byte) bool {
	var apiErr facebookAPIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return false
	}
	return apiErr.Error.Type == "OAuthException" && apiErr.Error.Code == 190 &&
		(apiErr.Error.Subcode == 463 || apiErr.Error.Subcode == 467)
}

type instagramFeed struct {
	Data []struct {
		ID           string `json:"id"`
		Caption      string `json:"caption"`
		Shortcode    string `json:"shortcode"`
		LikeCount    int    `json:"like_count"`
		Timestamp    string `json:"timestamp"`
		MediaType    string `json:"media_type"`
		Username     string `json:"username"`
		RepostsCount *int   `json:"reposts_count"`
		SharesCount  *int   `json:"shares_count"`
		Insights     struct {
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
		ID           string `json:"id"`
		Caption      string `json:"caption"`
		LikeCount    int    `json:"like_count"`
		Timestamp    string `json:"timestamp"`
		MediaType    string `json:"media_type"`
		Username     string `json:"username"`
		Permalink    string `json:"permalink"`
		RepostsCount *int   `json:"reposts_count"`
		SharesCount  *int   `json:"shares_count"`
	} `json:"data"`
	Paging struct {
		Next string `json:"next,omitempty"`
	} `json:"paging"`
}

type instagramCollabsFeed struct {
	Data []struct {
		ID              string `json:"id"`
		Caption         string `json:"caption"`
		TotalLikeCount  int    `json:"total_like_count"`
		TotalViewsCount *int   `json:"total_views_count"`
		Timestamp       string `json:"timestamp"`
		MediaType       string `json:"media_type"`
		Username        string `json:"username"`
		Permalink       string `json:"permalink"`
		RepostsCount    *int   `json:"reposts_count"`
		SharesCount     *int   `json:"shares_count"`
	} `json:"data"`
	Paging struct {
		Next string `json:"next,omitempty"`
	} `json:"paging"`
}

type instagramProfile struct {
	FollowsCount   int `json:"follows_count"`
	FollowersCount int `json:"followers_count"`
}

func calculateReposts(repostsCount, sharesCount *int) sql.NullInt64 {
	if repostsCount == nil && sharesCount == nil {
		return sql.NullInt64{Valid: false}
	}
	total := 0
	if repostsCount != nil {
		total += *repostsCount
	}
	if sharesCount != nil {
		total += *sharesCount
	}
	return sql.NullInt64{Int64: int64(total), Valid: true}
}

func getInstagramApiString(dbQueries *database.Queries, sid uuid.UUID, next string, version string, encryptionKey []byte, noInsights bool) (string, string, string, string, error) {

	token, pid, _, _, err := authhelp.GetSourceToken(context.Background(), dbQueries, encryptionKey, sid)
	if err != nil {
		return "", "", "", "", err
	}

	var apiString string
	if noInsights {
		apiString = fmt.Sprintf("https://graph.facebook.com/%v/%v/media?fields=id,caption,shortcode,like_count,timestamp,media_type,username,reposts_count,shares_count&access_token=%v&limit=10", version, pid, token)
	} else {
		apiString = fmt.Sprintf("https://graph.facebook.com/%v/%v/media?fields=id,caption,shortcode,like_count,timestamp,media_type,username,reposts_count,shares_count,insights.metric(views)&access_token=%v&limit=10", version, pid, token)
	}

	if next != "" {
		apiString = next
	}

	return apiString, token, pid, version, nil

}

func getInstagramTagstring(dbQueries *database.Queries, sid uuid.UUID, next string, version string, encryptionKey []byte) (string, error) {

	token, pid, _, _, err := authhelp.GetSourceToken(context.Background(), dbQueries, encryptionKey, sid)
	if err != nil {
		return "", err
	}

	apiString := fmt.Sprintf("https://graph.facebook.com/%v/%v/tags?fields=id,caption,like_count,timestamp,media_type,username,permalink,reposts_count,shares_count&access_token=%v&limit=10", version, pid, token)

	if next != "" {
		apiString = next
	}

	return apiString, nil

}

func getInstagramCollabsString(dbQueries *database.Queries, sid uuid.UUID, next string, version string, encryptionKey []byte) (string, error) {

	token, pid, _, _, err := authhelp.GetSourceToken(context.Background(), dbQueries, encryptionKey, sid)
	if err != nil {
		return "", err
	}

	apiString := fmt.Sprintf("https://graph.facebook.com/%v/%v/collaborative_media?fields=id,caption,total_like_count,timestamp,media_type,username,permalink,reposts_count,shares_count,total_views_count&access_token=%v&limit=10", version, pid, token)

	if next != "" {
		apiString = next
	}

	return apiString, nil

}

func fetchInstagramProfile(token, pid, version string, c *common.Client) (*instagramProfile, error) {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s?fields=follows_count,followers_count&access_token=%s", version, pid, token)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get profile: %v %v. Body: %s", resp.StatusCode, resp.Status, string(data))
	}

	var profile instagramProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

func FetchInstagramPosts(dbQueries *database.Queries, c *common.Client, sourceId uuid.UUID, version string, encryptionKey []byte) error {

	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	processedLinks := make(map[string]struct{})

	const maxPages = 500

	var next string
	var token, pid, ver string
	var url string
	var noInsights bool

	for page := 0; page < maxPages; page++ {

		url, token, pid, ver, err = getInstagramApiString(dbQueries, sourceId, next, version, encryptionKey, noInsights)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			if strings.Contains(string(data), "business account conversion") {
				if !noInsights {
					log.Printf("Instagram: retrying without insights for source %s (pre-business-account content)", sourceId)
					noInsights = true
					next = ""
					page = -1
					continue
				}
				log.Printf("Instagram: ignoring pre-business-account conversion error for source %s (Instagram API limitation)", sourceId)
				break
			}
			if isFacebookSessionExpired(data) {
				return fmt.Errorf("Facebook session has expired — please reconnect your Instagram account via Source Settings")
			}
			return fmt.Errorf("Failed to get a successfull response. Code: %v. Status: %v. Body: %s", resp.StatusCode, resp.Status, string(data))
		}

		var feed instagramFeed
		if err := json.Unmarshal(data, &feed); err != nil {
			return err
		}

		if len(feed.Data) == 0 {
			return nil
		}

		for _, item := range feed.Data {

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

			var viewsVal sql.NullInt64
			if !noInsights {
				views := 0
				if len(item.Insights.Data) != 0 {
					insight := item.Insights.Data[0]
					if len(insight.Values) != 0 {
						views = insight.Values[0].Value
					}
				}
				viewsVal = sql.NullInt64{Int64: int64(views), Valid: true}
			}

			err = common.ProcessScrapedPost(
				context.Background(), dbQueries, sourceId, item.Shortcode, "Instagram", timeParse, post_type, item.Username, item.Caption,
				sql.NullInt64{Int64: int64(item.LikeCount), Valid: true},
				calculateReposts(item.RepostsCount, item.SharesCount),
				viewsVal,
			)
			if err != nil {
				return err
			}
		}

		if feed.Paging.Next == "" {
			break
		}

		next = feed.Paging.Next

		time.Sleep(common.APIRateLimit)
	}

	if len(processedLinks) == 0 {
		return errors.New("No content found")
	}

	if err := common.UpdateSourceStats(context.Background(), dbQueries, sourceId, func(s *common.ProfileStats) {
		profile, err := fetchInstagramProfile(token, pid, ver, c)
		if err != nil {
			log.Printf("Instagram: Failed to fetch profile for source %s: %v", sourceId, err)
		} else {
			s.FollowersCount = &profile.FollowersCount
			s.FollowingCount = &profile.FollowsCount
		}
	}); err != nil {
		log.Printf("Instagram: Failed to update stats for source %s: %v", sourceId, err)
	}

	return nil

}

func FetchInstagramCollabs(dbQueries *database.Queries, c *common.Client, sourceId uuid.UUID, version string, encryptionKey []byte) error {

	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	processedShortcodes := make(map[string]struct{})
	var next string
	const maxPages = 500

	for page := 0; page < maxPages; page++ {

		url, err := getInstagramCollabsString(dbQueries, sourceId, next, version, encryptionKey)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			if isFacebookSessionExpired(data) {
				return fmt.Errorf("Facebook session has expired — please reconnect your Instagram account via Source Settings")
			}
			return fmt.Errorf("Failed to get a successfull response. %v: %v. Body: %s", resp.StatusCode, resp.Status, string(data))
		}

		var feed instagramCollabsFeed
		if err := json.Unmarshal(data, &feed); err != nil {
			return err
		}

		if len(feed.Data) == 0 {
			break
		}

		for _, item := range feed.Data {

			shortcode := strings.TrimPrefix(item.Permalink, "https://www.instagram.com/p/")
			shortcode = strings.TrimSuffix(shortcode, "/")

			if _, exists := processedShortcodes[shortcode]; exists {
				continue
			}

			processedShortcodes[shortcode] = struct{}{}

			if exclusionMap[shortcode] {
				continue
			}

			timeParse, _ := time.Parse("2006-01-02T15:04:05-0700", item.Timestamp)

			var viewsVal sql.NullInt64
			if item.TotalViewsCount != nil {
				viewsVal = sql.NullInt64{Int64: int64(*item.TotalViewsCount), Valid: true}
			}

			err = common.ProcessScrapedPost(
				context.Background(), dbQueries, sourceId, shortcode, "Instagram", timeParse, "collab", item.Username, item.Caption,
				sql.NullInt64{Int64: int64(item.TotalLikeCount), Valid: true},
				calculateReposts(item.RepostsCount, item.SharesCount),
				viewsVal,
			)
			if err != nil {
				return err
			}
		}

		if feed.Paging.Next == "" {
			break
		}

		next = feed.Paging.Next

		time.Sleep(common.APIRateLimit)
	}

	return nil

}

func FetchInstagramTags(dbQueries *database.Queries, c *common.Client, sourceId uuid.UUID, version string, encryptionKey []byte) error {

	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceId)
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

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			if isFacebookSessionExpired(data) {
				return fmt.Errorf("Facebook session has expired — please reconnect your Instagram account via Source Settings")
			}
			return fmt.Errorf("Failed to get a successfull response. %v: %v. Body: %s", resp.StatusCode, resp.Status, string(data))
		}

		var feed instagramTagsFeed
		if err := json.Unmarshal(data, &feed); err != nil {
			return err
		}

		if len(feed.Data) == 0 {
			return nil
		}

		for _, item := range feed.Data {

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

			err = common.ProcessScrapedPost(
				context.Background(), dbQueries, sourceId, shortcode, "Instagram", timeParse, "tag", item.Username, item.Caption,
				sql.NullInt64{Int64: int64(item.LikeCount), Valid: true},
				calculateReposts(item.RepostsCount, item.SharesCount),
				sql.NullInt64{Valid: false},
			)
			if err != nil {
				return err
			}
		}

		if feed.Paging.Next == "" {
			break
		}

		next = feed.Paging.Next

		time.Sleep(common.APIRateLimit)
	}

	if len(processedLinks) == 0 {
		return errors.New("No content found")
	}

	return nil

}

package fetcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

func getTiktokString(dbQueries *database.Queries, uid uuid.UUID) (string, string, error) {
	username, err := dbQueries.GetUserActiveSourceByName(context.Background(), database.GetUserActiveSourceByNameParams{
		UserID:  uid,
		Network: "TikTok",
	},
	)
	if err != nil {
		return "", "", err
	}

	urlString := fmt.Sprintf("https://www.tiktok.com/@%s", username.UserName)
	return urlString, username.UserName, nil
}

func FetchTikTokPosts(dbQueries *database.Queries, c *Client, uid uuid.UUID, sourceId uuid.UUID) error {

	processedLinks := make(map[string]struct{})

	profileURL, username, err := getTiktokString(dbQueries, uid)
	if err != nil {
		return err
	}

	secUid, err := resolveTikTokSecUID(c, username, profileURL)
	if err != nil {
		return err
	}

	cursor := "0"
	const maxPages = 500

	for page := 0; page < maxPages; page++ {

		url := buildTikTokPostsURL(secUid, cursor)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		setTikTokHeaders(req, username, c)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("tiktok: bad status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		var feed tiktokPostFeed
		if err := json.Unmarshal(body, &feed); err != nil {
			return err
		}

		if len(feed.ItemList) == 0 {
			break
		}

		for _, item := range feed.ItemList {

			if _, exists := processedLinks[item.ID]; exists {
				continue
			}
			processedLinks[item.ID] = struct{}{}

			postType := "video"
			if item.ImagePost != nil {
				postType = "image"
			}

			createdAt := time.Unix(item.CreateTime, 0)

			content := strings.TrimSpace(item.Desc)
			if content == "" && item.ImagePost != nil {
				content = strings.TrimSpace(item.ImagePost.Title)
			}

			var intId uuid.UUID

			post, err := dbQueries.GetPostByNetworkAndId(
				context.Background(),
				database.GetPostByNetworkAndIdParams{
					NetworkInternalID: item.ID,
					Network:           "TikTok",
				},
			)

			if err != nil {
				newPost, errN := dbQueries.CreatePost(
					context.Background(),
					database.CreatePostParams{
						ID:                uuid.New(),
						CreatedAt:         createdAt,
						LastSyncedAt:      time.Now(),
						SourceID:          sourceId,
						IsArchived:        false,
						Author:            item.Author.UniqueID,
						PostType:          postType,
						NetworkInternalID: item.ID,
						Content: sql.NullString{
							String: content,
							Valid:  content != "",
						},
					},
				)
				if errN != nil {
					return errN
				}
				intId = newPost.ID
			} else {
				intId = post.ID
			}

			likes, _ := strconv.Atoi(item.Stats.DiggCount)
			reposts, _ := strconv.Atoi(item.Stats.RepostCount)
			views, _ := strconv.Atoi(item.Stats.PlayCount)
			shares, _ := strconv.Atoi(item.Stats.ShareCount)

			_, err = dbQueries.SyncReactions(
				context.Background(),
				database.SyncReactionsParams{
					ID:       uuid.New(),
					SyncedAt: time.Now(),
					PostID:   intId,
					Likes: sql.NullInt32{
						Int32: int32(likes),
						Valid: true,
					},
					Reposts: sql.NullInt32{
						Int32: int32(reposts + shares),
						Valid: true,
					},
					Views: sql.NullInt32{
						Int32: int32(views),
						Valid: true,
					},
				},
			)
			if err != nil {
				return err
			}
		}

		if !feed.HasMore {
			break
		}

		cursor = feed.Cursor
		time.Sleep(600 * time.Millisecond)
	}

	if len(processedLinks) == 0 {
		return errors.New("tiktok: no content found")
	}

	return nil
}

func resolveTikTokSecUID(c *Client, username, profileURL string) (string, error) {
	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		return "", err
	}

	setTikTokHeaders(req, username, c)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if cookies := resp.Header["Set-Cookie"]; len(cookies) > 0 {
		c.tikTokCookie = strings.Join(cookies, "; ")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`"secUid":"([^"]+)"`)
	matches := re.FindSubmatch(body)
	if len(matches) < 2 {
		return "", errors.New("tiktok: secUid not found")
	}

	return string(matches[1]), nil
}

func buildTikTokPostsURL(secUid, cursor string) string {
	return fmt.Sprintf(
		"https://www.tiktok.com/api/post/item_list/?aid=1988&secUid=%s&cursor=%s&count=30",
		secUid,
		cursor,
	)
}

func setTikTokHeaders(req *http.Request, username string, c *Client) {
	req.Host = "www.tiktok.com"

	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	)
	req.Header.Set(
		"Accept",
		"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", fmt.Sprintf("https://www.tiktok.com/@%s", username))

	if c.tikTokCookie != "" {
		req.Header.Set("Cookie", c.tikTokCookie)
	}
}

type tiktokPostFeed struct {
	ItemList []tiktokPost `json:"itemList"`
	Cursor   string       `json:"cursor"`
	HasMore  bool         `json:"hasMore"`
}

type tiktokPost struct {
	ID         string `json:"id"`
	Desc       string `json:"desc"`
	CreateTime int64  `json:"createTime"`

	CategoryType int `json:"CategoryType"`

	ImagePost *struct {
		Title string `json:"title"`
	} `json:"imagePost"`

	Author struct {
		UniqueID string `json:"uniqueId"`
	} `json:"author"`

	Stats struct {
		DiggCount   string `json:"diggCount"`
		ShareCount  string `json:"shareCount"`
		PlayCount   string `json:"playCount"`
		RepostCount string `json:"repostCount"`
	} `json:"statsV2"`
}

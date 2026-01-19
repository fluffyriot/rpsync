package fetcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

type mastUser struct {
	ID string `json:"id"`
}

type mastFeed []struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	FavouritesCount int       `json:"favourites_count"`
	ReblogsCount    int       `json:"reblogs_count"`
	QuotesCount     int       `json:"quotes_count"`
	Content         string    `json:"content"`
	Account         struct {
		Id  string `json:"id"`
		Uri string `json:"uri"`
	} `json:"account"`
	Reblog *struct {
		ID              string    `json:"id"`
		Uri             string    `json:"uri"`
		CreatedAt       time.Time `json:"created_at"`
		FavouritesCount int       `json:"favourites_count"`
		ReblogsCount    int       `json:"reblogs_count"`
		QuotesCount     int       `json:"quotes_count"`
		Content         string    `json:"content"`
		Account         struct {
			Id  string `json:"id"`
			Uri string `json:"uri"`
		} `json:"account"`
	} `json:"reblog"`
}

func getMastodonApiString(dbQueries *database.Queries, uid uuid.UUID, c *Client, max_id string) (string, error) {

	username, err := dbQueries.GetUserActiveSourceByName(
		context.Background(),
		database.GetUserActiveSourceByNameParams{
			UserID:  uid,
			Network: "Mastodon",
		},
	)

	if err != nil {
		return "", err
	}

	splits := strings.SplitN(username.UserName, "@", 2)
	user := splits[0]
	domain := splits[1]

	initUrl := fmt.Sprintf(
		"https://%s/api/v1/accounts/lookup?acct=%s",
		domain,
		user,
	)

	req, err := http.NewRequest("GET", initUrl, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Failed to get a successfull response. %v: %v", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", err
	}

	var mastUser mastUser
	if err := json.Unmarshal(data, &mastUser); err != nil {
		return "", err
	}

	apiString := fmt.Sprintf(
		"https://%s/api/v1/accounts/%s/statuses?only_media=false&exclude_reblogs=false&exclude_replies=true&limit=40",
		domain,
		mastUser.ID,
	)

	if max_id != "" {
		apiString += "&max_id=" + max_id
	}

	return apiString, nil

}

func FetchMastodonPosts(dbQueries *database.Queries, c *Client, uid uuid.UUID, sourceId uuid.UUID) error {

	processedLinks := make(map[string]struct{})

	var max_id string

	const maxPages = 500

	for page := 0; page < maxPages; page++ {

		urlReq, err := getMastodonApiString(dbQueries, uid, c, max_id)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("GET", urlReq, nil)
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

		var feed mastFeed
		if err := json.Unmarshal(data, &feed); err != nil {
			return err
		}

		if len(feed) == 0 {
			return nil
		}

		for _, item := range feed {

			max_id = item.ID

			var postId string

			if item.Reblog != nil {
				if item.Reblog.Account.Id == item.Account.Id {
					continue
				}
				postId = strings.Split(item.Reblog.Uri, "statuses/")[1]
			} else {
				postId = item.ID
			}

			if _, exists := processedLinks[postId]; exists {
				continue
			}

			processedLinks[postId] = struct{}{}

			var intId uuid.UUID

			post, err := dbQueries.GetPostByNetworkAndId(context.Background(), database.GetPostByNetworkAndIdParams{
				NetworkInternalID: postId,
				Network:           "Mastodon",
			})

			var postDb database.CreatePostParams
			var postReactions database.SyncReactionsParams

			if item.Reblog != nil {
				content := stripHTMLToText(item.Reblog.Content)

				u, errU := url.Parse(item.Reblog.Account.Uri)
				if errU != nil {
					return errU
				}

				username := path.Base(u.Path)
				domain := u.Host

				postDb = database.CreatePostParams{
					ID:                uuid.New(),
					CreatedAt:         item.Reblog.CreatedAt,
					LastSyncedAt:      time.Now(),
					SourceID:          sourceId,
					IsArchived:        false,
					Author:            fmt.Sprintf("%s@%s", username, domain),
					PostType:          "repost",
					NetworkInternalID: postId,
					Content: sql.NullString{
						String: content,
						Valid:  true,
					},
				}

				postReactions = database.SyncReactionsParams{
					ID:       uuid.New(),
					SyncedAt: time.Now(),
					Likes: sql.NullInt32{
						Int32: int32(item.Reblog.FavouritesCount),
						Valid: true,
					},
					Reposts: sql.NullInt32{
						Int32: int32(item.Reblog.QuotesCount) + int32(item.Reblog.ReblogsCount),
						Valid: true,
					},
					Views: sql.NullInt32{
						Int32: 0,
						Valid: true,
					},
				}
			} else {
				content := stripHTMLToText(item.Content)

				u, errU := url.Parse(item.Account.Uri)
				if errU != nil {
					return errU
				}

				username := path.Base(u.Path)
				domain := u.Host

				postDb = database.CreatePostParams{
					ID:                uuid.New(),
					CreatedAt:         item.CreatedAt,
					LastSyncedAt:      time.Now(),
					SourceID:          sourceId,
					IsArchived:        false,
					Author:            fmt.Sprintf("%s@%s", username, domain),
					PostType:          "post",
					NetworkInternalID: postId,
					Content: sql.NullString{
						String: content,
						Valid:  true,
					},
				}

				postReactions = database.SyncReactionsParams{
					ID:       uuid.New(),
					SyncedAt: time.Now(),
					Likes: sql.NullInt32{
						Int32: int32(item.FavouritesCount),
						Valid: true,
					},
					Reposts: sql.NullInt32{
						Int32: int32(item.QuotesCount) + int32(item.ReblogsCount),
						Valid: true,
					},
					Views: sql.NullInt32{
						Int32: 0,
						Valid: true,
					},
				}
			}

			if err != nil {
				newPost, errN := dbQueries.CreatePost(context.Background(), postDb)
				if errN != nil {
					return errN
				}
				intId = newPost.ID
			} else {
				intId = post.ID
			}

			postReactions.PostID = intId
			_, err = dbQueries.SyncReactions(context.Background(), postReactions)

		}

	}

	if len(processedLinks) == 0 {
		return fmt.Errorf("No content found")
	}

	return nil

}

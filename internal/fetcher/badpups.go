package fetcher

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

func getBadpupsString(
	dbQueries *database.Queries,
	uid uuid.UUID,
) (string, error) {

	username, err := dbQueries.GetUserActiveSourceByName(
		context.Background(),
		database.GetUserActiveSourceByNameParams{
			UserID:  uid,
			Network: "BadPups",
		},
	)
	if err != nil {
		return "", err
	}

	urlString := fmt.Sprintf(
		"https://badpups.com/lite/profile/%s/",
		username.UserName,
	)

	return urlString, nil
}

func FetchBadpupsPosts(uid uuid.UUID, dbQueries *database.Queries, c *Client, sourceId uuid.UUID) error {
	processedLinks := make(map[string]struct{})

	profileURL, err := getBadpupsString(dbQueries, uid)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	linkPattern := regexp.MustCompile(`^https?://[^/]+/lite/video/[^/]+$`)

	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		var intId uuid.UUID
		href, exists := s.Attr("href")
		if !exists || !linkPattern.MatchString(href) {
			return
		}

		if _, exists := processedLinks[href]; exists {
			return
		}

		processedLinks[href] = struct{}{}

		videoReq, err := http.NewRequest("GET", href, nil)
		if err != nil {
			return
		}
		videoReq.Header.Set(
			"User-Agent",
			"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		)
		videoReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		videoReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

		videoResp, err := c.httpClient.Do(videoReq)
		if err != nil {
			return
		}

		videoDoc, err := goquery.NewDocumentFromReader(videoResp.Body)
		if err != nil {
			return
		}

		pageText := videoDoc.Text()

		videoViews, _ := extractMurrNumber(pageText, `([\d]+)\s*views`)

		likesText := strings.TrimSpace(
			videoDoc.Find("span.likes_count").First().Text(),
		)

		dislikesText := strings.TrimSpace(
			videoDoc.Find("span.dislikes_count").First().Text(),
		)

		likes := 0
		dislikes := 0

		if likesText != "" {
			likes, _ = strconv.Atoi(likesText)
		}

		if dislikesText != "" {
			dislikes, _ = strconv.Atoi(dislikesText)
		}

		videoLikes := likes - dislikes

		id := strings.TrimPrefix(href, "https://badpups.com/lite/video/")

		title, _ := videoDoc.Find(`meta[property="og:title"]`).Attr("content")
		description, _ := videoDoc.Find(`meta[property="og:description"]`).Attr("content")

		post, err := dbQueries.GetPostByNetworkAndId(context.Background(), database.GetPostByNetworkAndIdParams{
			NetworkInternalID: id,
			Network:           "BadPups",
		})

		if err != nil {
			newPost, _ := dbQueries.CreatePost(context.Background(), database.CreatePostParams{
				ID:                uuid.New(),
				CreatedAt:         time.Now(),
				LastSyncedAt:      time.Now(),
				SourceID:          sourceId,
				IsArchived:        false,
				NetworkInternalID: id,
				Content: sql.NullString{
					String: fmt.Sprintf("%s\n\n%s", title, description),
					Valid:  true,
				},
			})
			intId = newPost.ID
		} else {
			intId = post.ID
		}

		_, err = dbQueries.SyncReactions(context.Background(), database.SyncReactionsParams{
			ID:       uuid.New(),
			SyncedAt: time.Now(),
			PostID:   intId,
			Likes: sql.NullInt32{
				Int32: int32(videoLikes),
				Valid: true,
			},
			Reposts: sql.NullInt32{
				Int32: 0,
				Valid: true,
			},
			Views: sql.NullInt32{
				Int32: int32(videoViews),
				Valid: true,
			},
		})

	})

	return nil
}

func extractBpNumber(text, pattern string) (int, error) {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, fmt.Errorf("nothing matched")
	}

	clean := strings.ReplaceAll(match[1], ",", "")
	value, err := strconv.Atoi(clean)
	if err != nil {
		return 0, nil
	}
	return value, nil
}

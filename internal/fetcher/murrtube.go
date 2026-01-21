package fetcher

import (
	"context"
	"database/sql"
	"errors"
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

func getMurrtubeString(dbQueries *database.Queries, uid uuid.UUID) (string, string, error) {

	username, err := dbQueries.GetUserActiveSourceByName(
		context.Background(),
		database.GetUserActiveSourceByNameParams{
			UserID:  uid,
			Network: "Murrtube",
		},
	)
	if err != nil {
		return "", "", err
	}

	urlString := fmt.Sprintf(
		"https://murrtube.net/%s",
		username.UserName,
	)

	return urlString, username.UserName, nil
}

func FetchMurrtubePosts(uid uuid.UUID, dbQueries *database.Queries, c *Client, sourceId uuid.UUID) error {

	exclusionMap, err := loadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	processedLinks := make(map[string]struct{})

	profileURL, username, err := getMurrtubeString(dbQueries, uid)
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

	linkPattern := regexp.MustCompile(`^/v/.{4}$`)

	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		var intId uuid.UUID
		href, exists := s.Attr("href")
		if !exists || !linkPattern.MatchString(href) {
			return
		}

		videoURL := "https://murrtube.net" + href
		if _, exists := processedLinks[videoURL]; exists {
			return
		}

		processedLinks[videoURL] = struct{}{}

		videoReq, err := http.NewRequest("GET", videoURL, nil)
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

		id := strings.TrimPrefix(href, "/v/")

		if exclusionMap[id] {
			return
		}

		title, _ := videoDoc.Find(`meta[property="og:title"]`).Attr("content")
		description, _ := videoDoc.Find(`meta[property="og:description"]`).Attr("content")

		createdAt, err := extractMurrtubeCreatedAt(videoDoc)
		if err != nil {
			createdAt = time.Now()
		}

		post, err := dbQueries.GetPostByNetworkAndId(context.Background(), database.GetPostByNetworkAndIdParams{
			NetworkInternalID: id,
			Network:           "Murrtube",
		})

		if err != nil {
			newPost, _ := dbQueries.CreatePost(context.Background(), database.CreatePostParams{
				ID:                uuid.New(),
				CreatedAt:         createdAt,
				LastSyncedAt:      time.Now(),
				SourceID:          sourceId,
				PostType:          "video",
				Author:            username,
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

		videoViews, _ := extractMurrNumber(pageText, `([\d,]+)\s+Views`)
		videoLikes, _ := extractMurrNumber(pageText, `([\d,]+)\s+Likes`)

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

	if len(processedLinks) == 0 {
		return errors.New("No content found")
	}

	return nil
}

func extractMurrtubeCreatedAt(doc *goquery.Document) (time.Time, error) {
	span := doc.Find(`span[data-tooltip]`).First()
	if span.Length() == 0 {
		return time.Time{}, errors.New("created date not found")
	}

	raw, exists := span.Attr("data-tooltip")
	if !exists || strings.TrimSpace(raw) == "" {
		return time.Time{}, errors.New("created date empty")
	}

	t, err := time.Parse("January 2, 2006 - 15:04", raw)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

func extractMurrNumber(text, pattern string) (int, error) {
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

// SPDX-License-Identifier: AGPL-3.0-only
package sources

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"
)

type VideoObjectLD struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UploadDate  string `json:"uploadDate"`
}

func buildHashtags(genres []string, keywords string) string {
	seen := make(map[string]struct{})
	var tags []string

	normalize := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.ReplaceAll(s, " ", "-")
		return s
	}

	add := func(s string) {
		tag := normalize(s)
		if tag == "" {
			return
		}
		if _, exists := seen[tag]; !exists {
			seen[tag] = struct{}{}
			tags = append(tags, "#"+tag)
		}
	}

	for _, g := range genres {
		add(g)
	}
	if keywords != "" {
		for _, k := range strings.Split(keywords, ",") {
			add(k)
		}
	}

	return strings.Join(tags, " ")
}

func getBadpupsString(dbQueries *database.Queries, sourceId uuid.UUID) (string, string, error) {

	username, err := dbQueries.GetSourceById(
		context.Background(),
		sourceId,
	)
	if err != nil {
		return "", "", err
	}

	urlString := fmt.Sprintf(
		"https://badpups.com/lite/profile/%s/",
		username.UserName,
	)

	return urlString, username.UserName, nil

}

func extractVideoObjectLD(doc *goquery.Document) (*VideoObjectLD, error) {
	var result *VideoObjectLD

	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return true
		}

		var probe map[string]any
		if err := json.Unmarshal([]byte(raw), &probe); err != nil {
			return true
		}

		if t, ok := probe["@type"].(string); ok && t == "VideoObject" {
			var video VideoObjectLD
			if err := json.Unmarshal([]byte(raw), &video); err == nil {
				result = &video
				return false
			}
		}

		return true
	})

	if result == nil {
		return nil, errors.New("VideoObject JSON-LD not found")
	}

	return result, nil
}

func FetchBadpupsPosts(uid uuid.UUID, dbQueries *database.Queries, c *common.Client, sourceId uuid.UUID) error {

	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	processedLinks := make(map[string]struct{})

	profileURL, username, err := getBadpupsString(dbQueries, sourceId)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set(
		"User-Agent",
		common.UserAgentBingbot,
	)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	var followersCount *int
	doc.Find("div.profile-stat").Each(func(_ int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Find("div.stat-label").Text())
		if label == "Followers" {
			numText := strings.TrimSpace(s.Find("div.stat-num").Text())
			if num, err := strconv.Atoi(numText); err == nil {
				followersCount = &num
			}
		}
	})

	linkPattern := regexp.MustCompile(`^https?://[^/]+/lite/video/[^/]+$`)
	viewsRe := regexp.MustCompile(`([\d,]+)\s+views`)

	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
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
			common.UserAgentBingbot,
		)
		videoReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		videoReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

		videoResp, err := c.HTTPClient.Do(videoReq)
		if err != nil {
			return
		}

		videoDoc, err := goquery.NewDocumentFromReader(videoResp.Body)
		if err != nil {
			return
		}

		id := strings.TrimPrefix(href, "https://badpups.com/lite/video/")

		if exclusionMap[id] {
			return
		}

		videoLD, err := extractVideoObjectLD(videoDoc)
		if err != nil {
			return
		}

		title := videoLD.Name
		description := videoLD.Description

		uploadTime, err := time.Parse(time.RFC3339, videoLD.UploadDate)
		if err != nil {
			uploadTime = time.Now()
		}

		var categories []string
		videoDoc.Find("div.post-categories a.taxonomy-label").Each(func(_ int, s *goquery.Selection) {
			if t := strings.TrimSpace(s.Text()); t != "" {
				categories = append(categories, t)
			}
		})

		var tags []string
		videoDoc.Find("div.post-tags a.taxonomy-label").Each(func(_ int, s *goquery.Selection) {
			if t := strings.TrimSpace(s.Text()); t != "" {
				tags = append(tags, t)
			}
		})

		hashtags := buildHashtags(categories, strings.Join(tags, ","))
		content := fmt.Sprintf("%s\n\n%s", title, description)
		if hashtags != "" {
			content += "\n\n" + hashtags
		}

		postID, err := common.CreateOrUpdatePost(
			context.Background(),
			dbQueries,
			sourceId,
			id,
			"BadPups",
			uploadTime,
			"video",
			username,
			content,
		)
		if err != nil {
			return
		}

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

		videoViews := 0
		videoDoc.Find("div span").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if m := viewsRe.FindStringSubmatch(s.Text()); m != nil {
				videoViews, _ = strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
				return false
			}
			return true
		})

		_, err = dbQueries.SyncReactions(context.Background(), database.SyncReactionsParams{
			ID:       uuid.New(),
			SyncedAt: time.Now(),
			PostID:   postID,
			Likes: sql.NullInt64{
				Int64: int64(videoLikes),
				Valid: true,
			},
			Reposts: sql.NullInt64{
				Valid: false,
			},
			Views: sql.NullInt64{
				Int64: int64(videoViews),
				Valid: true,
			},
		})

	})

	if len(processedLinks) == 0 {
		return errors.New("No content found")
	}

	stats, err := common.CalculateAverageStats(context.Background(), dbQueries, sourceId)
	if err != nil {
		log.Printf("BadPups: Failed to calculate stats for source %s: %v", sourceId, err)
	} else {
		stats.FollowersCount = followersCount
		if err := common.SaveOrUpdateSourceStats(context.Background(), dbQueries, sourceId, stats); err != nil {
			log.Printf("BadPups: Failed to save stats for source %s: %v", sourceId, err)
		}
	}

	return nil

}

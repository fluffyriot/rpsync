// SPDX-License-Identifier: AGPL-3.0-only
package sources

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"
)

type murrCookieEntry struct {
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	Domain         string   `json:"domain"`
	Path           string   `json:"path"`
	Secure         bool     `json:"secure"`
	HTTPOnly       bool     `json:"httpOnly"`
	SameSite       *string  `json:"sameSite"`
	ExpirationDate *float64 `json:"expirationDate"`
}

type murrPageData struct {
	Props murrProps `json:"props"`
}

type murrProps struct {
	Medium murrMedium `json:"medium"`
}

type murrMedium struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	ViewsCount  int       `json:"views_count"`
	LikesCount  int       `json:"likes_count"`
	Tags        []murrTag `json:"tags"`
}

type murrTag struct {
	Name string `json:"name"`
}

func FetchMurrtubePosts(dbQueries *database.Queries, c *common.Client, sourceID uuid.UUID, encryptionKey []byte) error {
	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceID)
	if err != nil {
		return err
	}

	source, err := dbQueries.GetSourceById(context.Background(), sourceID)
	if err != nil {
		return err
	}
	username := source.UserName

	cookieJSON, _, _, _, err := authhelp.GetSourceToken(context.Background(), dbQueries, encryptionKey, sourceID)
	if err != nil {
		return fmt.Errorf("failed to load murrtube credentials: %w", err)
	}

	var cookies []murrCookieEntry
	if err := json.Unmarshal([]byte(cookieJSON), &cookies); err != nil {
		return fmt.Errorf("failed to parse murrtube cookie JSON: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	defer ctxCancel()

	if err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			for _, cookie := range cookies {
				params := network.SetCookie(cookie.Name, cookie.Value).
					WithDomain(cookie.Domain).
					WithPath(cookie.Path).
					WithSecure(cookie.Secure).
					WithHTTPOnly(cookie.HTTPOnly)

				if cookie.SameSite != nil {
					switch strings.ToLower(*cookie.SameSite) {
					case "no_restriction", "none":
						params = params.WithSameSite(network.CookieSameSiteNone)
					case "lax":
						params = params.WithSameSite(network.CookieSameSiteLax)
					case "strict":
						params = params.WithSameSite(network.CookieSameSiteStrict)
					}
				}

				if cookie.ExpirationDate != nil {
					ts := cdp.TimeSinceEpoch(time.Unix(int64(*cookie.ExpirationDate), 0))
					params = params.WithExpires(&ts)
				}

				if err := params.Do(ctx); err != nil {
					return err
				}
			}
			return nil
		}),
	); err != nil {
		return fmt.Errorf("murrtube: failed to initialise browser: %w", err)
	}

	profileURL := fmt.Sprintf("https://murrtube.net/%s", username)
	if err := chromedp.Run(ctx,
		chromedp.Navigate(profileURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return fmt.Errorf("Murrtube: failed to navigate to profile: %w", err)
	}

	const maxScrolls = 30
	sameHeightCount := 0
	var prevHeight int64
	for i := 0; i < maxScrolls; i++ {
		var height int64
		chromedp.Run(ctx, chromedp.Evaluate(`document.body.scrollHeight`, &height)) //nolint:errcheck
		if height == prevHeight {
			sameHeightCount++
			if sameHeightCount >= 3 {
				break
			}
		} else {
			sameHeightCount = 0
		}
		prevHeight = height
		chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil)) //nolint:errcheck
		time.Sleep(common.ScraperRateLimit)
	}

	var pageHTML string
	if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &pageHTML)); err != nil {
		return fmt.Errorf("Murrtube: failed to get page HTML: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(pageHTML))
	if err != nil {
		return fmt.Errorf("Murrtube: failed to parse profile HTML: %w", err)
	}

	var followersCount, followingCount *int
	doc.Find("ul li a").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.HasPrefix(text, "Followers") {
			if count, err := extractMurrNumber(text, `Followers \((\d+)\)`); err == nil {
				followersCount = &count
			}
		} else if strings.HasPrefix(text, "Following") {
			if count, err := extractMurrNumber(text, `Following \((\d+)\)`); err == nil {
				followingCount = &count
			}
		}
	})

	linkPattern := regexp.MustCompile(`^/v/.{4}$`)
	var videoIDs []string
	seen := make(map[string]struct{})
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || !linkPattern.MatchString(href) {
			return
		}
		id := strings.TrimPrefix(href, "/v/")
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		videoIDs = append(videoIDs, id)
	})

	if len(videoIDs) == 0 {
		return errors.New("no videos found: session may have expired or age check cookie missing")
	}

	for _, id := range videoIDs {
		if exclusionMap[id] {
			continue
		}

		videoURL := "https://murrtube.net/v/" + id
		var videoHTML string
		if err := chromedp.Run(ctx,
			chromedp.Navigate(videoURL),
			chromedp.Sleep(2*time.Second),
			chromedp.OuterHTML("html", &videoHTML),
		); err != nil {
			log.Printf("Murrtube: failed to navigate to video %s: %v", id, err)
			continue
		}

		videoDoc, err := goquery.NewDocumentFromReader(strings.NewReader(videoHTML))
		if err != nil {
			log.Printf("Murrtube: failed to parse video page %s: %v", id, err)
			continue
		}

		dataPage, exists := videoDoc.Find("#app").Attr("data-page")
		if !exists {
			log.Printf("Murrtube: data-page attribute not found for video %s", id)
			continue
		}

		var pageData murrPageData
		if err := json.Unmarshal([]byte(dataPage), &pageData); err != nil {
			log.Printf("Murrtube: failed to parse data-page JSON for video %s: %v", id, err)
			continue
		}

		medium := pageData.Props.Medium
		createdAt := medium.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}

		tagParts := make([]string, 0, len(medium.Tags))
		for _, tag := range medium.Tags {
			tagParts = append(tagParts, "#"+tag.Name)
		}
		content := medium.Title + "\n\n" + medium.Description
		if len(tagParts) > 0 {
			content += "\n\n" + strings.Join(tagParts, " ")
		}

		postID, err := common.CreateOrUpdatePost(
			context.Background(),
			dbQueries,
			sourceID,
			id,
			"Murrtube",
			createdAt,
			"video",
			username,
			content,
		)
		if err != nil {
			log.Printf("Murrtube: failed to process video %s: %v", id, err)
			continue
		}

		videoViews := medium.ViewsCount
		videoLikes := medium.LikesCount

		if _, err = dbQueries.SyncReactions(context.Background(), database.SyncReactionsParams{
			ID:       uuid.New(),
			SyncedAt: time.Now(),
			PostID:   postID,
			Likes:    sql.NullInt64{Int64: int64(videoLikes), Valid: true},
			Reposts:  sql.NullInt64{Valid: false},
			Views:    sql.NullInt64{Int64: int64(videoViews), Valid: true},
		}); err != nil {
			log.Printf("Murrtube: failed to sync reactions for video %s: %v", id, err)
		}
	}

	if err := common.UpdateSourceStats(context.Background(), dbQueries, sourceID, func(s *common.ProfileStats) {
		s.FollowersCount = followersCount
		s.FollowingCount = followingCount
	}); err != nil {
		log.Printf("Murrtube: failed to update stats for source %s: %v", sourceID, err)
	}

	return nil
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

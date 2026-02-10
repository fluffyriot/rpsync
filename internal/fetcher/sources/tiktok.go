// SPDX-License-Identifier: AGPL-3.0-only
package sources

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"
)

type ScrapedPost struct {
	ID        string `json:"id"`
	Desc      string `json:"desc"`
	CoverURL  string `json:"cover_url"`
	DateText  string `json:"date_text"`
	Type      string `json:"type"`
	Views     string `json:"views"`
	Likes     string `json:"likes"`
	IsScraped bool   `json:"is_scraped"`
}

func FetchTikTokPosts(dbQueries *database.Queries, c *common.Client, uid uuid.UUID, sourceId uuid.UUID) error {

	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	source, err := dbQueries.GetSourceById(context.Background(), sourceId)
	if err != nil {
		return fmt.Errorf("failed to get source: %w", err)
	}

	username := source.UserName

	cookies, err := loadCookies(username)
	if err != nil {
		return fmt.Errorf("failed to load cookies for %s: %w. Please re-authenticate", username, err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			for _, cookie := range cookies {
				err := network.SetCookie(cookie.Name, cookie.Value).
					WithDomain(cookie.Domain).
					WithPath(cookie.Path).
					WithSecure(cookie.Secure).
					WithHTTPOnly(cookie.HTTPOnly).
					WithSameSite(cookie.SameSite).
					Do(ctx)
				if err != nil {
					return err
				}
			}
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to set cookies: %w", err)
	}

	url := "https://www.tiktok.com/tiktokstudio/content"

	uniquePosts := make(map[string]ScrapedPost)

	scrapeJs := `
		(function() {
			const posts = [];
			const rows = document.querySelectorAll('div[data-tt="components_RowLayout_FlexRow"]');
			rows.forEach(row => {
				try {
					const linkEl = row.querySelector('a[data-tt="components_PostInfoCell_a"]');
					if (!linkEl) return;
					const href = linkEl.getAttribute('href');
					const idMatch = href.match(/\/video\/(\d+)/);
					const id = idMatch ? idMatch[1] : "";
					const desc = linkEl.innerText;
					
					const imgEl = row.querySelector('img[data-tt="VideoCover_index_Image"]');
					const coverUrl = imgEl ? imgEl.src : "";
					
					const dateEl = row.querySelector('div[data-tt="components_PublishStageLabel_FlexCenter"]');
					const dateText = dateEl ? dateEl.innerText : "";
					
					const statElements = row.querySelectorAll('div[data-tt="components_ItemRow_FlexCenter"] span.TUXText');
					let views = "0";
					let likes = "0";
					
					if (statElements.length >= 3) {
						views = statElements[0].innerText;
						likes = statElements[1].innerText;
					}

					let type = "video";
					const imageIcon = row.querySelector('span[data-icon="ImageFill"]');
					if (imageIcon) {
						type = "photo";
					}
					
					posts.push({ id, desc, cover_url: coverUrl, date_text: dateText, views, likes, is_scraped: true, type });
				} catch (e) {
					// ignore
				}
			});
			return posts;
		})()
	`

	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var currentURL string
			if err := chromedp.Location(&currentURL).Do(ctx); err != nil {
				return err
			}
			if strings.Contains(currentURL, "login") || strings.Contains(currentURL, "signup") {
				return fmt.Errorf("session expired: redirected to login page")
			}

			var loginElementExists bool
			ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_ = chromedp.Run(ctxTimeout,
				chromedp.WaitVisible(`div[data-e2e="login-modal"]`, chromedp.ByQuery),
			)

			if err := chromedp.Evaluate(`document.querySelector('div[data-e2e="login-modal"]') !== null`, &loginElementExists).Do(ctx); err != nil {

			}
			if loginElementExists {
				return fmt.Errorf("session expired: login modal detected")
			}

			var previousPostCount int
			sameCountIterations := 0
			maxScrolls := 50

			for i := 0; i < maxScrolls; i++ {
				var batch []ScrapedPost
				if err := chromedp.Evaluate(scrapeJs, &batch).Do(ctx); err != nil {
					log.Printf("Error scraping batch %d: %v", i, err)
				} else {
					newCount := 0
					for _, p := range batch {
						if _, exists := uniquePosts[p.ID]; !exists && p.ID != "" {
							uniquePosts[p.ID] = p
							newCount++
						}
					}
				}

				currentPostCount := len(uniquePosts)
				if currentPostCount == previousPostCount {
					sameCountIterations++
					if sameCountIterations >= 5 {
						break
					}
				} else {
					sameCountIterations = 0
				}
				previousPostCount = currentPostCount

				if err := chromedp.Evaluate(`
					(function(){
						const rows = document.querySelectorAll('div[data-tt="components_RowLayout_FlexRow"]');
						if (rows.length > 0) {
							rows[rows.length-1].scrollIntoView({behavior: "smooth", block: "end"});
						}
					})()
				`, nil).Do(ctx); err != nil {
					log.Printf("Scroll failed: %v", err)
				}

				time.Sleep(2 * time.Second)
			}
			return nil
		}),
	)

	scrapedPosts := make([]ScrapedPost, 0, len(uniquePosts))
	for _, p := range uniquePosts {
		scrapedPosts = append(scrapedPosts, p)
	}

	if err != nil {
		return fmt.Errorf("scraping failed: %w", err)
	}

	if len(scrapedPosts) == 0 {
		return fmt.Errorf("no posts found: login might have expired")
	}

	for _, item := range scrapedPosts {
		if item.ID == "" {
			continue
		}

		if exclusionMap[item.ID] {
			continue
		}

		createdAt, err := parseDate(item.DateText)
		if err != nil {
			createdAt = extractTimestampFromID(item.ID)
		}

		content := item.Desc
		postType := item.Type

		viewsCount := parseCount(item.Views)
		likesCount := parseCount(item.Likes)

		err = common.ProcessScrapedPost(
			context.Background(), dbQueries, sourceId, item.ID, "TikTok", createdAt, postType, username, content,
			sql.NullInt64{Int64: int64(likesCount), Valid: likesCount >= 0},
			sql.NullInt64{Int64: 0, Valid: false},
			sql.NullInt64{Int64: int64(viewsCount), Valid: viewsCount > 0},
		)
		if err != nil {
			log.Printf("Failed to process post %s: %v", item.ID, err)
			continue
		}
	}

	var followersCount *int
	err = chromedp.Run(ctx,
		chromedp.Navigate("https://www.tiktok.com/tiktokstudio/analytics/followers"),
		chromedp.Sleep(3*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var followerText string
			err := chromedp.Evaluate(`
				(function() {
					const cards = document.querySelectorAll('div[data-tt="components_AnalyticsCard_CardWrapper"]');
					for (const card of cards) {
						const text = card.innerText;
						if (text.includes('Total followers')) {
							const valueSpan = card.querySelector('span.absolute-value');
							if (valueSpan) {
								return valueSpan.innerText;
							}
						}
					}
					return '';
				})()
			`, &followerText).Do(ctx)

			if err == nil && followerText != "" {
				count := parseCount(followerText)
				followersCount = &count
			}
			return nil
		}),
	)
	if err != nil {
		log.Printf("TikTok: Failed to scrape follower count: %v", err)
	}

	if followersCount == nil {
		return fmt.Errorf("login might have expired: no followers found")
	}

	if err := common.UpdateSourceStats(context.Background(), dbQueries, sourceId, func(s *common.ProfileStats) {
		s.FollowersCount = followersCount
	}); err != nil {
		log.Printf("TikTok: Failed to update stats for source %s: %v", sourceId, err)
	}

	return nil
}

func parseDate(s string) (time.Time, error) {
	layout := "02 Jan 2006, 3:04 pm"
	s = strings.ReplaceAll(s, "\u202f", " ")
	s = strings.TrimSpace(s)
	return time.Parse(layout, s)
}

func parseCount(s string) int {
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	multi := 1.0
	if strings.HasSuffix(s, "K") {
		multi = 1000.0
		s = strings.TrimSuffix(s, "K")
	} else if strings.HasSuffix(s, "M") {
		multi = 1000000.0
		s = strings.TrimSuffix(s, "M")
	}

	val, _ := strconv.ParseFloat(s, 64)
	return int(val * multi)
}

func extractTimestampFromID(idStr string) time.Time {
	idInt, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return time.Now()
	}
	timestamp := idInt >> 32
	if timestamp <= 0 {
		return time.Now()
	}
	return time.Unix(timestamp, 0)
}

// SPDX-License-Identifier: AGPL-3.0-only
package sources

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"
)

type twitterCookieEntry struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
	SameSite *string `json:"sameSite"`
}

type twitterTweetsResp struct {
	Data struct {
		User struct {
			Result struct {
				Timeline struct {
					Timeline struct {
						Instructions []twitterInstruction `json:"instructions"`
					} `json:"timeline"`
				} `json:"timeline"`
			} `json:"result"`
		} `json:"user"`
	} `json:"data"`
}

type twitterInstruction struct {
	Type    string         `json:"type"`
	Entries []twitterEntry `json:"entries"`
	Entry   *twitterEntry  `json:"entry"`
}

type twitterEntry struct {
	EntryID string `json:"entryId"`
	Content struct {
		ItemContent *struct {
			TweetResults struct {
				Result twitterRawResult `json:"result"`
			} `json:"tweet_results"`
		} `json:"itemContent"`
	} `json:"content"`
}

type twitterRawResult struct {
	TypeName string              `json:"__typename"`
	RestID   string              `json:"rest_id"`
	Tweet    *twitterTweetData   `json:"tweet"`
	Core     *twitterTweetCore   `json:"core"`
	Legacy   *twitterTweetLegacy `json:"legacy"`
	Views    *twitterTweetViews  `json:"views"`
}

type twitterTweetData struct {
	RestID string             `json:"rest_id"`
	Core   twitterTweetCore   `json:"core"`
	Legacy twitterTweetLegacy `json:"legacy"`
	Views  twitterTweetViews  `json:"views"`
}

type twitterTweetCore struct {
	UserResults struct {
		Result struct {
			Core struct {
				ScreenName string `json:"screen_name"`
			} `json:"core"`
			RelationshipCounts struct {
				Followers int `json:"followers"`
				Following int `json:"following"`
			} `json:"relationship_counts"`
		} `json:"result"`
	} `json:"user_results"`
}

type twitterTweetLegacy struct {
	CreatedAt             string `json:"created_at"`
	FullText              string `json:"full_text"`
	FavoriteCount         int    `json:"favorite_count"`
	RetweetCount          int    `json:"retweet_count"`
	QuoteCount            int    `json:"quote_count"`
	RetweetedStatusIDStr  string `json:"retweeted_status_id_str"`
	IsQuoteStatus         bool   `json:"is_quote_status"`
	RetweetedStatusResult *struct {
		Result twitterRawResult `json:"result"`
	} `json:"retweeted_status_result"`
}

type twitterTweetViews struct {
	Count string `json:"count"`
	State string `json:"state"`
}

func resolveRetweetedTweet(tweet *twitterTweetData) *twitterTweetData {
	if tweet.Legacy.RetweetedStatusResult == nil {
		return nil
	}
	raw := tweet.Legacy.RetweetedStatusResult.Result
	switch raw.TypeName {
	case "Tweet":
		if raw.Core != nil && raw.Legacy != nil {
			orig := &twitterTweetData{
				RestID: raw.RestID,
				Core:   *raw.Core,
				Legacy: *raw.Legacy,
			}
			if raw.Views != nil {
				orig.Views = *raw.Views
			}
			return orig
		}
	case "TweetWithVisibilityResults":
		return raw.Tweet
	}
	return nil
}

func parseRTContent(text string) (author, content string) {
	if !strings.HasPrefix(text, "RT @") {
		return "", ""
	}
	rest := text[4:]
	idx := strings.Index(rest, ": ")
	if idx < 0 {
		return "", ""
	}
	return rest[:idx], rest[idx+2:]
}

func FetchTwitterPosts(dbQueries *database.Queries, c *common.Client, username string, sourceID uuid.UUID, encryptionKey []byte) error {
	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceID)
	if err != nil {
		return err
	}

	cookieJSON, _, _, _, err := authhelp.GetSourceToken(context.Background(), dbQueries, encryptionKey, sourceID)
	if err != nil {
		return fmt.Errorf("failed to load twitter credentials: %w", err)
	}

	var cookies []twitterCookieEntry
	if err := json.Unmarshal([]byte(cookieJSON), &cookies); err != nil {
		return fmt.Errorf("failed to parse twitter cookie JSON: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(common.UserAgentChrome),
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

	bodyChan := make(chan []byte, 100)
	var trackedRequests sync.Map

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if strings.Contains(e.Response.URL, "UserTweets") ||
				strings.Contains(e.Response.URL, "UserOriginalsTimeline") ||
				strings.Contains(e.Response.URL, "UserRepostsTimeline") {
				trackedRequests.Store(e.RequestID, struct{}{})
			}
		case *network.EventLoadingFinished:
			if _, ok := trackedRequests.LoadAndDelete(e.RequestID); ok {
				reqID := e.RequestID
				go func() {
					ch := chromedp.FromContext(ctx)
					if ch == nil || ch.Target == nil {
						return
					}
					body, err := network.GetResponseBody(reqID).Do(cdp.WithExecutor(ctx, ch.Target))
					if err != nil {
						log.Printf("Twitter: failed to read response body: %v", err)
						return
					}
					select {
					case bodyChan <- body:
					default:
					}
				}()
			}
		}
	})

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
					switch *cookie.SameSite {
					case "no_restriction":
						params = params.WithSameSite(network.CookieSameSiteNone)
					case "lax":
						params = params.WithSameSite(network.CookieSameSiteLax)
					case "strict":
						params = params.WithSameSite(network.CookieSameSiteStrict)
					}
				}

				if err := params.Do(ctx); err != nil {
					return err
				}
			}
			return nil
		}),
	); err != nil {
		return fmt.Errorf("twitter: failed to initialise browser: %w", err)
	}

	profileURL := fmt.Sprintf("https://twitter.com/%s", username)
	if err := chromedp.Run(ctx,
		chromedp.Navigate(profileURL),
		chromedp.Sleep(5*time.Second),
	); err != nil {
		return fmt.Errorf("twitter: failed to navigate to profile: %w", err)
	}

	var currentURL string
	if err := chromedp.Run(ctx, chromedp.Location(&currentURL)); err == nil {
		if strings.Contains(currentURL, "login") || strings.Contains(currentURL, "i/flow/login") {
			return errors.New("twitter: session expired, please re-authenticate with fresh cookies")
		}
	}

	drainChan := func(allEntries *[]twitterEntry) {
		for {
			select {
			case body := <-bodyChan:
				var resp twitterTweetsResp
				if err := json.Unmarshal(body, &resp); err != nil {
					log.Printf("Twitter: failed to parse response: %v", err)
					continue
				}
				for _, inst := range resp.Data.User.Result.Timeline.Timeline.Instructions {
					switch inst.Type {
					case "TimelineAddEntries":
						*allEntries = append(*allEntries, inst.Entries...)
					case "TimelinePinEntry":
						if inst.Entry != nil {
							*allEntries = append(*allEntries, *inst.Entry)
						}
					}
				}
			default:
				return
			}
		}
	}

	scrollAndCollect := func(allEntries *[]twitterEntry) {
		previousCount := len(*allEntries)
		sameCountIterations := 0
		const maxScrolls = 50

		for i := 0; i < maxScrolls; i++ {
			time.Sleep(common.ScraperRateLimit)
			drainChan(allEntries)

			currentCount := len(*allEntries)
			if currentCount == previousCount {
				sameCountIterations++
				if sameCountIterations >= 4 {
					break
				}
			} else {
				sameCountIterations = 0
			}
			previousCount = currentCount

			chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil)) //nolint:errcheck
		}

		time.Sleep(common.ScraperRateLimit)
		drainChan(allEntries)
	}

	var allEntries []twitterEntry
	scrollAndCollect(&allEntries)

	repostsURL := profileURL + "/reposts"
	if err := chromedp.Run(ctx,
		chromedp.Navigate(repostsURL),
		chromedp.Sleep(common.ScraperRateLimit),
	); err != nil {
		log.Printf("Twitter: failed to navigate to reposts tab for %s: %v", username, err)
	} else {
		scrollAndCollect(&allEntries)
	}

	processedLinks := make(map[string]struct{})
	var followersCount, followingCount *int

	for _, entry := range allEntries {
		if !strings.HasPrefix(entry.EntryID, "tweet-") || entry.Content.ItemContent == nil {
			continue
		}

		raw := entry.Content.ItemContent.TweetResults.Result

		var tweet *twitterTweetData
		switch raw.TypeName {
		case "Tweet":
			if raw.Core != nil && raw.Legacy != nil {
				tweet = &twitterTweetData{
					RestID: raw.RestID,
					Core:   *raw.Core,
					Legacy: *raw.Legacy,
				}
				if raw.Views != nil {
					tweet.Views = *raw.Views
				}
			}
		case "TweetWithVisibilityResults":
			tweet = raw.Tweet
		default:
			continue
		}

		if tweet == nil || tweet.RestID == "" {
			continue
		}

		tweetID := tweet.RestID

		if _, exists := processedLinks[tweetID]; exists {
			continue
		}
		processedLinks[tweetID] = struct{}{}

		if exclusionMap[tweetID] {
			continue
		}

		postType := "post"
		src := tweet
		hasOriginalData := false
		isRetweet := tweet.Legacy.RetweetedStatusIDStr != "" ||
			tweet.Legacy.RetweetedStatusResult != nil ||
			strings.HasPrefix(tweet.Legacy.FullText, "RT @")
		if isRetweet {
			postType = "repost"
			if orig := resolveRetweetedTweet(tweet); orig != nil {
				tweetID = orig.RestID
				src = orig
				hasOriginalData = true
			} else if tweet.Legacy.RetweetedStatusIDStr != "" {
				tweetID = tweet.Legacy.RetweetedStatusIDStr
			}
			if tweetID != tweet.RestID {
				if _, exists := processedLinks[tweetID]; exists {
					continue
				}
				if exclusionMap[tweetID] {
					continue
				}
			}
		} else if tweet.Legacy.IsQuoteStatus {
			postType = "quote"
		}
		processedLinks[tweetID] = struct{}{}

		createdAt, err := time.Parse("Mon Jan 02 15:04:05 +0000 2006", src.Legacy.CreatedAt)
		if err != nil {
			if id, idErr := strconv.ParseInt(tweetID, 10, 64); idErr == nil {
				const twitterEpoch = int64(1288834974657)
				createdAt = time.UnixMilli((id >> 22) + twitterEpoch).UTC()
			} else {
				createdAt = time.Now()
			}
		}

		var viewsVal sql.NullInt64
		if src.Views.Count != "" {
			if v, err := strconv.ParseInt(src.Views.Count, 10, 64); err == nil {
				viewsVal = sql.NullInt64{Int64: v, Valid: true}
			}
		}

		if followersCount == nil {
			fc := tweet.Core.UserResults.Result.RelationshipCounts.Followers
			fng := tweet.Core.UserResults.Result.RelationshipCounts.Following
			followersCount = &fc
			followingCount = &fng
		}

		author := src.Core.UserResults.Result.Core.ScreenName
		fullText := src.Legacy.FullText

		if postType == "repost" {
			if !hasOriginalData {
				if rtAuthor, rtContent := parseRTContent(tweet.Legacy.FullText); rtAuthor != "" {
					author = rtAuthor
					fullText = rtContent
				}
			} else if author == "" {
				if rtAuthor, _ := parseRTContent(tweet.Legacy.FullText); rtAuthor != "" {
					author = rtAuthor
				}
			}
		}

		if author == "" {
			author = username
		}

		if err := common.ProcessScrapedPost(
			context.Background(),
			dbQueries,
			sourceID,
			tweetID,
			"Twitter",
			createdAt,
			postType,
			author,
			fullText,
			sql.NullInt64{Int64: int64(src.Legacy.FavoriteCount), Valid: true},
			sql.NullInt64{Int64: int64(src.Legacy.RetweetCount + src.Legacy.QuoteCount), Valid: true},
			viewsVal,
		); err != nil {
			log.Printf("Twitter: failed to process tweet %s: %v", tweetID, err)
		}
	}

	if len(processedLinks) == 0 {
		return errors.New("no tweets found: session may have expired")
	}

	if err := common.UpdateSourceStats(context.Background(), dbQueries, sourceID, func(s *common.ProfileStats) {
		s.FollowersCount = followersCount
		s.FollowingCount = followingCount
	}); err != nil {
		log.Printf("Twitter: failed to update stats for source %s: %v", sourceID, err)
	}

	return nil
}

// SPDX-License-Identifier: AGPL-3.0-only
package sources

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"
)

type redditCookieEntry struct {
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	Domain         string   `json:"domain"`
	Path           string   `json:"path"`
	Secure         bool     `json:"secure"`
	HTTPOnly       bool     `json:"httpOnly"`
	SameSite       *string  `json:"sameSite"`
	ExpirationDate *float64 `json:"expirationDate"`
}

type redditListing struct {
	Data struct {
		After    string `json:"after"`
		Children []struct {
			Data struct {
				ID            string  `json:"id"`
				Subreddit     string  `json:"subreddit"`
				Title         string  `json:"title"`
				Selftext      string  `json:"selftext"`
				Score         int     `json:"score"`
				CreatedUTC    float64 `json:"created_utc"`
				Author        string  `json:"author"`
				IsVideo       bool    `json:"is_video"`
				NumCrossposts int     `json:"num_crossposts"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type redditAboutData struct {
	Data struct {
		Subreddit struct {
			Subscribers int `json:"subscribers"`
		} `json:"subreddit"`
		TotalKarma int `json:"total_karma"`
	} `json:"data"`
}

func getRedditDetails(ctx context.Context, dbQueries *database.Queries, encryptionKey []byte, sid uuid.UUID) (username string, subreddits []string, cookieJSON string, err error) {
	userSource, err := dbQueries.GetSourceById(ctx, sid)
	if err != nil {
		return "", nil, "", err
	}
	username = userSource.UserName

	token, profileID, _, _, tokenErr := authhelp.GetSourceToken(ctx, dbQueries, encryptionKey, sid)
	if tokenErr == nil {
		cookieJSON = token
		if profileID != "" {
			for _, s := range strings.Split(profileID, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					subreddits = append(subreddits, strings.ToLower(s))
				}
			}
		}
	}

	return username, subreddits, cookieJSON, nil
}

func handleSubredditChanges(ctx context.Context, dbQueries *database.Queries, sourceID uuid.UUID, newSubreddits []string) {
	if len(newSubreddits) == 0 {
		return
	}

	newSet := make(map[string]bool)
	for _, s := range newSubreddits {
		newSet[strings.ToLower(s)] = true
	}

	posts, err := dbQueries.GetNetworkIdsAndContentBySource(ctx, sourceID)
	if err != nil {
		log.Printf("Reddit: Failed to get existing posts for subreddit pruning: %v", err)
		return
	}

	removedSubreddits := make(map[string]bool)
	for _, post := range posts {
		if !post.Content.Valid {
			continue
		}
		if !strings.HasPrefix(post.Content.String, "r/") {
			continue
		}
		colon := strings.Index(post.Content.String, ":")
		if colon < 3 {
			continue
		}
		subreddit := strings.ToLower(post.Content.String[2:colon])
		if !newSet[subreddit] {
			removedSubreddits[subreddit] = true
		}
	}

	for subreddit := range removedSubreddits {
		pattern := "r/" + subreddit + ":%"
		err := dbQueries.DeletePostsByContentPrefix(ctx, database.DeletePostsByContentPrefixParams{
			SourceID: sourceID,
			Content:  sql.NullString{String: pattern, Valid: true},
		})
		if err != nil {
			log.Printf("Reddit: Failed to delete posts for removed subreddit %s: %v", subreddit, err)
		} else {
			log.Printf("Reddit: Deleted posts for removed subreddit %s", subreddit)
		}
	}
}

func redditXHR(chromeCtx context.Context, url string) (string, error) {
	var body string
	err := chromedp.Run(chromeCtx,
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				var xhr = new XMLHttpRequest();
				xhr.open('GET', %q, false);
				xhr.setRequestHeader('Accept', 'application/json');
				xhr.send(null);
				return xhr.responseText;
			})()
		`, url), &body),
	)
	return body, err
}

func FetchRedditPosts(dbQueries *database.Queries, encryptionKey []byte, sourceId uuid.UUID, _ *common.Client) error {
	ctx := context.Background()

	username, subreddits, cookieJSON, err := getRedditDetails(ctx, dbQueries, encryptionKey, sourceId)
	if err != nil {
		return err
	}

	if cookieJSON == "" || cookieJSON == "public" {
		return errors.New("Reddit cookies not configured — use the key icon to paste your Cookie-Editor JSON export")
	}

	var cookies []redditCookieEntry
	if err := json.Unmarshal([]byte(cookieJSON), &cookies); err != nil {
		return fmt.Errorf("failed to parse Reddit cookie JSON: %w", err)
	}

	handleSubredditChanges(ctx, dbQueries, sourceId, subreddits)

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

	chromeCtx, ctxCancel := chromedp.NewContext(allocCtx)
	defer ctxCancel()

	if err := chromedp.Run(chromeCtx,
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
		chromedp.Navigate("https://www.reddit.com"),
		chromedp.Sleep(common.ScraperRateLimit),
	); err != nil {
		return fmt.Errorf("failed to initialize Reddit session: %w", err)
	}

	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	subredditFilter := make(map[string]bool)
	for _, s := range subreddits {
		subredditFilter[s] = true
	}

	processedPosts := make(map[string]struct{})
	var after string
	const maxPages = 500

	for page := 0; page < maxPages; page++ {
		time.Sleep(common.ScraperRateLimit)

		apiURL := fmt.Sprintf("https://www.reddit.com/user/%s/submitted.json?limit=100&raw_json=1", username)
		if after != "" {
			apiURL += "&after=" + after
		}

		bodyText, err := redditXHR(chromeCtx, apiURL)
		if err != nil {
			log.Printf("Reddit: XHR failed on page %d: %v", page, err)
			break
		}
		if bodyText == "" {
			log.Printf("Reddit: empty response on page %d", page)
			break
		}

		var listing redditListing
		if err := json.Unmarshal([]byte(bodyText), &listing); err != nil {
			return fmt.Errorf("failed to decode Reddit listing: %w", err)
		}

		if len(listing.Data.Children) == 0 {
			break
		}

		for _, child := range listing.Data.Children {
			post := child.Data
			postID := post.ID

			if _, exists := processedPosts[postID]; exists {
				continue
			}
			processedPosts[postID] = struct{}{}

			if exclusionMap[postID] {
				continue
			}

			if len(subredditFilter) > 0 && !subredditFilter[strings.ToLower(post.Subreddit)] {
				continue
			}

			postedAt := time.Unix(int64(post.CreatedUTC), 0)

			postType := "post"
			if post.IsVideo {
				postType = "video"
			}

			content := fmt.Sprintf("r/%s: %s", post.Subreddit, post.Title)
			if post.Selftext != "" && post.Selftext != "[deleted]" && post.Selftext != "[removed]" {
				content = fmt.Sprintf("r/%s: %s\n\n%s", post.Subreddit, post.Title, post.Selftext)
			}

			internalID, err := common.CreateOrUpdatePost(
				ctx,
				dbQueries,
				sourceId,
				postID,
				"Reddit",
				postedAt,
				postType,
				post.Author,
				content,
			)
			if err != nil {
				log.Printf("Reddit: Failed to save post %s: %v", postID, err)
				continue
			}

			_, err = dbQueries.SyncReactions(ctx, database.SyncReactionsParams{
				ID:       uuid.New(),
				SyncedAt: time.Now(),
				PostID:   internalID,
				Likes:    sql.NullInt64{Int64: int64(post.Score), Valid: true},
				Reposts:  sql.NullInt64{Int64: int64(post.NumCrossposts), Valid: true},
				Views:    sql.NullInt64{Valid: false},
			})
			if err != nil {
				log.Printf("Reddit: Failed to sync reactions for post %s: %v", postID, err)
			}
		}

		if listing.Data.After == "" {
			break
		}

		after = listing.Data.After
	}

	if len(processedPosts) == 0 {
		return errors.New("no posts found — Reddit cookies may be expired")
	}

	aboutURL := fmt.Sprintf("https://www.reddit.com/user/%s/about.json?raw_json=1", username)
	aboutText, err := redditXHR(chromeCtx, aboutURL)
	if err != nil {
		log.Printf("Reddit: failed to fetch about.json: %v", err)
	} else if aboutText != "" {
		var about redditAboutData
		if err := json.Unmarshal([]byte(aboutText), &about); err == nil {
			followers := about.Data.Subreddit.Subscribers
			if err := common.UpdateSourceStats(ctx, dbQueries, sourceId, func(s *common.ProfileStats) {
				s.FollowersCount = &followers
			}); err != nil {
				log.Printf("Reddit: failed to update follower stats: %v", err)
			}
			return nil
		}
	}

	avgStats, err := common.CalculateAverageStats(ctx, dbQueries, sourceId)
	if err != nil {
		log.Printf("Reddit: Failed to calculate average stats: %v", err)
	} else {
		if err := common.SaveOrUpdateSourceStats(ctx, dbQueries, sourceId, avgStats); err != nil {
			log.Printf("Reddit: Failed to save stats: %v", err)
		}
	}

	return nil
}

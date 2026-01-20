package fetcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

type TikTokManager struct {
	sessions map[string]*LoginSession
	mu       sync.RWMutex
}

type LoginSession struct {
	Username  string
	QRCode    []byte
	Status    string
	CreatedAt time.Time
	Error     string
}

var (
	GlobalTikTokManager *TikTokManager
	cookiesDir          = "outputs/tiktok_cookies"
)

func init() {
	GlobalTikTokManager = &TikTokManager{
		sessions: make(map[string]*LoginSession),
	}
	if err := os.MkdirAll(cookiesDir, 0755); err != nil {
		log.Printf("Failed to create cookies directory: %v", err)
	}
}

func (tm *TikTokManager) StartLoginSession(username string) ([]byte, error) {
	tm.mu.Lock()
	session := &LoginSession{
		Username:  username,
		Status:    "initiating",
		CreatedAt: time.Now(),
	}
	tm.sessions[username] = session
	tm.mu.Unlock()

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.WindowSize(1280, 800),
	)...)

	ctx, ctxCancel := chromedp.NewContext(allocCtx)

	qrCodeChan := make(chan []byte)
	errChan := make(chan error)

	go func() {
		defer allocCancel()
		defer ctxCancel()

		var screenshot []byte
		var title string

		if err := chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				var res []byte
				err := chromedp.Evaluate(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined})`, &res).Do(ctx)
				return err
			}),
			chromedp.Navigate("https://www.tiktok.com/login/qrcode"),
			chromedp.Title(&title),
		); err != nil {
			log.Printf("Failed to navigate: %v", err)
		}

		err := chromedp.Run(ctx,
			chromedp.WaitVisible(`div[data-e2e="qr-code"]`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second),
			chromedp.Screenshot(`div[data-e2e="qr-code"] canvas`, &screenshot, chromedp.NodeVisible),
		)

		if err != nil {
			err = chromedp.Run(ctx,
				chromedp.Navigate("https://www.tiktok.com/login"),
				chromedp.WaitVisible(`//a[contains(text(), "Use QR code")]`, chromedp.BySearch),
				chromedp.Click(`//a[contains(text(), "Use QR code")]`, chromedp.BySearch),
				chromedp.WaitVisible(`canvas`, chromedp.ByQuery),
				chromedp.Screenshot(`canvas`, &screenshot, chromedp.NodeVisible),
			)
		}

		if err != nil {
			tm.mu.Lock()
			session.Status = "failed"
			session.Error = fmt.Sprintf("Failed to load QR code: %v", err)
			tm.mu.Unlock()
			errChan <- err
			return
		}

		tm.mu.Lock()
		session.QRCode = screenshot
		session.Status = "waiting_scan"
		tm.mu.Unlock()

		qrCodeChan <- screenshot

		timeout := 3 * time.Minute
		deadline := time.Now().Add(timeout)

		loginSuccess := false
		var networkCookies []*network.Cookie

		for time.Now().Before(deadline) {
			if err := chromedp.Run(ctx,
				chromedp.ActionFunc(func(ctx context.Context) error {
					var err error
					networkCookies, err = network.GetCookies().Do(ctx)
					return err
				}),
			); err != nil {
				time.Sleep(2 * time.Second)
				continue
			}

			hasSession := false
			for _, c := range networkCookies {
				if c.Name == "sessionid" {
					hasSession = true
					break
				}
			}

			if hasSession {
				loginSuccess = true
				break
			}

			time.Sleep(3 * time.Second)
		}

		if !loginSuccess {
			tm.mu.Lock()
			session.Status = "failed"
			session.Error = "Login timed out"
			tm.mu.Unlock()
			return
		}

		if err := saveCookies(username, networkCookies); err != nil {
			log.Printf("Failed to save cookies: %v", err)
			return
		}

		tm.mu.Lock()
		session.Status = "success"
		tm.mu.Unlock()
	}()

	select {
	case qr := <-qrCodeChan:
		return qr, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(90 * time.Second):
		return nil, errors.New("timeout waiting for QR code")
	}
}

func (tm *TikTokManager) CheckStatus(username string) (string, string, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	session, ok := tm.sessions[username]
	if !ok {
		return "", "", errors.New("session not found")
	}
	return session.Status, session.Error, nil
}

func saveCookies(username string, cookies []*network.Cookie) error {
	data, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	path := filepath.Join(cookiesDir, fmt.Sprintf("tiktok_%s.json", username))
	return os.WriteFile(path, data, 0644)
}

func loadCookies(username string) ([]*network.Cookie, error) {
	path := filepath.Join(cookiesDir, fmt.Sprintf("tiktok_%s.json", username))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cookies []*network.Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return nil, err
	}
	return cookies, nil
}

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

func FetchTikTokPosts(dbQueries *database.Queries, c *Client, uid uuid.UUID, sourceId uuid.UUID) error {
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
	log.Printf("Navigating to TikTok Studio: %s", url)

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
						log.Printf("No new posts for 5 iterations, stopping scroll.")
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

	log.Printf("Successfully scraped %d posts", len(scrapedPosts))

	if len(scrapedPosts) == 0 {
		var debugHtml string
		var debugScreenshot []byte
		chromedp.Run(ctx,
			chromedp.OuterHTML("html", &debugHtml),
			chromedp.CaptureScreenshot(&debugScreenshot),
		)
		_ = os.WriteFile(filepath.Join("outputs", "debug_tiktok_studio.html"), []byte(debugHtml), 0644)
		_ = os.WriteFile(filepath.Join("outputs", "debug_tiktok_studio.png"), debugScreenshot, 0644)
	}

	for _, item := range scrapedPosts {
		if item.ID == "" {
			continue
		}

		createdAt, err := parseDate(item.DateText)
		if err != nil {
			createdAt = extractTimestampFromID(item.ID)
		}

		content := item.Desc
		postType := item.Type

		postId := uuid.New()
		existing, err := dbQueries.GetPostByNetworkAndId(context.Background(), database.GetPostByNetworkAndIdParams{
			NetworkInternalID: item.ID,
			Network:           "TikTok",
		})

		if err != nil {
			newPost, errC := dbQueries.CreatePost(context.Background(), database.CreatePostParams{
				ID:                postId,
				CreatedAt:         createdAt,
				LastSyncedAt:      time.Now(),
				SourceID:          sourceId,
				IsArchived:        false,
				Author:            username,
				PostType:          postType,
				NetworkInternalID: item.ID,
				Content:           sql.NullString{String: content, Valid: content != ""},
			})
			if errC != nil {
				log.Printf("Failed to create post %s: %v", item.ID, errC)
				continue
			}
			postId = newPost.ID
		} else {
			postId = existing.ID
		}

		viewsCount := parseCount(item.Views)
		likesCount := parseCount(item.Likes)

		_, err = dbQueries.SyncReactions(context.Background(), database.SyncReactionsParams{
			ID:       uuid.New(),
			SyncedAt: time.Now(),
			PostID:   postId,
			Views:    sql.NullInt32{Int32: int32(viewsCount), Valid: viewsCount > 0},
			Likes:    sql.NullInt32{Int32: int32(likesCount), Valid: likesCount >= 0},
			Reposts:  sql.NullInt32{Int32: int32(0), Valid: false},
		})
		if err != nil {
			log.Printf("Failed to sync stats for %s: %v", item.ID, err)
		}
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

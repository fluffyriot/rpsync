// SPDX-License-Identifier: AGPL-3.0-only
package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
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
	if err := os.MkdirAll(cookiesDir, 0700); err != nil {
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
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
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
	safeUsername, err := sanitizeUsername(username)
	if err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	data, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	path := filepath.Join(cookiesDir, fmt.Sprintf("tiktok_%s.json", safeUsername))
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func loadCookies(username string) ([]*network.Cookie, error) {
	safeUsername, err := sanitizeUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid username: %w", err)
	}

	path := filepath.Join(cookiesDir, fmt.Sprintf("tiktok_%s.json", safeUsername))
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

func sanitizeUsername(username string) (string, error) {
	if username == "" {
		return "", fmt.Errorf("username cannot be empty")
	}
	if strings.ContainsAny(username, `/\`) {
		return "", fmt.Errorf("username contains invalid characters")
	}
	if strings.Contains(username, "..") {
		return "", fmt.Errorf("username contains directory traversal sequence")
	}
	if username == "." || username == ".." {
		return "", fmt.Errorf("username cannot be . or ..")
	}
	for _, r := range username {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-') {
			return "", fmt.Errorf("username contains invalid characters")
		}
	}
	return username, nil
}

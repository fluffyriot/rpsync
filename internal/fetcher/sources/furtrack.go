package sources

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"
)

type FurTrackUserResponse struct {
	Success bool `json:"success"`
	User    struct {
		UserID      int    `json:"userId"`
		Username    string `json:"username"`
		Submissions int    `json:"submissions"`
		UserMeta    struct {
			Location string `json:"location"`
		} `json:"userMeta"`
	} `json:"user"`
}

type FurTrackPostsResponse struct {
	Success   bool            `json:"success"`
	SubAlbums []FurTrackAlbum `json:"subAlbums"`
	Posts     []FurTrackPost  `json:"posts"`
}

type FurTrackAlbum struct {
	AlbumID    int    `json:"albumId"`
	AlbumTitle string `json:"albumTitle"`
	Tags       string `json:"tags"`
}

type FurTrackPost struct {
	PostID        int    `json:"postId"`
	TimestampPost string `json:"timestampPost"`
	CC            int    `json:"cc"`
	CV            int    `json:"cv"`
	CL            int    `json:"cl"`
}

func FetchFurTrackPosts(dbQueries *database.Queries, c *common.Client, uid uuid.UUID, sourceId uuid.UUID) error {

	source, err := dbQueries.GetSourceById(context.Background(), sourceId)
	if err != nil {
		return fmt.Errorf("failed to get source: %w", err)
	}

	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	username := source.UserName
	log.Printf("FurTrack: Starting sync for user %s", username)

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

	url := fmt.Sprintf("https://www.furtrack.com/user/%s/photography", username)

	mainPostsChan := make(chan string, 1)

	var mainPageJSON string
	err = chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			chromedp.ListenTarget(ctx, func(ev interface{}) {
				if evt, ok := ev.(*network.EventResponseReceived); ok {
					if strings.Contains(evt.Response.URL, "solar.furtrack.com/get/a/") {
						go func(reqID network.RequestID) {
							tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
							defer cancel()
							body, err := network.GetResponseBody(reqID).Do(tCtx)
							if err == nil {
								select {
								case mainPostsChan <- string(body):
								default:
								}
							}
						}(evt.RequestID)
					}
				}
			})
			return nil
		}),
		chromedp.Navigate(url),
	)
	if err != nil {
		return fmt.Errorf("FurTrack: Browser navigation failed: %w", err)
	}

	select {
	case mainPageJSON = <-mainPostsChan:
	case <-time.After(20 * time.Second):
		return fmt.Errorf("FurTrack: Timed out waiting for album list")
	}

	var postsResp FurTrackPostsResponse
	if err := json.Unmarshal([]byte(mainPageJSON), &postsResp); err != nil {
		return fmt.Errorf("FurTrack: Failed to unmarshal entries: %w", err)
	}

	log.Printf("FurTrack: Found %d albums to sync", len(postsResp.SubAlbums))

	processedAlbums := make(map[string]struct{})

	for _, album := range postsResp.SubAlbums {
		networkID := fmt.Sprintf("%d", album.AlbumID)

		if _, exists := processedAlbums[networkID]; exists {
			continue
		}
		processedAlbums[networkID] = struct{}{}

		if exclusionMap[networkID] {
			continue
		}

		albumURL := fmt.Sprintf("https://www.furtrack.com/user/%s/album-%d", username, album.AlbumID)
		log.Printf("FurTrack: Syncing Album %s (%d)", album.AlbumTitle, album.AlbumID)

		var albumDataJSON string
		albumChan := make(chan string, 1)

		err = chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				chromedp.ListenTarget(ctx, func(ev interface{}) {
					if evt, ok := ev.(*network.EventResponseReceived); ok {
						if strings.Contains(evt.Response.URL, "solar.furtrack.com/get/a/") {
							go func(reqID network.RequestID) {
								tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
								defer cancel()
								body, err := network.GetResponseBody(reqID).Do(tCtx)
								if err == nil {
									select {
									case albumChan <- string(body):
									default:
									}
								}
							}(evt.RequestID)
						}
					}
				})
				return nil
			}),
			chromedp.Navigate(albumURL),
		)
		if err != nil {
			log.Printf("FurTrack: Failed to navigate to album %d: %v", album.AlbumID, err)
			continue
		}

		select {
		case albumDataJSON = <-albumChan:
		case <-time.After(15 * time.Second):
			log.Printf("FurTrack: Timeout waiting for data for album %d", album.AlbumID)
			continue
		}

		var albumPosts FurTrackPostsResponse
		if err := json.Unmarshal([]byte(albumDataJSON), &albumPosts); err != nil {
			log.Printf("FurTrack: Failed to parse album data: %v", err)
			continue
		}

		totalLikes := 0
		for _, post := range albumPosts.Posts {
			totalLikes += (post.CV - 1 + post.CL)
		}

		postID, err := common.CreateOrUpdatePost(
			context.Background(),
			dbQueries,
			sourceId,
			networkID,
			"FurTrack",
			getAlbumDate(albumPosts.Posts),
			"album",
			username,
			album.AlbumTitle+" | "+album.Tags,
		)
		if err != nil {
			log.Printf("FurTrack: Failed to create/update post for album %s: %v", networkID, err)
			continue
		}

		_, err = dbQueries.SyncReactions(context.Background(), database.SyncReactionsParams{
			ID:       uuid.New(),
			SyncedAt: time.Now(),
			PostID:   postID,
			Likes: sql.NullInt64{
				Int64: int64(totalLikes),
				Valid: true,
			},
			Views: sql.NullInt64{
				Valid: false,
			},
			Reposts: sql.NullInt64{
				Valid: false,
			},
		})
		if err != nil {
			log.Printf("FurTrack: Failed to sync reactions for album %s: %v", networkID, err)
		}
	}

	if len(processedAlbums) == 0 {
		return fmt.Errorf("FurTrack: No albums found")
	}

	stats, err := common.CalculateAverageStats(context.Background(), dbQueries, sourceId)
	if err != nil {
		log.Printf("FurTrack: Failed to calculate stats for source %s: %v", sourceId, err)
	} else {
		if err := common.SaveOrUpdateSourceStats(context.Background(), dbQueries, sourceId, stats); err != nil {
			log.Printf("FurTrack: Failed to save stats for source %s: %v", sourceId, err)
		}
	}

	log.Println("FurTrack: Sync complete.")
	return nil
}

func getAlbumDate(posts []FurTrackPost) time.Time {
	if len(posts) > 0 {
		t, err := time.Parse(time.RFC3339, posts[0].TimestampPost)
		if err == nil {
			return t
		}
	}
	return time.Now()
}

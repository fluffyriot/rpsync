package fetcher

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/google/uuid"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

func getTgDetails(ctx context.Context, dbQueries *database.Queries, encryptionKey []byte, sid uuid.UUID) (string, string, int, string, error) {
	botToken, channelUsername, _, _, err := authhelp.GetSourceToken(ctx, dbQueries, encryptionKey, sid)
	if err != nil {
		return "", "", 0, "", err
	}

	parts := strings.Split(botToken, ":::")
	if len(parts) != 3 {
		return "", "", 0, "", fmt.Errorf("invalid bot token format")
	}

	appID, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", "", 0, "", err
	}

	return parts[0], channelUsername, appID, parts[2], nil
}

func botAuth(ctx context.Context, client *telegram.Client, botToken string, maxRetries int) error {

	status, err := client.Auth().Status(ctx)
	if err == nil && status.Authorized {
		return nil
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := client.Auth().Bot(ctx, botToken)
		if err == nil {
			return nil
		}

		if wait, isFlood := telegram.AsFloodWait(err); isFlood {
			if wait > 60*time.Second {
				return fmt.Errorf("flood wait too long: %v", wait)
			}
			time.Sleep(wait)
			continue
		}

		backoff := time.Duration(attempt*attempt) * time.Second
		time.Sleep(backoff)
	}

	return fmt.Errorf("bot auth failed after %d retries", maxRetries)
}

func FetchTelegramPosts(dbQueries *database.Queries, encryptionKey []byte, sid uuid.UUID, c *Client) error {
	ctx := context.Background()

	botToken, channelUsername, appID, appHash, err := getTgDetails(ctx, dbQueries, encryptionKey, sid)
	if err != nil {
		return err
	}

	sessionFile := fmt.Sprintf("outputs/telegram_session_%s.json", sid.String())
	sess := &session.FileStorage{Path: sessionFile}

	client := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: sess,
	})

	return client.Run(ctx, func(ctx context.Context) error {

		exclusionMap, err := loadExclusionMap(dbQueries, sid)
		if err != nil {
			return err
		}

		if err := botAuth(ctx, client, botToken, 5); err != nil {
			return err
		}

		res, err := client.API().ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
			Username: channelUsername,
		})
		if err != nil {
			return fmt.Errorf("failed to resolve channel: %w", err)
		}

		var channel *tg.Channel
		for _, c := range res.Chats {
			if ch, ok := c.(*tg.Channel); ok {
				channel = ch
				break
			}
		}
		if channel == nil {
			return fmt.Errorf("resolved chat is not a channel")
		}

		input := &tg.InputChannel{
			ChannelID:  channel.ID,
			AccessHash: channel.AccessHash,
		}

		var participantCount *int
		fullChan, err := client.API().ChannelsGetFullChannel(ctx, input)
		if err != nil {
			log.Printf("Telegram: Failed to get full channel info: %v", err)
		} else {
			if channelFull, ok := fullChan.FullChat.(*tg.ChannelFull); ok {
				if channelFull.ParticipantsCount != 0 {
					count := int(channelFull.ParticipantsCount)
					participantCount = &count
				}
			}
		}

		var (
			startID          = 1
			batchSize        = 100
			emptyHits        = 0
			maxEmptyAttempts = 5
		)

		processedLinks := make(map[int]struct{})

		for emptyHits < maxEmptyAttempts {
			ids := make([]tg.InputMessageClass, 0, batchSize)
			for i := 0; i < batchSize; i++ {
				ids = append(ids, &tg.InputMessageID{ID: startID + i})
			}

			var resp any
			retries := 0
			for {
				resp, err = client.API().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
					Channel: input,
					ID:      ids,
				})
				if err == nil {
					break
				}
				if retries >= 5 {
					return fmt.Errorf("channels.getMessages failed after retries: %w", err)
				}
				retries++
				backoff := time.Duration(retries*retries) * time.Second
				time.Sleep(backoff)
			}

			var messages []tg.MessageClass
			switch v := resp.(type) {
			case *tg.MessagesMessages:
				messages = v.Messages
			case *tg.MessagesChannelMessages:
				messages = v.Messages
			default:
				startID += batchSize
				continue
			}

			realMessages := 0

			for _, m := range messages {
				if msg, ok := m.(*tg.Message); ok {

					msgIDStr := fmt.Sprintf("%d", msg.ID)

					if _, exists := processedLinks[msg.ID]; exists {
						continue
					}
					processedLinks[msg.ID] = struct{}{}

					if exclusionMap[msgIDStr] {
						continue
					}

					likes, _ := FetchTelegramWebStats(channelUsername, msg.ID, c)

					msgTime := time.Unix(int64(msg.Date), 0).UTC()

					postID, err := createOrUpdatePost(
						ctx,
						dbQueries,
						sid,
						fmt.Sprintf("%d", msg.ID),
						"Telegram",
						msgTime,
						"post",
						channelUsername,
						msg.Message,
					)
					if err != nil {
						continue
					}

					_, err = dbQueries.SyncReactions(ctx, database.SyncReactionsParams{
						ID:       uuid.New(),
						SyncedAt: time.Now(),
						PostID:   postID,
						Likes: sql.NullInt32{
							Int32: int32(likes),
							Valid: true,
						},
						Reposts: sql.NullInt32{
							Int32: int32(msg.Forwards),
							Valid: true,
						},
						Views: sql.NullInt32{
							Int32: int32(msg.Views),
							Valid: true,
						},
					})
					if err != nil {
						log.Printf("[WARN] Failed to sync reactions for post ID=%d: %v", msg.ID, err)
					}
				}
			}

			if realMessages == 0 {
				emptyHits++
			} else {
				emptyHits = 0
			}

			startID += batchSize
		}

		if len(processedLinks) == 0 {
			return fmt.Errorf("no new messages found")
		}

		stats, err := calculateAverageStats(ctx, dbQueries, sid)
		if err != nil {
			log.Printf("Telegram: Failed to calculate stats for source %s: %v", sid, err)
		} else {
			stats.FollowersCount = participantCount

			if err := saveOrUpdateSourceStats(ctx, dbQueries, sid, stats); err != nil {
				log.Printf("Telegram: Failed to save stats for source %s: %v", sid, err)
			}
		}

		return nil
	})
}

func FetchTelegramWebStats(channel string, messageID int, c *Client) (int, error) {
	url := fmt.Sprintf("https://t.me/%s/%d?embed=1&mode=tme", channel, messageID)

	likes := 0

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return likes, err
	}

	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return likes, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return likes, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return likes, err
	}

	html := string(bodyBytes)

	reactionCountRe := regexp.MustCompile(`<span[^>]*class="tgme_reaction"[^>]*>.*?(\d+)\s*</span>`)
	reactionMatches := reactionCountRe.FindAllStringSubmatch(html, -1)
	for _, match := range reactionMatches {
		if len(match) == 2 {
			likes += parseInt(match[1])
		}
	}

	return likes, nil
}

func parseInt(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	n, _ := strconv.Atoi(s)
	return n
}

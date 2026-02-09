package sources

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"
)

func getDiscordDetails(ctx context.Context, dbQueries *database.Queries, encryptionKey []byte, sid uuid.UUID) (string, string, []string, error) {
	botToken, channelConfig, _, _, err := authhelp.GetSourceToken(ctx, dbQueries, encryptionKey, sid)
	if err != nil {
		return "", "", nil, err
	}

	parts := strings.Split(channelConfig, ":::")
	if len(parts) != 2 {
		return "", "", nil, fmt.Errorf("invalid channel config format")
	}

	serverID := parts[0]
	channelIDs := strings.Split(parts[1], ",")

	for i, id := range channelIDs {
		channelIDs[i] = strings.TrimSpace(id)
	}

	return botToken, serverID, channelIDs, nil
}

func handleChannelChanges(
	ctx context.Context,
	dbQueries *database.Queries,
	sourceID uuid.UUID,
	serverID string,
	newChannelIDs []string,
) error {
	newSet := make(map[string]bool)
	for _, ch := range newChannelIDs {
		newSet[ch] = true
	}

	posts, err := dbQueries.GetAllPostsWithTheLatestInfoForUser(ctx, sourceID)
	if err != nil {
		log.Printf("Discord: Failed to get existing posts: %v", err)
		return nil
	}

	existingChannels := make(map[string]bool)
	for _, post := range posts {
		parts := strings.Split(post.NetworkInternalID, "/")
		if len(parts) == 3 {
			channelID := parts[1]
			existingChannels[channelID] = true
		}
	}

	removedChannels := []string{}
	for channelID := range existingChannels {
		if !newSet[channelID] {
			removedChannels = append(removedChannels, channelID)
		}
	}

	for _, channelID := range removedChannels {
		pattern := fmt.Sprintf("%s/%s/%%", serverID, channelID)
		err := dbQueries.DeletePostsByNetworkIdPrefix(ctx, database.DeletePostsByNetworkIdPrefixParams{
			SourceID:          sourceID,
			NetworkInternalID: pattern,
		})
		if err != nil {
			log.Printf("Discord: Failed to delete posts from removed channel %s: %v", channelID, err)
		} else {

		}
	}

	return nil
}

func FetchDiscordPosts(dbQueries *database.Queries, encryptionKey []byte, sourceId uuid.UUID, c *common.Client) error {
	ctx := context.Background()

	botToken, serverID, channelIDs, err := getDiscordDetails(ctx, dbQueries, encryptionKey, sourceId)
	if err != nil {
		return err
	}

	session, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return fmt.Errorf("failed to create Discord session: %w", err)
	}
	defer session.Close()

	if err := handleChannelChanges(ctx, dbQueries, sourceId, serverID, channelIDs); err != nil {
		log.Printf("Discord: Failed to handle channel changes: %v", err)
	}

	exclusionMap, err := common.LoadExclusionMap(dbQueries, sourceId)
	if err != nil {
		return err
	}

	guild, err := session.GuildWithCounts(serverID)
	var memberCount *int
	if err != nil {
		log.Printf("Discord: Failed to get server info: %v", err)
	} else {
		count := guild.ApproximateMemberCount
		memberCount = &count
	}

	processedMessages := make(map[string]struct{})

	for _, channelID := range channelIDs {

		channel, err := session.Channel(channelID)
		if err != nil {
			log.Printf("Discord: Failed to get channel info for %s: %v", channelID, err)
			continue
		}

		if channel.Type == discordgo.ChannelTypeGuildForum {

			activeThreads, err := session.ThreadsActive(channelID)
			if err != nil {
				log.Printf("Discord: Failed to fetch active threads: %v", err)
			} else {
				for _, thread := range activeThreads.Threads {
					if err := processForumThread(ctx, dbQueries, session, sourceId, serverID, channelID, thread, exclusionMap, processedMessages); err != nil {
						log.Printf("Discord: Error processing thread %s: %v", thread.ID, err)
					}
				}
			}

			archivedThreads, err := session.ThreadsArchived(channelID, nil, 100)
			if err != nil {
				log.Printf("Discord: Failed to fetch archived threads: %v", err)
			} else {
				for _, thread := range archivedThreads.Threads {
					if err := processForumThread(ctx, dbQueries, session, sourceId, serverID, channelID, thread, exclusionMap, processedMessages); err != nil {
						log.Printf("Discord: Error processing archived thread %s: %v", thread.ID, err)
					}
				}
			}

			continue
		}

		var beforeID string
		const maxPages = 500

		for page := 0; page < maxPages; page++ {
			messages, err := session.ChannelMessages(channelID, 100, beforeID, "", "")
			if err != nil {
				log.Printf("Discord: Failed to fetch messages from channel %s: %v", channelID, err)
				break
			}

			if len(messages) == 0 {
				break
			}

			for _, msg := range messages {
				msgID := msg.ID

				if _, exists := processedMessages[msgID]; exists {
					continue
				}
				processedMessages[msgID] = struct{}{}

				if msg.Type == discordgo.MessageTypeReply || msg.Type == discordgo.MessageTypeThreadStarterMessage {
					continue
				}

				timestamp, err := discordgo.SnowflakeTimestamp(msgID)
				if err != nil {
					log.Printf("Discord: Failed to parse timestamp from message ID %s: %v", msgID, err)
					continue
				}
				networkInternalID := fmt.Sprintf("%s/%s/%s", serverID, channelID, msgID)

				if exclusionMap[networkInternalID] {
					continue
				}

				totalReactions := 0
				for _, reaction := range msg.Reactions {
					totalReactions += reaction.Count
				}

				postID, err := common.CreateOrUpdatePost(
					ctx,
					dbQueries,
					sourceId,
					networkInternalID,
					"Discord",
					timestamp,
					"post",
					msg.Author.Username,
					msg.Content,
				)
				if err != nil {
					log.Printf("Discord: Failed to create/update post %s: %v", msgID, err)
					continue
				}

				_, err = dbQueries.SyncReactions(ctx, database.SyncReactionsParams{
					ID:       uuid.New(),
					PostID:   postID,
					SyncedAt: time.Now(),
					Likes: sql.NullInt64{
						Int64: int64(totalReactions),
						Valid: true,
					},
					Reposts: sql.NullInt64{},
					Views:   sql.NullInt64{},
				})
				if err != nil {
					log.Printf("Discord: Failed to sync reactions for message %s: %v", msgID, err)
				}
			}

			beforeID = messages[len(messages)-1].ID
		}

	}

	if len(processedMessages) == 0 {
		return errors.New("No messages found in any configured channels")
	}

	stats, err := common.CalculateAverageStats(context.Background(), dbQueries, sourceId)
	if err != nil {
		log.Printf("Discord: Failed to calculate stats: %v", err)
	} else {
		stats.FollowersCount = memberCount

		if err := common.SaveOrUpdateSourceStats(context.Background(), dbQueries, sourceId, stats); err != nil {
			log.Printf("Discord: Failed to save stats: %v", err)
		}
	}

	return nil
}

func processForumThread(
	ctx context.Context,
	dbQueries *database.Queries,
	session *discordgo.Session,
	sourceId uuid.UUID,
	serverID, channelID string,
	thread *discordgo.Channel,
	exclusionMap map[string]bool,
	processedMessages map[string]struct{},
) error {
	threadID := thread.ID

	if _, exists := processedMessages[threadID]; exists {
		return nil
	}
	processedMessages[threadID] = struct{}{}

	networkInternalID := fmt.Sprintf("%s/%s/%s", serverID, channelID, threadID)

	if exclusionMap[networkInternalID] {
		return nil
	}

	timestamp, err := discordgo.SnowflakeTimestamp(threadID)
	if err != nil {
		return fmt.Errorf("failed to parse thread timestamp: %w", err)
	}

	owner, err := session.User(thread.OwnerID)
	var authorName string
	if err != nil {
		log.Printf("Discord: Failed to fetch thread owner %s: %v, using ID", thread.OwnerID, err)
		authorName = thread.OwnerID
	} else {
		authorName = owner.Username
	}

	postID, err := common.CreateOrUpdatePost(
		ctx,
		dbQueries,
		sourceId,
		networkInternalID,
		"Discord",
		timestamp,
		"thread",
		authorName,
		thread.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to create/update thread post: %w", err)
	}

	totalReactions := 0
	messages, err := session.ChannelMessages(threadID, 1, "", "0", "")
	if err != nil {
		log.Printf("Discord: Failed to fetch first message for thread %s: %v", threadID, err)
	} else if len(messages) > 0 {
		firstMsg := messages[0]
		for _, reaction := range firstMsg.Reactions {
			totalReactions += reaction.Count
		}
	}

	_, err = dbQueries.SyncReactions(ctx, database.SyncReactionsParams{
		ID:       uuid.New(),
		PostID:   postID,
		SyncedAt: time.Now(),
		Likes: sql.NullInt64{
			Int64: int64(totalReactions),
			Valid: true,
		},
		Reposts: sql.NullInt64{},
		Views:   sql.NullInt64{},
	})
	if err != nil {
		return fmt.Errorf("failed to sync reactions: %w", err)
	}

	return nil
}

package sources

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/common"
	"github.com/google/uuid"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func FetchYouTubePosts(dbQueries *database.Queries, sourceId uuid.UUID, encryptionKey []byte) error {
	ctx := context.Background()

	source, err := dbQueries.GetSourceById(ctx, sourceId)
	if err != nil {
		return fmt.Errorf("failed to get source: %w", err)
	}

	token, _, _, _, err := authhelp.GetSourceToken(ctx, dbQueries, encryptionKey, sourceId)
	if err != nil {
		return fmt.Errorf("failed to get source token: %w", err)
	}

	creds, err := google.CredentialsFromJSON(ctx, []byte(token), youtube.YoutubeReadonlyScope)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	service, err := youtube.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return fmt.Errorf("failed to create Youtube service: %w", err)
	}

	call := service.Channels.List([]string{"contentDetails", "id", "snippet", "statistics"}).MaxResults(1)

	if strings.HasPrefix(source.UserName, "@") {
		call = call.ForHandle(source.UserName)
	} else if strings.HasPrefix(source.UserName, "UC") && len(source.UserName) == 24 {
		call = call.Id(source.UserName)
	} else {
		call = call.ForUsername(source.UserName)
	}

	response, err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to get channel details: %w", err)
	}

	if len(response.Items) == 0 {
		return fmt.Errorf("channel not found for %s", source.UserName)
	}

	channel := response.Items[0]
	uploadsPlaylistId := channel.ContentDetails.RelatedPlaylists.Uploads

	if channel.Statistics != nil {
		parsedCount := int(channel.Statistics.SubscriberCount)

		currentStats, _ := common.CalculateAverageStats(context.Background(), dbQueries, sourceId)
		if currentStats == nil {
			currentStats = &common.ProfileStats{}
		}
		currentStats.FollowersCount = &parsedCount
		if err := common.SaveOrUpdateSourceStats(context.Background(), dbQueries, sourceId, currentStats); err != nil {
			log.Printf("Failed to save/update source stats: %v", err)
		}
	}

	exclusionMap, _ := common.LoadExclusionMap(dbQueries, sourceId)

	nextPageToken := ""
	for {
		playlistCall := service.PlaylistItems.List([]string{"snippet", "contentDetails"}).
			PlaylistId(uploadsPlaylistId).
			MaxResults(50).
			PageToken(nextPageToken)

		playlistResponse, err := playlistCall.Do()
		if err != nil {
			return fmt.Errorf("failed to get playlist items: %w", err)
		}

		for _, item := range playlistResponse.Items {
			videoId := item.ContentDetails.VideoId
			if videoId == "" {
				continue
			}

			if exclusionMap[videoId] {
				continue
			}

			pubAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
			if err != nil {
				pubAt = time.Now()
			}

			content := fmt.Sprintf("%s\n\n%s", item.Snippet.Title, item.Snippet.Description)

			postID, err := common.CreateOrUpdatePost(
				ctx,
				dbQueries,
				sourceId,
				videoId,
				"YouTube",
				pubAt,
				"video",
				item.Snippet.ChannelTitle,
				content,
			)
			if err != nil {
				log.Printf("Failed to create/update post %s: %v", videoId, err)
				continue
			}

			videoCall := service.Videos.List([]string{"statistics"}).Id(videoId)
			videoResp, err := videoCall.Do()
			if err != nil {
				log.Printf("Failed to get video stats for %s: %v", videoId, err)
				continue
			}

			if len(videoResp.Items) > 0 {
				vStats := videoResp.Items[0].Statistics

				_, err = dbQueries.SyncReactions(ctx, database.SyncReactionsParams{
					ID:       uuid.New(),
					SyncedAt: time.Now(),
					PostID:   postID,
					Views:    sql.NullInt32{Int32: int32(vStats.ViewCount), Valid: true},
					Likes:    sql.NullInt32{Int32: int32(vStats.LikeCount), Valid: true},
				})
				if err != nil {
					log.Printf("Failed to sync reactions for %s: %v", videoId, err)
				}
			}
		}

		nextPageToken = playlistResponse.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	return nil
}

package noco

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/google/uuid"
)

func SyncNoco(dbQueries *database.Queries, c *common.Client, encryptionKey []byte, target database.Target) error {

	sourcesTable, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "sources",
	})
	if err != nil {
		return fmt.Errorf("failed to get target source table: %w", err)
	}

	err = syncNocoSources(c, dbQueries, encryptionKey, target, sourcesTable.TargetTableCode.String)
	if err != nil {
		return fmt.Errorf("failed to sync sources: %w", err)
	}

	if err := syncNocoAnalyticsSiteStats(dbQueries, c, encryptionKey, target); err != nil {
		return fmt.Errorf("failed to sync site stats: %w", err)
	}

	if err := syncNocoAnalyticsPageStats(dbQueries, c, encryptionKey, target); err != nil {
		return fmt.Errorf("failed to sync page stats: %w", err)
	}

	if err := syncNocoSourcesStats(dbQueries, c, encryptionKey, target); err != nil {
		return fmt.Errorf("failed to sync sources stats: %w", err)
	}

	posts, err := dbQueries.GetAllPostsWithTheLatestInfoForUser(context.Background(), target.UserID)
	if err != nil {
		return err
	}

	targetTable, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "posts",
	})
	if err != nil {
		return fmt.Errorf("failed to get target table: %w", err)
	}

	mappedPosts, err := dbQueries.GetPostsPreviouslySynced(context.Background(), target.ID)
	if err != nil {
		return fmt.Errorf("error fetching mapped posts: %w", err)
	}

	var createPosts []database.GetAllPostsWithTheLatestInfoForUserRow
	var removePosts []database.PostsOnTarget
	var updatePosts []database.GetAllPostsWithTheLatestInfoForUserRow

	localMap := make(map[string]database.GetAllPostsWithTheLatestInfoForUserRow, len(posts))
	mappedMap := make(map[string]database.PostsOnTarget, len(mappedPosts))

	for _, p := range posts {
		localMap[p.ID.String()] = p
	}
	for _, m := range mappedPosts {
		if !m.PostID.Valid {
			removePosts = append(removePosts, m)
		} else {
			mappedMap[m.PostID.UUID.String()] = m
		}
	}

	for id, p := range localMap {
		if _, ok := mappedMap[id]; !ok {
			createPosts = append(createPosts, p)
		} else {
			updatePosts = append(updatePosts, p)
		}
	}
	for id, m := range mappedMap {
		if _, ok := localMap[id]; !ok {
			removePosts = append(removePosts, m)
		}
	}

	const batchSize = 10

	var records []NocoTableRecord

	flush := func(postsInBatch []database.GetAllPostsWithTheLatestInfoForUserRow) error {
		if len(records) == 0 {
			return nil
		}
		createdRecords, err := createNocoRecords(c, dbQueries, encryptionKey, target, targetTable.TargetTableCode.String, records)
		if err != nil {
			return err
		}

		type postInfo struct {
			nocoPostId int32
			sourceId   uuid.UUID
		}
		postMapping := make(map[string]postInfo)

		for _, rec := range createdRecords {
			var id float64
			if val, ok := rec["Id"].(float64); ok {
				id = val
			} else if val, ok := rec["id"].(float64); ok {
				id = val
			} else {
				continue
			}

			fields, ok := rec["fields"].(map[string]interface{})
			if !ok {
				continue
			}

			ctId, ok := fields["ct_id"].(string)
			if !ok {
				continue
			}

			parsedCtId, err := uuid.Parse(ctId)
			if err != nil {
				return fmt.Errorf("failed to parse ct_id: %w", err)
			}

			_, err = dbQueries.AddPostToTarget(context.Background(), database.AddPostToTargetParams{
				ID:            uuid.New(),
				FirstSyncedAt: time.Now(),
				PostID:        uuid.NullUUID{UUID: parsedCtId, Valid: true},
				TargetID:      target.ID,
				TargetPostID:  fmt.Sprintf("%.0f", id),
			})

			if err != nil {
				return fmt.Errorf("failed to map post: %w", err)
			}

			for _, post := range postsInBatch {
				if post.ID == parsedCtId {
					postMapping[ctId] = postInfo{
						nocoPostId: int32(id),
						sourceId:   post.SourceID,
					}
					break
				}
			}
		}

		postsBySource := make(map[uuid.UUID][]int32)
		for _, info := range postMapping {
			postsBySource[info.sourceId] = append(postsBySource[info.sourceId], info.nocoPostId)
		}

		for sourceId, postIds := range postsBySource {
			sourceMapping, err := dbQueries.GetTargetSourceBySource(context.Background(), database.GetTargetSourceBySourceParams{
				TargetID: target.ID,
				SourceID: sourceId,
			})
			if err != nil {
				continue
			}

			sourceNocoId, _ := strconv.Atoi(sourceMapping.TargetSourceID)
			err = linkChildrenToParent(
				c,
				dbQueries,
				encryptionKey,
				target,
				sourcesTable,
				"posts",
				int32(sourceNocoId),
				postIds,
			)
			if err != nil {
				return err
			}
		}

		records = records[:0]
		return nil
	}

	var currentBatch []database.GetAllPostsWithTheLatestInfoForUserRow

	for _, post := range createPosts {

		url, err := common.ConvPostToURL(post.Network.String, post.Author, post.NetworkInternalID)
		if err != nil {
			return err
		}

		fieldMap := NocoRecordFields{
			ID:                post.ID.String(),
			CreatedAt:         post.CreatedAt,
			LastSynced:        time.Now(),
			IsArchived:        post.IsArchived,
			NetworkInternalID: post.NetworkInternalID,
			PostType:          post.PostType,
			Author:            post.Author,
			Content:           post.Content.String,
			Likes:             post.Likes.Int32,
			Views:             post.Views.Int32,
			Reposts:           post.Reposts.Int32,
			URL:               url,
		}

		records = append(records, NocoTableRecord{
			Fields: fieldMap,
		})
		currentBatch = append(currentBatch, post)

		if len(records) == batchSize {
			if err := flush(currentBatch); err != nil {
				return err
			}
			currentBatch = currentBatch[:0]
		}
	}

	if err := flush(currentBatch); err != nil {
		return err
	}

	var recordRemove []NocoDeleteRecord

	flushRemove := func() error {
		if len(recordRemove) == 0 {
			return nil
		}

		if err := deleteNocoRecords(
			c,
			dbQueries,
			encryptionKey,
			target,
			targetTable.TargetTableCode.String,
			recordRemove,
		); err != nil {
			return err
		}

		recordRemove = recordRemove[:0]
		return nil
	}

	for _, post := range removePosts {

		v, _ := strconv.Atoi(post.TargetPostID)
		intId := int32(v)

		recordRemove = append(recordRemove, NocoDeleteRecord{
			ID: intId,
		})

		if len(recordRemove) == batchSize {
			if err := flushRemove(); err != nil {
				return err
			}
		}
	}

	if err := flushRemove(); err != nil {
		return err
	}

	for _, post := range removePosts {
		err := dbQueries.DeletePostOnTarget(context.Background(), post.ID)
		if err != nil {
			log.Printf("Warning: Failed to delete posts_on_target mapping: %v", err)
		}
	}

	var recordsUpdate []NocoTableRecord
	var currentUpdateBatch []database.GetAllPostsWithTheLatestInfoForUserRow

	flushUpdate := func() error {
		if len(recordsUpdate) == 0 {
			return nil
		}

		if err := updateNocoRecords(
			c,
			dbQueries,
			encryptionKey,
			target,
			targetTable.TargetTableCode.String,
			recordsUpdate,
		); err != nil {
			return err
		}

		recordsUpdate = recordsUpdate[:0]
		return nil
	}

	for _, post := range updatePosts {
		mappedPost, ok := mappedMap[post.ID.String()]
		if !ok {
			continue
		}

		url, err := common.ConvPostToURL(post.Network.String, post.Author, post.NetworkInternalID)
		if err != nil {
			return err
		}

		targetPostIDVal, err := strconv.Atoi(mappedPost.TargetPostID)
		if err != nil {
			return fmt.Errorf("invalid target post id %s: %w", mappedPost.TargetPostID, err)
		}

		fieldMap := NocoRecordFields{
			ID:                post.ID.String(),
			CreatedAt:         post.CreatedAt,
			LastSynced:        time.Now(),
			IsArchived:        post.IsArchived,
			NetworkInternalID: post.NetworkInternalID,
			PostType:          post.PostType,
			Author:            post.Author,
			Content:           post.Content.String,
			Likes:             post.Likes.Int32,
			Views:             post.Views.Int32,
			Reposts:           post.Reposts.Int32,
			URL:               url,
		}

		recordsUpdate = append(recordsUpdate, NocoTableRecord{
			Id:     int32(targetPostIDVal),
			Fields: fieldMap,
		})
		currentUpdateBatch = append(currentUpdateBatch, post)

		if len(recordsUpdate) == batchSize {
			if err := flushUpdate(); err != nil {
				return err
			}
			currentUpdateBatch = currentUpdateBatch[:0]
		}
	}

	if err := flushUpdate(); err != nil {
		return err
	}

	return nil
}

func DeletePostsAndSourceNoco(dbQueries *database.Queries, c *common.Client, encryptionKey []byte, target database.Target, source database.Source) error {

	sourceMapping, err := dbQueries.GetTargetSourceBySource(context.Background(), database.GetTargetSourceBySourceParams{
		TargetID: target.ID,
		SourceID: source.ID,
	})
	if err != nil {
		return fmt.Errorf("error fetching source mapping: %w", err)
	}

	sourceId32, _ := strconv.Atoi(sourceMapping.TargetSourceID)
	sourceRecords := []NocoDeleteRecord{
		{ID: int32(sourceId32)},
	}

	sourcesTable, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "sources",
	})
	if err != nil {
		return err
	}

	if err := deleteNocoRecords(
		c,
		dbQueries,
		encryptionKey,
		target,
		sourcesTable.TargetTableCode.String,
		sourceRecords,
	); err != nil {
		return fmt.Errorf("failed to delete source from NocoDB: %w", err)
	}

	postsTable, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "posts",
	})
	if err != nil {
		return err
	}

	postsToDelete, err := dbQueries.GetPostsBySourceAndTarget(context.Background(), database.GetPostsBySourceAndTargetParams{
		TargetID: target.ID,
		SourceID: source.ID,
	})

	const batchSize = 10

	var recordRemove []NocoDeleteRecord

	flushRemove := func() error {
		if len(recordRemove) == 0 {
			return nil
		}

		if err := deleteNocoRecords(
			c,
			dbQueries,
			encryptionKey,
			target,
			postsTable.TargetTableCode.String,
			recordRemove,
		); err != nil {
			return err
		}

		recordRemove = recordRemove[:0]
		return nil
	}

	for _, post := range postsToDelete {
		v, _ := strconv.Atoi(post.TargetPostID)
		intId := int32(v)

		recordRemove = append(recordRemove, NocoDeleteRecord{
			ID: intId,
		})

		if len(recordRemove) == batchSize {
			if err := flushRemove(); err != nil {
				return err
			}
		}
	}

	if err := flushRemove(); err != nil {
		return err
	}

	err = dbQueries.DeletePostsOnTargetAndSource(context.Background(), database.DeletePostsOnTargetAndSourceParams{
		TargetID: target.ID,
		SourceID: source.ID,
	})
	if err != nil {
		return err
	}

	err = dbQueries.DeleteSourceTarget(context.Background(), database.DeleteSourceTargetParams{
		TargetID: target.ID,
		SourceID: source.ID,
	})
	if err != nil {
		return err
	}

	return nil
}

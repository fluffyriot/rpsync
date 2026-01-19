package puller

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/authhelp"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

type NocoTableRecord struct {
	Fields NocoRecordFields `json:"fields,omitempty"`
}

type NocoDeleteRecord struct {
	ID int32 `json:"id"`
}

type NocoRecordFields struct {
	ID                string    `json:"ct_id"`
	CreatedAt         time.Time `json:"created_at,omitempty"`
	LastSynced        time.Time `json:"last_synced,omitempty"`
	IsArchived        bool      `json:"is_archived,omitempty"`
	NetworkInternalID string    `json:"network_internal_id,omitempty"`
	Network           string    `json:"network,omitempty"`
	Username          string    `json:"username,omitempty"`
	PostType          string    `json:"post_type,omitempty"`
	Author            string    `json:"author,omitempty"`
	Content           string    `json:"content,omitempty"`
	Likes             int32     `json:"likes,omitempty"`
	Views             int32     `json:"views,omitempty"`
	Reposts           int32     `json:"reposts,omitempty"`
	URL               string    `json:"URL,omitempty"`
}

type NocoColumnTypeOptions struct {
	Title string `json:"title"`
}

type NocoColumnTypeSelectOptions struct {
	Choices []NocoColumnTypeOptions `json:"choices"`
}

type NocoColumnTypeRelation struct {
	RelationType   string `json:"relation_type,omitempty"`
	RelatedTableId string `json:"related_table_id,omitempty"`
}

type NocoColumn struct {
	Title       string      `json:"title"`
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Unique      bool        `json:"unique,omitempty"`
	Options     interface{} `json:"options,omitempty"`
}

type NocoTable struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Fields      []NocoColumn `json:"fields"`
}

type NocoCreateTableResponse struct {
	ID     string           `json:"id"`
	Title  string           `json:"title"`
	Fields []NocoColumnInfo `json:"fields"`
}

type NocoColumnInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func InitializeNoco(dbQueries *database.Queries, c *Client, encryptionKey []byte, target database.Target) error {

	nocoURL := target.HostUrl.String +
		"/api/v3/meta/bases/" +
		target.DbID.String +
		"/tables"

	postsTable := NocoTable{
		Title:       "Posts",
		Description: "Posts from your social networks",
		Fields: []NocoColumn{
			{Title: "ct_id", Type: "SingleLineText", Unique: true},
			{Title: "created_at", Type: "DateTime"},
			{Title: "last_synced", Type: "DateTime"},
			{Title: "is_archived", Type: "Checkbox"},
			{Title: "network_internal_id", Type: "SingleLineText"},
			{Title: "post_type", Type: "SingleLineText"},
			{Title: "author", Type: "SingleLineText"},
			{Title: "content", Type: "LongText"},
			{Title: "likes", Type: "Number"},
			{Title: "views", Type: "Number"},
			{Title: "reposts", Type: "Number"},
			{Title: "URL", Type: "URL"},
		},
	}

	postsResp, err := createNocoTable(c, dbQueries, encryptionKey, target.ID, nocoURL, postsTable)
	if err != nil {
		return err
	}

	postsMapping, err := dbQueries.CreateMappingForTable(context.Background(), database.CreateMappingForTableParams{
		ID:              uuid.New(),
		CreatedAt:       time.Now(),
		SourceTableName: "posts",
		TargetTableName: postsResp.Title,
		TargetTableCode: sql.NullString{String: postsResp.ID, Valid: true},
		TargetID:        target.ID,
	})
	if err != nil {
		return fmt.Errorf("create posts table mapping: %w", err)
	}

	for _, field := range postsResp.Fields {
		_, err := dbQueries.CreateMappingForColumn(context.Background(), database.CreateMappingForColumnParams{
			ID:               uuid.New(),
			CreatedAt:        time.Now(),
			TableMappingID:   postsMapping.ID,
			SourceColumnName: field.Title,
			TargetColumnName: field.Title,
			TargetColumnCode: sql.NullString{String: field.ID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create posts column mapping %s: %w", field.Title, err)
		}
	}

	sourcesTable := NocoTable{
		Title:       "Sources",
		Description: "Social media sources",
		Fields: []NocoColumn{
			{Title: "ct_id", Type: "SingleLineText", Unique: true},
			{
				Title: "network",
				Type:  "SingleSelect",
				Options: NocoColumnTypeSelectOptions{
					Choices: []NocoColumnTypeOptions{
						{Title: "Instagram"},
						{Title: "Bluesky"},
						{Title: "Murrtube"},
						{Title: "BadPups"},
						{Title: "TikTok"},
						{Title: "Mastodon"},
						{Title: "Telegram"},
					}},
			},
			{Title: "username", Type: "SingleLineText"},
			{Title: "URL", Type: "URL"},
			{Title: "last_synced", Type: "DateTime"},
			{
				Title: "posts",
				Type:  "Links",
				Options: NocoColumnTypeRelation{
					RelationType:   "hm",
					RelatedTableId: postsResp.ID,
				},
			},
		},
	}

	sourcesResp, err := createNocoTable(c, dbQueries, encryptionKey, target.ID, nocoURL, sourcesTable)
	if err != nil {
		return err
	}

	sourcesMapping, err := dbQueries.CreateMappingForTable(context.Background(), database.CreateMappingForTableParams{
		ID:              uuid.New(),
		CreatedAt:       time.Now(),
		SourceTableName: "sources",
		TargetTableName: sourcesResp.Title,
		TargetTableCode: sql.NullString{String: sourcesResp.ID, Valid: true},
		TargetID:        target.ID,
	})
	if err != nil {
		return fmt.Errorf("create sources table mapping: %w", err)
	}

	for _, field := range sourcesResp.Fields {
		_, err := dbQueries.CreateMappingForColumn(context.Background(), database.CreateMappingForColumnParams{
			ID:               uuid.New(),
			CreatedAt:        time.Now(),
			TableMappingID:   sourcesMapping.ID,
			SourceColumnName: field.Title,
			TargetColumnName: field.Title,
			TargetColumnCode: sql.NullString{String: field.ID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create sources column mapping %s: %w", field.Title, err)
		}
	}

	err = syncNocoSources(c, dbQueries, encryptionKey, target, sourcesResp.ID)
	if err != nil {
		return err
	}

	return nil
}

func SyncNoco(dbQueries *database.Queries, c *Client, encryptionKey []byte, target database.Target) error {

	sourcesTable, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "Sources",
	})
	if err != nil {
		return fmt.Errorf("failed to get target source table: %w", err)
	}

	err = syncNocoSources(c, dbQueries, encryptionKey, target, sourcesTable.TargetTableCode.String)
	if err != nil {
		return fmt.Errorf("failed to sync sources: %w", err)
	}

	posts, err := dbQueries.GetAllPostsWithTheLatestInfoForUser(context.Background(), target.UserID)
	if err != nil {
		return err
	}

	targetTable, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "Posts",
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

	localMap := make(map[string]database.GetAllPostsWithTheLatestInfoForUserRow, len(posts))
	mappedMap := make(map[string]database.PostsOnTarget, len(mappedPosts))

	for _, p := range posts {
		localMap[p.ID.String()] = p
	}
	for _, m := range mappedPosts {
		mappedMap[m.PostID.String()] = m
	}

	for id, p := range localMap {
		if _, ok := mappedMap[id]; !ok {
			createPosts = append(createPosts, p)
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
				PostID:        parsedCtId,
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

			err = linkPostsToSource(
				c,
				dbQueries,
				encryptionKey,
				target,
				sourceMapping.TargetSourceID,
				sourcesTable,
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

		url, err := ConvPostToURL(post.Network.String, post.Author, post.NetworkInternalID)
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

	return nil
}

func DeletePostsAndSourceNoco(dbQueries *database.Queries, c *Client, encryptionKey []byte, target database.Target, source database.Source) error {

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
		TargetTableName: "Sources",
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
		TargetTableName: "Posts",
	})
	if err != nil {
		return err
	}

	postsToDelete, err := dbQueries.GetPostsBySourceAndTarget(context.Background(), database.GetPostsBySourceAndTargetParams{
		TargetID: target.ID,
		SourceID: source.ID,
	})

	sourcePostIds := make(map[string]bool)
	for _, post := range postsToDelete {
		sourcePostIds[post.TargetPostID] = true
	}

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

func createNocoTable(c *Client, dbQueries *database.Queries, encryptionKey []byte, targetID uuid.UUID, url string, table NocoTable) (*NocoCreateTableResponse, error) {

	body, err := json.Marshal(table)
	if err != nil {
		return nil, fmt.Errorf("marshal table schema: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	err = setNocoHeaders(targetID, req, dbQueries, encryptionKey)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d, %v", resp.StatusCode, resp.Status)
	}

	var result NocoCreateTableResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

func createNocoRecords(c *Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableId string, records []NocoTableRecord) ([]map[string]interface{}, error) {

	url := target.HostUrl.String +
		"/api/v3/data/" +
		target.DbID.String +
		"/" + tableId + "/records"

	body, err := json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("marshal records schema: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	err = setNocoHeaders(target.ID, req, dbQueries, encryptionKey)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d, %v", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var wrapper map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &wrapper); err == nil {
		if recordsVal, ok := wrapper["records"]; ok {
			recordsBytes, _ := json.Marshal(recordsVal)
			var records []map[string]interface{}
			if err := json.Unmarshal(recordsBytes, &records); err == nil {
				return records, nil
			}
		}
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err == nil {
		return result, nil
	}

	return nil, fmt.Errorf("decode response failed. Body: %s", string(bodyBytes))
}

func syncNocoSources(c *Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableId string) error {

	var createSources []database.Source
	var removeSources []database.SourcesOnTarget

	userSources, err := dbQueries.GetUserSources(context.Background(), target.UserID)
	if err != nil {
		return fmt.Errorf("error fetching user sources: %w", err)
	}

	mappedSources, err := dbQueries.GetTargetSources(context.Background(), target.ID)
	if err != nil {
		return fmt.Errorf("error fetching user sources: %w", err)
	}

	internMap := make(map[string]database.Source, len(userSources))
	mappedMap := make(map[string]database.SourcesOnTarget, len(mappedSources))

	for _, uSource := range userSources {
		internMap[uSource.ID.String()] = uSource
	}

	for _, mSource := range mappedSources {
		mappedMap[mSource.SourceID.String()] = mSource
	}

	for id, uSource := range internMap {
		if _, ok := mappedMap[id]; !ok {
			createSources = append(createSources, uSource)
		}
	}

	for id, mSource := range mappedMap {
		if _, ok := internMap[id]; !ok {
			removeSources = append(removeSources, mSource)
		}
	}

	const batchSize = 10

	var recordsCreate []NocoTableRecord

	flush := func() error {
		if len(recordsCreate) == 0 {
			return nil
		}

		createdRecords, err := createNocoRecords(
			c,
			dbQueries,
			encryptionKey,
			target,
			tableId,
			recordsCreate,
		)
		if err != nil {
			return err
		}

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

			_, err = dbQueries.AddSourceToTarget(context.Background(), database.AddSourceToTargetParams{
				ID:             uuid.New(),
				SourceID:       parsedCtId,
				TargetID:       target.ID,
				TargetSourceID: fmt.Sprintf("%.0f", id),
			})

			if err != nil {
				return fmt.Errorf("failed to map source: %w", err)
			}

		}

		recordsCreate = recordsCreate[:0]
		return nil
	}

	for _, source := range createSources {

		url, err := ConvNetworkToURL(source.Network, source.UserName)
		if err != nil {
			return err
		}

		fieldMap := NocoRecordFields{
			ID:         source.ID.String(),
			Network:    source.Network,
			Username:   source.UserName,
			URL:        url,
			LastSynced: source.LastSynced.Time,
		}

		recordsCreate = append(recordsCreate, NocoTableRecord{
			Fields: fieldMap,
		})

		if len(recordsCreate) == batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	if err := flush(); err != nil {
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
			tableId,
			recordRemove,
		); err != nil {
			return err
		}

		recordRemove = recordRemove[:0]
		return nil
	}

	for _, source := range removeSources {

		v, _ := strconv.Atoi(source.TargetSourceID)
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

	return nil

}

func deleteNocoRecords(c *Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableId string, records []NocoDeleteRecord) error {

	url := target.HostUrl.String +
		"/api/v3/data/" +
		target.DbID.String +
		"/" + tableId + "/records"

	body, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("marshal records schema: %w", err)
	}

	req, err := http.NewRequest("DELETE", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	err = setNocoHeaders(target.ID, req, dbQueries, encryptionKey)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d, %v", resp.StatusCode, resp.Status)
	}

	return nil

}

func linkPostsToSource(c *Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, sourceId string, sourceTableNocoId database.TableMapping, postIds []int32) error {

	postsColumn, err := dbQueries.GetColumnMappingsByTableAndName(context.Background(), database.GetColumnMappingsByTableAndNameParams{
		TableMappingID:   sourceTableNocoId.ID,
		TargetColumnName: "posts",
	})
	if err != nil {
		return err
	}

	if len(postIds) > 10 {
		return fmt.Errorf("cannot link more than 10 posts per request, got %d", len(postIds))
	}

	type linkRecord struct {
		ID int32 `json:"id"`
	}

	linkRecords := make([]linkRecord, len(postIds))
	for i, postId := range postIds {
		linkRecords[i] = linkRecord{ID: postId}
	}

	url := target.HostUrl.String +
		"/api/v3/data/" +
		target.DbID.String +
		"/" + sourceTableNocoId.TargetTableCode.String +
		"/links/" + postsColumn.TargetColumnCode.String +
		"/" + sourceId

	body, err := json.Marshal(linkRecords)
	if err != nil {
		return fmt.Errorf("marshal link records: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	err = setNocoHeaders(target.ID, req, dbQueries, encryptionKey)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func setNocoHeaders(tid uuid.UUID, req *http.Request, dbQueries *database.Queries, encryptionKey []byte) error {
	token, _, _, err := authhelp.GetTargetToken(context.Background(), dbQueries, encryptionKey, tid)
	if err != nil {
		return err
	}
	req.Header.Set("xc-auth", token)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return nil
}

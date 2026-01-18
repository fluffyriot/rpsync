package puller

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/auth"
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

func createNocoRecords(c *Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableId string, records []NocoTableRecord) error {

	url := target.HostUrl.String +
		"/api/v3/data/" +
		target.DbID.String +
		"/" + tableId + "/records"

	body, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("marshal records schema: %w", err)
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
		return fmt.Errorf("unexpected status code: %d, %v", resp.StatusCode, resp.Status)
	}

	return nil
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

		if err := createNocoRecords(
			c,
			dbQueries,
			encryptionKey,
			target,
			tableId,
			recordsCreate,
		); err != nil {
			return err
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

	flush = func() error {
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
			if err := flush(); err != nil {
				return err
			}
		}
	}

	if err := flush(); err != nil {
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

func setNocoHeaders(tid uuid.UUID, req *http.Request, dbQueries *database.Queries, encryptionKey []byte) error {
	token, _, _, err := auth.GetTargetToken(context.Background(), dbQueries, encryptionKey, tid)
	if err != nil {
		return err
	}
	req.Header.Set("xc-auth", token)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return nil
}

func SyncNoco(dbQueries *database.Queries, c *Client, encryptionKey []byte, target database.Target) error {

	posts, err := dbQueries.GetAllPostsWithTheLatestInfoForUser(context.Background(), target.UserID)
	if err != nil {
		return err
	}

	targetTable, err := dbQueries.GetTableMappingsByTargetAndName(context.Background(), database.GetTableMappingsByTargetAndNameParams{
		TargetID:        target.ID,
		TargetTableName: "Posts",
	})

	const batchSize = 10

	var records []NocoTableRecord

	flush := func() error {
		if len(records) == 0 {
			return nil
		}
		err := createNocoRecords(c, dbQueries, encryptionKey, target, targetTable.TargetTableCode.String, records)
		if err != nil {
			return err
		}

		records = records[:0]
		return nil
	}

	for _, post := range posts {

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

		if len(records) == batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	if err := flush(); err != nil {
		return err
	}

	return nil
}

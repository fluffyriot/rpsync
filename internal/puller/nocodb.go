package puller

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/auth"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

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
	Fields []NocoColumnInfo `json:"columns"`
}

type NocoColumnInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func InitializeNoco(tid uuid.UUID, dbQueries *database.Queries, c *Client, encryptionKey []byte, target database.Target) error {

	nocoURL := target.HostUrl.String +
		"/api/v3/meta/bases/" +
		target.DbID.String +
		"/tables"

	postsTable := NocoTable{
		Title:       "Posts",
		Description: "Posts from your social networks",
		Fields: []NocoColumn{
			{Title: "id", Type: "SingleLineText", Unique: true, Description: "Unique Post Id"},
			{Title: "created_at", Type: "DateTime"},
			{Title: "last_synced_at", Type: "DateTime"},
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
		SourceTableName: "Posts",
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
			{Title: "id", Type: "SingleLineText", Unique: true},
			{
				Title: "network",
				Type:  "SingleSelect",
				Options: NocoColumnTypeSelectOptions{
					Choices: []NocoColumnTypeOptions{
						{Title: "Instagram"},
						{Title: "Bluesky"},
						{Title: "Murrtube"},
						{Title: "Badpups"},
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
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result NocoCreateTableResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
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

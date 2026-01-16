package puller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fluffyriot/commission-tracker/internal/auth"
	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

type NocoColumnTypeOptions struct {
	Title string `json:"title"`
}

type NocoColumnTypeSelectOptions struct {
	Options []NocoColumnTypeOptions `json:"options"`
}

type NocoColumnTypeRelation struct {
	Type     string `json:"type,omitempty"`
	ChildId  string `json:"fk_child_column_id,omitempty"`
	ParentId string `json:"fk_parent_column_id,omitempty"`
}

type NocoColumn struct {
	Title       string      `json:"title"`
	UIDT        string      `json:"uidt"`
	Description string      `json:"description,omitempty"`
	PV          bool        `json:"pv,omitempty"`
	RQD         bool        `json:"rqd,omitempty"`
	ColOptions  interface{} `json:"colOptions,omitempty"`
}

type NocoTable struct {
	TableName   string       `json:"table_name,omitempty"`
	Description string       `json:"description,omitempty"`
	Title       string       `json:"title"`
	Columns     []NocoColumn `json:"columns"`
}

type NocoCreateTableResponse struct {
	ID      string           `json:"id"`
	Title   string           `json:"title"`
	Name    string           `json:"table_name"`
	Columns []NocoColumnInfo `json:"columns"`
}

type NocoColumnInfo struct {
	ID         string `json:"id"`
	ColumnName string `json:"column_name"`
	Title      string `json:"title"`
}

func InitializeNoco(tid uuid.UUID, dbQueries *database.Queries, c *Client, encryptionKey []byte, target database.Target) error {

	nocoURL := target.HostUrl.String +
		"/api/v2/meta/bases/" +
		target.DbID.String +
		"/tables"

	sourcesTable := NocoTable{
		TableName:   "Sources",
		Description: "Social media sources",
		Title:       "sources",
		Columns: []NocoColumn{
			{Title: "id", UIDT: "SingleLineText", PV: true, RQD: true},
			{
				Title: "network",
				UIDT:  "SingleSelect",
				ColOptions: NocoColumnTypeSelectOptions{
					[]NocoColumnTypeOptions{
						{Title: "Instagram"},
						{Title: "Bluesky"},
						{Title: "Murrtube"},
						{Title: "Badpups"},
						{Title: "TikTok"},
						{Title: "Mastodon"},
					}},
			},
			{Title: "username", UIDT: "SingleLineText"},
			{Title: "URL", UIDT: "URL"},
			{Title: "last_synced", UIDT: "DateTime"},
			{Title: "posts", UIDT: "LinkToAnotherRecord"},
		},
	}

	var (
		sourceIDColID    string
		sourcePostsColID string
	)

	sourceResp, err := createNocoTable(c, dbQueries, encryptionKey, target.ID, nocoURL, sourcesTable)
	if err != nil {
		return err
	}

	for _, col := range sourceResp.Columns {
		if col.Title == "posts" {
			sourceIDColID = col.ID
			break
		}
	}

	postsTable := NocoTable{
		TableName:   "Posts",
		Description: "Posts from your social networks",
		Title:       "posts",
		Columns: []NocoColumn{
			{Title: "id", UIDT: "SingleLineText", PV: true, RQD: true, Description: "Unique Post Id"},
			{Title: "created_at", UIDT: "DateTime"},
			{Title: "last_synced_at", UIDT: "DateTime"},
			{Title: "source_id", UIDT: "LinkToAnotherRecord", Description: "Source network of the post"},
			{Title: "is_archived", UIDT: "Checkbox"},
			{Title: "network_internal_id", UIDT: "SingleLineText"},
			{Title: "post_type", UIDT: "SingleLineText"},
			{Title: "author", UIDT: "SingleLineText"},
			{Title: "content", UIDT: "LongText"},
			{Title: "likes", UIDT: "Number"},
			{Title: "views", UIDT: "Number"},
			{Title: "reposts", UIDT: "Number"},
			{Title: "URL", UIDT: "URL"},
		},
	}

	sourceResp, err = createNocoTable(c, dbQueries, encryptionKey, target.ID, nocoURL, postsTable)
	if err != nil {
		return err
	}
	for _, col := range sourceResp.Columns {
		if col.Title == "source_id" {
			sourcePostsColID = col.ID
			break
		}
	}

	err = connectNocoRelation(c, dbQueries, encryptionKey, target, sourcePostsColID, NocoColumnTypeRelation{
		Type:     "oo",
		ParentId: sourceIDColID,
		ChildId:  sourcePostsColID,
	})
	if err != nil {
		return err
	}

	err = connectNocoRelation(c, dbQueries, encryptionKey, target, sourceIDColID, NocoColumnTypeRelation{
		Type:     "om",
		ParentId: sourcePostsColID,
		ChildId:  sourceIDColID,
	})
	if err != nil {
		return err
	}

	return err
}

func createNocoTable(c *Client, dbQueries *database.Queries, encryptionKey []byte, targetID uuid.UUID, url string, table NocoTable) (*NocoCreateTableResponse, error) {

	body, err := json.Marshal(table)
	if err != nil {
		return nil, fmt.Errorf("marshal table schema: %w", err)
	}

	// DEBUG: preview JSON sent to Noco
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		fmt.Println("Noco payload:\n", pretty.String())
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

func connectNocoRelation(c *Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, columnID string, relation NocoColumnTypeRelation) error {

	url := fmt.Sprintf(
		"%s/api/v2/meta/columns/%s",
		target.HostUrl.String,
		columnID,
	)

	payload := map[string]interface{}{
		"colOptions": relation,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	if err := setNocoHeaders(target.ID, req, dbQueries, encryptionKey); err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("relation patch failed: %d", resp.StatusCode)
	}

	return nil
}

func setNocoHeaders(tid uuid.UUID, req *http.Request, dbQueries *database.Queries, encryptionKey []byte) error {
	token, _, _, err := auth.GetTargetToken(context.Background(), dbQueries, encryptionKey, tid)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return nil
}

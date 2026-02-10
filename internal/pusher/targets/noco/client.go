// SPDX-License-Identifier: AGPL-3.0-only
package noco

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
	"github.com/google/uuid"
)

func createNocoTable(c *common.Client, dbQueries *database.Queries, encryptionKey []byte, targetID uuid.UUID, url string, table NocoTable) (*NocoCreateTableResponse, error) {

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

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("NocoDB Error: Status: %d, Body: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("unexpected status code: %d, %v. Body: %s", resp.StatusCode, resp.Status, string(bodyBytes))
	}

	var result NocoCreateTableResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

func createNocoRecords(c *common.Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableId string, records []NocoTableRecord) ([]map[string]any, error) {

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

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("NocoDB Create Record Error: Status: %d, Body: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("unexpected status code: %d, %v", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var wrapper map[string]any
	if err := json.Unmarshal(bodyBytes, &wrapper); err == nil {
		if recordsVal, ok := wrapper["records"]; ok {
			recordsBytes, _ := json.Marshal(recordsVal)
			var records []map[string]any
			if err := json.Unmarshal(recordsBytes, &records); err == nil {
				return records, nil
			}
		}
	}

	var result []map[string]any
	if err := json.Unmarshal(bodyBytes, &result); err == nil {
		return result, nil
	}

	return nil, fmt.Errorf("decode response failed. Body: %s", string(bodyBytes))
}

func updateNocoRecords(c *common.Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableId string, records []NocoTableRecord) error {

	url := target.HostUrl.String +
		"/api/v3/data/" +
		target.DbID.String +
		"/" + tableId + "/records"

	body, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("marshal records schema: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	err = setNocoHeaders(target.ID, req, dbQueries, encryptionKey)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func deleteNocoRecords(c *common.Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableId string, records []NocoDeleteRecord) error {

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

	resp, err := c.HTTPClient.Do(req)
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
	token, _, _, err := authhelp.GetTargetToken(context.Background(), dbQueries, encryptionKey, tid)
	if err != nil {
		return err
	}
	req.Header.Set("xc-auth", token)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return nil
}

func createNocoColumn(c *common.Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableID string, column NocoColumn) (*NocoColumnInfo, error) {
	url := target.HostUrl.String +
		"/api/v3/meta/bases/" +
		target.DbID.String +
		"/tables/" + tableID + "/fields"

	body, err := json.Marshal(column)
	if err != nil {
		return nil, fmt.Errorf("marshal column schema: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	err = setNocoHeaders(target.ID, req, dbQueries, encryptionKey)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create column error: status %d, body %s", resp.StatusCode, string(bodyBytes))
	}

	var result NocoColumnInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

func updateNocoColumn(c *common.Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, tableID, columnID string, column NocoColumn) error {
	url := target.HostUrl.String +
		"/api/v3/meta/bases/" +
		target.DbID.String +
		"/tables/" + tableID + "/fields/" + columnID

	body, err := json.Marshal(column)
	if err != nil {
		return fmt.Errorf("marshal column schema: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	err = setNocoHeaders(target.ID, req, dbQueries, encryptionKey)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update column error: status %d, body %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

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

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/pusher/common"
)

func linkChildrenToParent(c *common.Client, dbQueries *database.Queries, encryptionKey []byte, target database.Target, parentTableMapping database.TableMapping, columnName string, parentRecordID int, childRecordIDs []int) error {

	colMapping, err := dbQueries.GetColumnMappingsByTableAndName(context.Background(), database.GetColumnMappingsByTableAndNameParams{
		TableMappingID:   parentTableMapping.ID,
		TargetColumnName: columnName,
	})
	if err != nil {
		return err
	}

	if len(childRecordIDs) > 10 {
		return fmt.Errorf("cannot link more than 10 records per request, got %d", len(childRecordIDs))
	}

	type linkRecord struct {
		ID int `json:"id"`
	}

	linkRecords := make([]linkRecord, len(childRecordIDs))
	for i, childID := range childRecordIDs {
		linkRecords[i] = linkRecord{ID: childID}
	}

	url := target.HostUrl.String +
		"/api/v3/data/" +
		target.DbID.String +
		"/" + parentTableMapping.TargetTableCode.String +
		"/links/" + colMapping.TargetColumnCode.String +
		"/" + fmt.Sprintf("%d", parentRecordID)

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

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("NocoDB Link Error: Status: %d, Body: %s", resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

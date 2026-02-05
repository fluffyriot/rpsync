package common

import (
	"context"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/google/uuid"
)

func LoadExclusionMap(dbQueries *database.Queries, sourceID uuid.UUID) (map[string]bool, error) {
	exclusions, err := dbQueries.GetExclusionsForSource(context.Background(), sourceID)
	if err != nil {
		return nil, err
	}

	exclusionMap := make(map[string]bool, len(exclusions))
	for _, excl := range exclusions {
		exclusionMap[excl.NetworkInternalID] = true
	}

	return exclusionMap, nil
}

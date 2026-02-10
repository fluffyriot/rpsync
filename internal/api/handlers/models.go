// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import "github.com/google/uuid"

type TopSourceViewModel struct {
	ID                uuid.UUID
	UserName          string
	Network           string
	TotalInteractions int64
	FollowersCount    int64
	ProfileURL        string
}

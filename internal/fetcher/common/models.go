package common

import (
	"time"

	"github.com/google/uuid"
)

type Media struct {
	internalId uuid.UUID
	name       string
}

type Post struct {
	internalId        uuid.UUID
	postedOn          time.Time
	lastUpdated       time.Time
	networkName       string
	isArchived        bool
	networkInternalId string
	content           string
	url               string
	media             []Media
}

type PostReactions struct {
	internalId uuid.UUID
	syncedAt   time.Time
	postId     uuid.UUID
	likes      int
	reposts    int
	views      int
}

type ProfileStats struct {
	FollowersCount *int
	FollowingCount *int
	PostsCount     *int
	AverageLikes   *float64
	AverageReposts *float64
	AverageViews   *float64
}

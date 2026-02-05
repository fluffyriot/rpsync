package noco

import "time"

type NocoTableRecord struct {
	Id     int32            `json:"id,omitempty"`
	Fields NocoRecordFields `json:"fields,omitempty"`
}

type NocoDeleteRecord struct {
	ID int32 `json:"id"`
}

type NocoRecordFields struct {
	ID                 string    `json:"ct_id"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
	LastSynced         time.Time `json:"last_synced,omitempty"`
	IsArchived         bool      `json:"is_archived"`
	NetworkInternalID  string    `json:"network_internal_id,omitempty"`
	Network            string    `json:"network,omitempty"`
	Username           string    `json:"username,omitempty"`
	PostType           string    `json:"post_type,omitempty"`
	Author             string    `json:"author,omitempty"`
	Content            string    `json:"content,omitempty"`
	Likes              int32     `json:"likes,omitempty"`
	Views              int32     `json:"views,omitempty"`
	Reposts            int32     `json:"reposts,omitempty"`
	URL                string    `json:"URL,omitempty"`
	Date               time.Time `json:"date,omitempty"`
	Visitors           int32     `json:"visitors,omitempty"`
	AvgSessionDuration float64   `json:"avg_session_duration,omitempty"`
	PagePath           string    `json:"page_path,omitempty"`
	FollowersCount     int32     `json:"followers_count,omitempty"`
	FollowingCount     int32     `json:"following_count,omitempty"`
	PostsCount         int32     `json:"posts_count,omitempty"`
	AverageLikes       float64   `json:"average_likes,omitempty"`
	AverageReposts     float64   `json:"average_reposts,omitempty"`
	AverageViews       float64   `json:"average_views,omitempty"`
}

type NocoColumnTypeOptions struct {
	Title string `json:"title"`
	Color string `json:"color,omitempty"`
}

type NocoColumnTypeSelectOptions struct {
	Choices []NocoColumnTypeOptions `json:"choices"`
}

type NocoColumnTypeRelation struct {
	RelationType   string `json:"relation_type,omitempty"`
	RelatedTableId string `json:"related_table_id,omitempty"`
}

type NocoColumn struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Unique      bool   `json:"unique,omitempty"`
	Options     any    `json:"options,omitempty"`
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

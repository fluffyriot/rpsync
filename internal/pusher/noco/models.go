package noco

import "time"

// NocoTableRecord represents a record in a NocoDB table
type NocoTableRecord struct {
	Id     int32            `json:"id,omitempty"`
	Fields NocoRecordFields `json:"fields,omitempty"`
}

// NocoDeleteRecord represents a record to be deleted
type NocoDeleteRecord struct {
	ID int32 `json:"id"`
}

// NocoRecordFields contains all possible fields for NocoDB records
type NocoRecordFields struct {
	ID                 string    `json:"ct_id"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
	LastSynced         time.Time `json:"last_synced,omitempty"`
	IsArchived         bool      `json:"is_archived,omitempty"`
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

// NocoColumnTypeOptions represents options for a column type
type NocoColumnTypeOptions struct {
	Title string `json:"title"`
}

// NocoColumnTypeSelectOptions represents select options for a column
type NocoColumnTypeSelectOptions struct {
	Choices []NocoColumnTypeOptions `json:"choices"`
}

// NocoColumnTypeRelation represents a relation between tables
type NocoColumnTypeRelation struct {
	RelationType   string `json:"relation_type,omitempty"`
	RelatedTableId string `json:"related_table_id,omitempty"`
}

// NocoColumn represents a column definition in a NocoDB table
type NocoColumn struct {
	Title       string      `json:"title"`
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Unique      bool        `json:"unique,omitempty"`
	Options     interface{} `json:"options,omitempty"`
}

// NocoTable represents a table definition for creation
type NocoTable struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Fields      []NocoColumn `json:"fields"`
}

// NocoCreateTableResponse represents the response from table creation
type NocoCreateTableResponse struct {
	ID     string           `json:"id"`
	Title  string           `json:"title"`
	Fields []NocoColumnInfo `json:"fields"`
}

// NocoColumnInfo represents column information in a response
type NocoColumnInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

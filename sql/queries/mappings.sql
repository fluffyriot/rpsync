-- name: CreateMappingForTable :one
INSERT INTO table_mappings (id, created_at, source_table_name, target_table_name, target_table_code, target_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: CreateMappingForColumn :one
INSERT INTO column_mappings (id, created_at, table_mapping_id, source_column_name, target_column_name, target_column_code)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetTableMappingsByTargetAndName :one
SELECT * FROM table_mappings
where target_id = $1 and target_table_name = $2;

-- name: GetColumnMappingsByTable :many
SELECT * FROM column_mappings
where table_mapping_id = $1;

-- name: GetColumnMappingsByTableAndName :one
SELECT cm.* FROM column_mappings cm
left join table_mappings tm on cm.table_mapping_id = tm.id
where cm.table_mapping_id = $1 and cm.target_column_name = $2;

-- name: GetTableMappingsByTargetAndCode :one
SELECT * FROM table_mappings
WHERE target_id = $1 AND target_table_code = $2;
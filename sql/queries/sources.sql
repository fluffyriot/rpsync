-- name: CreateSource :one
INSERT INTO sources (id, created_at, updated_at, network, user_name, user_id, is_active, sync_status, status_reason, last_synced)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10
)
RETURNING *;

-- name: GetUserActiveSourceByName :one
SELECT * FROM sources
where user_id = $1 and network = $2 and is_active = TRUE
LIMIT 1;

-- name: GetUserActiveSources :many
SELECT * FROM sources
where user_id = $1 and is_active = TRUE;

-- name: GetUserSources :many
SELECT * FROM sources
where user_id = $1;

-- name: GetSourceById :one
SELECT * FROM sources
where id = $1;

-- name: ChangeSourceStatusById :one
UPDATE sources
SET is_active = $2, sync_status = $3, status_reason = $4
WHERE id = $1
RETURNING *;

-- name: UpdateSourceSyncStatusById :one
UPDATE sources
SET sync_status = $2, status_reason = $3, last_synced = $4
WHERE id = $1
RETURNING *;
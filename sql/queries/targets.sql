-- name: CreateTarget :one
INSERT INTO targets (id, created_at, updated_at, target_type, user_id, db_id, is_active, sync_frequency, sync_status)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
RETURNING *;

-- name: GetAllTargets :many
SELECT * FROM targets;

-- name: GetAllActiveTargets :many
SELECT * FROM targets
where is_active = TRUE;
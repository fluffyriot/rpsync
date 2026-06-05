-- name: CreateTarget :one
INSERT INTO targets (id, created_at, updated_at, target_type, user_id, db_id, is_active, sync_frequency, sync_status, host_url)
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

-- name: GetUserTargets :many
SELECT * FROM targets
where user_id = $1
ORDER BY
  CASE sync_status
    WHEN 'Initialized' THEN 1
    WHEN 'Deactivated' THEN 2
    WHEN 'Syncing' THEN 3
    ELSE 4
  END,
  last_synced DESC;

-- name: GetUserActiveTargets :many
SELECT * FROM targets
where is_active = TRUE and user_id = $1;

-- name: ChangeTargetStatusById :one
UPDATE targets
SET is_active = $2, sync_status = $3, status_reason = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteTarget :exec
DELETE FROM targets
WHERE id = $1;

-- name: GetTargetById :one
SELECT * FROM targets
where id = $1;

-- name: UpdateTargetSyncStatusById :one
UPDATE targets
SET sync_status = $2, status_reason = $3, last_synced = $4
WHERE id = $1
RETURNING *;
-- name: CreateUser :one
INSERT INTO users (id, username, created_at, updated_at, sync_method, access_key, target_database_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING *;

-- name: EmptyUsers :exec
DELETE FROM users;

-- name: GetAllUsers :many
SELECT * FROM users;
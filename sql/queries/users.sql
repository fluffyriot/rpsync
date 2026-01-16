-- name: CreateUser :one
INSERT INTO users (id, username, created_at, updated_at)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: EmptyUsers :exec
DELETE FROM users;

-- name: GetAllUsers :many
SELECT * FROM users;
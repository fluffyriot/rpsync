-- name: CreateUser :one
INSERT INTO
    users (
        id,
        username,
        created_at,
        updated_at,
        sync_period,
        enabled_on_startup
    )
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING
    *;

-- name: UpdateUserSyncSettings :one
UPDATE users
SET
    sync_period = $2,
    enabled_on_startup = $3,
    updated_at = NOW()
WHERE
    id = $1
RETURNING
    *;

-- name: UpdateUserPassword :one
UPDATE users
SET
    password_hash = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING
    *;

-- name: UpdateUserTOTP :one
UPDATE users
SET
    totp_secret = $2,
    totp_enabled = $3,
    updated_at = NOW()
WHERE
    id = $1
RETURNING
    *;

-- name: EmptyUsers :exec
DELETE FROM users;

-- name: GetAllUsers :many
SELECT * FROM users;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;
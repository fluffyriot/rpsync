-- name: CreateUser :one
INSERT INTO
    users (
        id,
        username,
        created_at,
        updated_at,
        sync_period
    )
VALUES ($1, $2, $3, $4, $5)
RETURNING
    *;

-- name: UpdateUserSyncSettings :one
UPDATE users
SET
    sync_period = $2,
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

-- name: UpdateUserProfileImage :one
UPDATE users
SET
    profile_image = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING
    *;

-- name: UpdateUserUsername :one
UPDATE users
SET
    username = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING
    *;
-- name: UpdateUserLastSeenVersion :one
UPDATE users
SET
    last_seen_version = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING
    *;

-- name: UpdateUserIntroCompleted :one
UPDATE users
SET
    intro_completed = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING
    *;
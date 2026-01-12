-- name: CreateSource :one
INSERT INTO sources (id, created_at, updated_at, network, user_name, user_id, is_active)
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

-- name: GetUserActiveSourceByName :one
SELECT * FROM sources
where user_id = $1 and network = $2 and is_active = TRUE
LIMIT 1;

-- name: GetUserActiveSources :many
SELECT * FROM sources
where user_id = $1 and is_active = TRUE;
-- name: CreateRedirect :one
INSERT INTO
    redirects (
        id,
        source_id,
        from_path,
        to_path,
        created_at
    )
VALUES ($1, $2, $3, $4, $5)
RETURNING
    *;

-- name: GetRedirectsForSource :many
SELECT *
FROM redirects
WHERE
    source_id = $1
ORDER BY created_at DESC;

-- name: GetRedirectForPath :one
SELECT * FROM redirects WHERE source_id = $1 AND from_path = $2;

-- name: GetRedirectById :one
SELECT * FROM redirects WHERE id = $1;

-- name: DeleteRedirect :exec
DELETE FROM redirects WHERE id = $1;
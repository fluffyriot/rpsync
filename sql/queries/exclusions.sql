-- name: CreateExclusion :one
INSERT INTO
    exclusions (
        id,
        created_at,
        source_id,
        network_internal_id
    )
VALUES ($1, $2, $3, $4)
RETURNING
    *;

-- name: GetExclusionsForSource :many
SELECT *
FROM exclusions
WHERE
    source_id = $1
ORDER BY created_at DESC;

-- name: GetExclusionsForUser :many
SELECT e.id, e.created_at, e.source_id, e.network_internal_id, s.network, s.user_name
FROM exclusions e
    JOIN sources s ON e.source_id = s.id
WHERE
    s.user_id = $1
ORDER BY e.created_at DESC;

-- name: DeleteExclusion :exec
DELETE FROM exclusions WHERE id = $1;

-- name: DeletePostBySourceAndNetworkId :exec
DELETE FROM posts WHERE source_id = $1 AND network_internal_id = $2;
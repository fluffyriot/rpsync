-- name: CreateLog :one
INSERT INTO
    logs (
        id,
        created_at,
        source_id,
        target_id,
        message
    )
VALUES ($1, $2, $3, $4, $5)
RETURNING
    *;

-- name: GetRecentLogs :many
SELECT
    l.id,
    l.created_at,
    l.message,
    s.network AS source_network,
    s.user_name AS source_username,
    t.target_type AS target_type
FROM
    logs l
    LEFT JOIN sources s ON l.source_id = s.id
    LEFT JOIN targets t ON l.target_id = t.id
ORDER BY l.created_at DESC
LIMIT 20;

-- name: GetSyncErrorsCountLast30Days :one
SELECT COUNT(*)
FROM logs
WHERE
    created_at > NOW() - INTERVAL '30 days';
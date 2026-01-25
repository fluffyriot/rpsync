-- name: SyncReactions :one
INSERT INTO
    posts_reactions_history (
        id,
        synced_at,
        post_id,
        likes,
        reposts,
        views
    )
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (
    post_id,
    (CAST(synced_at AS DATE))
) DO
UPDATE
SET
    likes = EXCLUDED.likes,
    reposts = EXCLUDED.reposts,
    views = EXCLUDED.views,
    synced_at = EXCLUDED.synced_at
RETURNING
    *;

-- name: GetDailyStats :many
SELECT
    s.id,
    s.network,
    s.user_name,
    DATE (p.created_at) as date,
    SUM(prh.likes) as total_likes,
    SUM(prh.reposts) as total_reposts
FROM
    posts_reactions_history prh
    JOIN posts p ON prh.post_id = p.id
    JOIN sources s ON p.source_id = s.id
WHERE
    s.user_id = $1
    AND prh.synced_at = (
        SELECT MAX(synced_at)
        FROM posts_reactions_history
        WHERE
            post_id = prh.post_id
            AND DATE (synced_at) = DATE (prh.synced_at)
    )
    AND p.post_type <> 'repost'
GROUP BY
    s.id,
    s.network,
    s.user_name,
    DATE (prh.synced_at),
    DATE (p.created_at)
ORDER BY s.id, date ASC;

-- name: GetWeeklyStats :many
WITH
    LatestStats AS (
        SELECT prh.post_id, prh.likes, prh.reposts
        FROM
            posts_reactions_history prh
            JOIN (
                SELECT post_id, MAX(synced_at) as max_sync
                FROM posts_reactions_history
                GROUP BY
                    post_id
            ) latest ON prh.post_id = latest.post_id
            AND prh.synced_at = latest.max_sync
    )
SELECT
    s.id,
    s.network,
    s.user_name,
    TO_CHAR(p.created_at, 'IYYY-IW') as year_week,
    COALESCE(SUM(ls.likes), 0)::bigint as total_likes,
    COALESCE(SUM(ls.reposts), 0)::bigint as total_reposts
FROM
    posts p
    JOIN sources s ON p.source_id = s.id
    JOIN LatestStats ls ON p.id = ls.post_id
WHERE
    s.user_id = $1
    AND p.post_type <> 'repost'
GROUP BY
    s.id,
    s.network,
    s.user_name,
    year_week
ORDER BY year_week ASC;

-- name: DeleteOldStats :exec
DELETE from posts_reactions_history
where
    synced_at < now() - INTERVAL '14 days';
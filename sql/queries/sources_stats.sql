-- name: CreateSourceStat :one
INSERT INTO
    sources_stats (
        id,
        date,
        source_id,
        followers_count,
        following_count,
        posts_count,
        average_likes,
        average_reposts,
        average_views
    )
VALUES (
        $1,
        $2,
        $3,
        $4,
        $5,
        $6,
        $7,
        $8,
        $9
    )
RETURNING
    *;

-- name: UpdateSourceDayStats :one
UPDATE sources_stats
SET
    followers_count = $1,
    following_count = $2,
    posts_count = $3,
    average_likes = $4,
    average_reposts = $5,
    average_views = $6
WHERE
    source_id = $7
    AND date = $8
RETURNING
    *;

-- name: GetSourceStatsByDate :one
SELECT *
FROM sources_stats
WHERE
    source_id = $1
    AND date = $2
LIMIT 1;

-- name: GetSourceTotals :one
SELECT
    COUNT(DISTINCT p.id)::BIGINT AS total_posts,
    COALESCE(SUM(prh.likes), 0)::BIGINT AS total_likes,
    COALESCE(SUM(prh.reposts), 0)::BIGINT AS total_reposts,
    COALESCE(SUM(prh.views), 0)::BIGINT AS total_views
FROM posts p
    LEFT JOIN (
        SELECT DISTINCT
            ON (post_id) post_id, likes, reposts, views
        FROM posts_reactions_history
        ORDER BY post_id, synced_at DESC
    ) prh ON p.id = prh.post_id
WHERE
    p.source_id = $1;
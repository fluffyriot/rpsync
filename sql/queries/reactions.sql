-- name: SyncReactions :one
INSERT INTO posts_reactions_history (id, synced_at, post_id, likes, reposts, views)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetDailyStats :many
SELECT 
        s.id, s.network, s.user_name, DATE(p.created_at) as date,
        SUM(prh.likes) as total_likes,
        SUM(prh.reposts) as total_reposts,
        SUM(prh.views) as total_views
 
    FROM posts_reactions_history prh
        JOIN posts p ON prh.post_id = p.id
        JOIN sources s ON p.source_id = s.id

    WHERE s.user_id = $1
    AND prh.synced_at = (
        SELECT MAX(synced_at) 
        FROM posts_reactions_history 
        WHERE post_id = prh.post_id 
        AND DATE(synced_at) = DATE(prh.synced_at)
    )
    AND p.post_type <> 'repost'

GROUP BY s.id, s.network, s.user_name, DATE(prh.synced_at), DATE(p.created_at)
ORDER BY s.id, date ASC;

-- name: DeleteOldStats :exec
DELETE from posts_reactions_history
where synced_at < now() - INTERVAL '14 days';
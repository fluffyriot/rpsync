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
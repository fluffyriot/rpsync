-- name: AddPostToTarget :one
INSERT INTO posts_on_target (id, first_synced_at, post_id, target_id, target_post_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetPostsPreviouslySynced :many
SELECT * FROM posts_on_target
where target_id = $1;

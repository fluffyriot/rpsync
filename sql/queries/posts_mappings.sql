-- name: AddPostToTarget :one
INSERT INTO
    posts_on_target (
        id,
        first_synced_at,
        post_id,
        target_id,
        target_post_id
    )
VALUES ($1, $2, $3, $4, $5)
RETURNING
    *;

-- name: GetPostsPreviouslySynced :many
SELECT * FROM posts_on_target where target_id = $1;

-- name: GetPostsBySourceAndTarget :many
SELECT pot.*
FROM posts_on_target pot
    left join posts p on pot.post_id = p.id
where
    target_id = $1
    and p.source_id = $2;

-- name: DeletePostsOnTargetAndSource :exec
DELETE FROM posts_on_target pot USING posts p
WHERE
    pot.post_id = p.id
    AND pot.target_id = $1
    AND p.source_id = $2;

-- name: DeletePostOnTarget :exec
DELETE FROM posts_on_target WHERE id = $1;
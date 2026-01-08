-- name: CreatePost :one
INSERT INTO posts (id, created_at, last_synced_at, source_id, is_archived, network_internal_id, content)
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

-- name: GetPostByNetworkAndId :one
SELECT posts.* FROM posts
join sources on posts.source_id = sources.id
where network_internal_id = $1 and sources.network = $2;
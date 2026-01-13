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

-- name: GetAllPostsWithTheLatestInfoForUser :many
SELECT
    p.id,
    p.created_at,
    p.source_id,
    p.is_archived,
    p.network_internal_id,
    p.content,
    s.user_id AS user_id,
    r.synced_at AS reactions_synced_at,
    r.likes,
    r.reposts,
    r.views
FROM posts p
left join sources s
    ON p.source_id = s.id
LEFT JOIN posts_reactions_history r
    ON r.post_id = p.id
   AND r.synced_at = (
        SELECT MAX(prh.synced_at)
        FROM posts_reactions_history prh
        WHERE prh.post_id = p.id
   )
WHERE s.user_id = $1;
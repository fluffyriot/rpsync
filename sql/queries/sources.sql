-- name: GetUserActiveSourceByName :one
SELECT * FROM sources
where user_id = $1 and network = $2 and is_active = TRUE
LIMIT 1;
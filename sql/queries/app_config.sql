-- name: GetAppConfig :one
SELECT value FROM app_config WHERE key = $1 LIMIT 1;

-- name: SetAppConfig :exec
INSERT INTO
    app_config (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO
UPDATE
SET
    value = $2;
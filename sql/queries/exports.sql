-- name: CreateExport :one
INSERT INTO exports (id, created_at, completed_at, export_status, status_message, user_id, download_url, export_method)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING *;

-- name: ChangeExportStatusById :one
UPDATE exports
SET export_status = $2, status_message = $3, download_url = $4, completed_at = $5
WHERE id = $1
RETURNING *;

-- name: GetLast20ExportsByUserId :many
SELECT * FROM exports
where user_id = $1
ORDER BY created_at DESC
LIMIT 20;
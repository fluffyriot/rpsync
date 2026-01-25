-- +goose Up
ALTER TABLE logs
ADD COLUMN is_dismissed BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE logs DROP COLUMN is_dismissed;
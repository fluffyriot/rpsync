-- +goose Up
ALTER TABLE users
ADD COLUMN last_seen_version TEXT NOT NULL DEFAULT '0.18';

-- +goose Down
ALTER TABLE users DROP COLUMN last_seen_version;
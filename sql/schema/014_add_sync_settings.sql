-- +goose Up
ALTER TABLE users ADD COLUMN sync_period TEXT NOT NULL DEFAULT '30m';

ALTER TABLE users
ADD COLUMN enabled_on_startup BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down
ALTER TABLE users DROP COLUMN sync_period;

ALTER TABLE users DROP COLUMN enabled_on_startup;
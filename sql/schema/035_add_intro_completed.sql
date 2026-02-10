-- +goose Up
ALTER TABLE users
ADD COLUMN intro_completed BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE users DROP COLUMN intro_completed;
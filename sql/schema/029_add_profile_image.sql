-- +goose Up
ALTER TABLE users ADD COLUMN profile_image TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN profile_image;
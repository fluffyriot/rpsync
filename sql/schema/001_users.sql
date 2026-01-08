-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    username TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    sync_method TEXT NOT NULL,
    CONSTRAINT sync_check
        CHECK (sync_method IN ('CSV', 'Notion', 'None / Dev')),
    access_key TEXT,
    target_database_id TEXT
);


-- +goose Down
DROP TABLE users;
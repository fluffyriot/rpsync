-- +goose Up
CREATE TABLE posts (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    last_synced_at TIMESTAMP NOT NULL,
    source_id UUID NOT NULL,
    CONSTRAINT fk_source
        FOREIGN KEY (source_id)
        REFERENCES sources(id)
        ON DELETE CASCADE,
    is_archived BOOLEAN NOT NULL,
    network_internal_id TEXT NOT NULL,
    content TEXT
);


-- +goose Down
DROP TABLE posts;


-- +goose Up
CREATE TABLE redirects (
    id UUID PRIMARY KEY,
    source_id UUID NOT NULL,
    from_path TEXT NOT NULL,
    to_path TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT fk_source FOREIGN KEY (source_id) REFERENCES sources (id) ON DELETE CASCADE,
    CONSTRAINT unique_redirect UNIQUE (source_id, from_path)
);

-- +goose Down
DROP TABLE redirects;
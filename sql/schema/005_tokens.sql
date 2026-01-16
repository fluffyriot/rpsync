-- +goose Up
CREATE TABLE tokens (
    id UUID PRIMARY KEY,
    encrypted_access_token BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    profile_id TEXT,
    source_id UUID NOT NULL,
    CONSTRAINT fk_source FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE tokens;
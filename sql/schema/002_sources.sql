-- +goose Up
CREATE TABLE sources (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    network TEXT NOT NULL,
    CONSTRAINT network_check
        CHECK (network IN ('Instagram', 'Bluesky')),
    user_name TEXT NOT NULL,
    user_id UUID NOT NULL,
    CONSTRAINT fk_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL,
    hashed_token TEXT
);


-- +goose Down
DROP TABLE sources;
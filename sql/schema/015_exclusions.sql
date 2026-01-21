-- +goose Up
CREATE TABLE exclusions (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    source_id UUID NOT NULL,
    CONSTRAINT fk_source FOREIGN KEY (source_id) REFERENCES sources (id) ON DELETE CASCADE,
    network_internal_id TEXT NOT NULL,
    CONSTRAINT unique_exclusion UNIQUE (
        source_id,
        network_internal_id
    )
);

-- +goose Down
DROP TABLE exclusions;
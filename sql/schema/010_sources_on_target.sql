-- +goose Up
CREATE TABLE sources_on_target (
    id UUID PRIMARY KEY,

    source_id UUID NOT NULL,
    CONSTRAINT fk_posts FOREIGN KEY (source_id) REFERENCES sources(id),

    target_id UUID NOT NULL,
    CONSTRAINT fk_target FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,

    target_source_id TEXT NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE sources_on_target
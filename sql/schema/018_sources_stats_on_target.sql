-- +goose Up
CREATE TABLE sources_stats_on_target (
    id UUID PRIMARY KEY,
    synced_at TIMESTAMP NOT NULL,
    stat_id UUID NOT NULL,
    CONSTRAINT fk_sources_stats FOREIGN KEY (stat_id) REFERENCES sources_stats (id) ON DELETE CASCADE,
    target_id UUID NOT NULL,
    CONSTRAINT fk_target FOREIGN KEY (target_id) REFERENCES targets (id) ON DELETE CASCADE,
    target_record_id TEXT NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE sources_stats_on_target;
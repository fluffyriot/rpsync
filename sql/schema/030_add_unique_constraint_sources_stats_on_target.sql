-- +goose Up
ALTER TABLE sources_stats_on_target
ADD CONSTRAINT unique_sources_stats_on_target_stat_id_target_id UNIQUE (stat_id, target_id);

-- +goose Down
ALTER TABLE sources_stats_on_target
DROP CONSTRAINT unique_sources_stats_on_target_stat_id_target_id;
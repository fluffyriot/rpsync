-- +goose Up
-- Site Stats
ALTER TABLE analytics_site_stats_on_target
ALTER COLUMN stat_id
DROP NOT NULL;

ALTER TABLE analytics_site_stats_on_target
DROP CONSTRAINT fk_site_stat;

ALTER TABLE analytics_site_stats_on_target
ADD CONSTRAINT fk_site_stat FOREIGN KEY (stat_id) REFERENCES analytics_site_stats (id) ON DELETE SET NULL;

-- Page Stats
ALTER TABLE analytics_page_stats_on_target
ALTER COLUMN stat_id
DROP NOT NULL;

ALTER TABLE analytics_page_stats_on_target
DROP CONSTRAINT fk_page_stat;

ALTER TABLE analytics_page_stats_on_target
ADD CONSTRAINT fk_page_stat FOREIGN KEY (stat_id) REFERENCES analytics_page_stats (id) ON DELETE SET NULL;

-- +goose Down
-- Site Stats
ALTER TABLE analytics_site_stats_on_target
DROP CONSTRAINT fk_site_stat;

ALTER TABLE analytics_site_stats_on_target
ADD CONSTRAINT fk_site_stat FOREIGN KEY (stat_id) REFERENCES analytics_site_stats (id) ON DELETE CASCADE;

ALTER TABLE analytics_site_stats_on_target
ALTER COLUMN stat_id
SET NOT NULL;

-- Page Stats
ALTER TABLE analytics_page_stats_on_target
DROP CONSTRAINT fk_page_stat;

ALTER TABLE analytics_page_stats_on_target
ADD CONSTRAINT fk_page_stat FOREIGN KEY (stat_id) REFERENCES analytics_page_stats (id) ON DELETE CASCADE;

ALTER TABLE analytics_page_stats_on_target
ALTER COLUMN stat_id
SET NOT NULL;
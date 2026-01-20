-- +goose Up
ALTER TABLE sources DROP CONSTRAINT network_check;
ALTER TABLE sources ADD CONSTRAINT network_check CHECK (network IN ('Instagram', 'Bluesky', 'Murrtube', 'BadPups', 'TikTok', 'Mastodon', 'Reddit', 'Telegram', 'Google Analytics'));

CREATE TABLE analytics_site_stats (
    id UUID PRIMARY KEY,
    date TIMESTAMP NOT NULL,
    visitors INT NOT NULL DEFAULT 0,
    avg_session_duration FLOAT NOT NULL DEFAULT 0.0,
    source_id UUID NOT NULL,
    CONSTRAINT fk_source_site 
        FOREIGN KEY (source_id) 
        REFERENCES sources(id) 
        ON DELETE CASCADE,
    CONSTRAINT unique_site_stat UNIQUE (source_id, date)
);

CREATE TABLE analytics_page_stats (
    id UUID PRIMARY KEY,
    date TIMESTAMP NOT NULL,
    url_path TEXT NOT NULL,
    views INT NOT NULL DEFAULT 0,
    source_id UUID NOT NULL,
    CONSTRAINT fk_source_page 
        FOREIGN KEY (source_id) 
        REFERENCES sources(id) 
        ON DELETE CASCADE,
    CONSTRAINT unique_page_stat UNIQUE (source_id, date, url_path)
);

CREATE TABLE analytics_site_stats_on_target (
    id UUID PRIMARY KEY,
    synced_at TIMESTAMP NOT NULL,
    stat_id UUID NOT NULL,
    CONSTRAINT fk_site_stat FOREIGN KEY (stat_id) REFERENCES analytics_site_stats(id) ON DELETE CASCADE,
    target_id UUID NOT NULL,
    CONSTRAINT fk_target_site FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    target_record_id TEXT NOT NULL,
    CONSTRAINT unique_site_stat_target UNIQUE (stat_id, target_id)
);

CREATE TABLE analytics_page_stats_on_target (
    id UUID PRIMARY KEY,
    synced_at TIMESTAMP NOT NULL,
    stat_id UUID NOT NULL,
    CONSTRAINT fk_page_stat FOREIGN KEY (stat_id) REFERENCES analytics_page_stats(id) ON DELETE CASCADE,
    target_id UUID NOT NULL,
    CONSTRAINT fk_target_page FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    target_record_id TEXT NOT NULL,
    CONSTRAINT unique_page_stat_target UNIQUE (stat_id, target_id)
);

-- +goose Down
ALTER TABLE sources DROP CONSTRAINT network_check;
ALTER TABLE sources ADD CONSTRAINT network_check CHECK (network IN ('Instagram', 'Bluesky', 'Murrtube', 'BadPups', 'TikTok', 'Mastodon', 'Reddit', 'Telegram'));

DROP TABLE analytics_page_stats_on_target;
DROP TABLE analytics_site_stats_on_target;
DROP TABLE analytics_page_stats;
DROP TABLE analytics_site_stats;

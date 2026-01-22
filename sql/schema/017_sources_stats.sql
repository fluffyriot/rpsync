-- +goose Up
CREATE TABLE sources_stats (
    id UUID PRIMARY KEY,
    date TIMESTAMP NOT NULL,
    source_id UUID NOT NULL,
    CONSTRAINT fk_source FOREIGN KEY (source_id) REFERENCES sources (id) ON DELETE CASCADE,
    followers_count INT,
    following_count INT,
    posts_count INT,
    average_likes FLOAT,
    average_reposts FLOAT,
    average_views FLOAT,
    CONSTRAINT unique_source_stat UNIQUE (source_id, date)
);

-- +goose Down
DROP TABLE sources_stats;
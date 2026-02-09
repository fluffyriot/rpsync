-- +goose Up
ALTER TABLE analytics_page_stats ALTER COLUMN views TYPE BIGINT;

ALTER TABLE analytics_site_stats ALTER COLUMN visitors TYPE BIGINT;

ALTER TABLE posts_reactions_history ALTER COLUMN likes TYPE BIGINT;

ALTER TABLE posts_reactions_history ALTER COLUMN reposts TYPE BIGINT;

ALTER TABLE posts_reactions_history ALTER COLUMN views TYPE BIGINT;

ALTER TABLE sources_stats ALTER COLUMN followers_count TYPE BIGINT;

ALTER TABLE sources_stats ALTER COLUMN following_count TYPE BIGINT;

ALTER TABLE sources_stats ALTER COLUMN posts_count TYPE BIGINT;

-- +goose Down
ALTER TABLE analytics_page_stats ALTER COLUMN views TYPE INT;

ALTER TABLE analytics_site_stats ALTER COLUMN visitors TYPE INT;

ALTER TABLE posts_reactions_history ALTER COLUMN likes TYPE INT;

ALTER TABLE posts_reactions_history ALTER COLUMN reposts TYPE INT;

ALTER TABLE posts_reactions_history ALTER COLUMN views TYPE INT;

ALTER TABLE sources_stats ALTER COLUMN followers_count TYPE INT;

ALTER TABLE sources_stats ALTER COLUMN following_count TYPE INT;

ALTER TABLE sources_stats ALTER COLUMN posts_count TYPE INT;
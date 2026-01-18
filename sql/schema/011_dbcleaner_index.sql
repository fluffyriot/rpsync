-- +goose Up
CREATE INDEX IF NOT EXISTS idx_posts_reactions_history_synced_at
ON posts_reactions_history (synced_at);

-- +goose Down
DROP INDEX IF EXISTS idx_posts_reactions_history_synced_at;
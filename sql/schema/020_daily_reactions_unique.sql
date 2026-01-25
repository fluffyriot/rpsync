-- +goose Up
DELETE FROM posts_reactions_history
WHERE
    id NOT IN (
        SELECT DISTINCT
            ON (post_id, synced_at::DATE) id
        FROM posts_reactions_history
        ORDER BY
            post_id,
            synced_at::DATE,
            synced_at DESC
    );

CREATE UNIQUE INDEX posts_reactions_history_post_date_idx ON posts_reactions_history (post_id, (synced_at::DATE));

-- +goose Down
DROP INDEX IF EXISTS posts_reactions_history_post_date_idx;
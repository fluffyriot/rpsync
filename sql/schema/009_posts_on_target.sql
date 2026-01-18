-- +goose Up
CREATE TABLE posts_on_target (
    id UUID PRIMARY KEY,

    first_synced_at TIMESTAMP NOT NULL,

    post_id UUID NOT NULL,
    CONSTRAINT fk_posts FOREIGN KEY (post_id) REFERENCES posts(id),

    target_id UUID NOT NULL,
    CONSTRAINT fk_target FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,

    target_post_id TEXT NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE posts_on_target
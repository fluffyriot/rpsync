-- +goose Up
CREATE TABLE posts_reactions_history (
    id UUID PRIMARY KEY,
    synced_at TIMESTAMP NOT NULL,
    post_id UUID NOT NULL,
    CONSTRAINT fk_post
        FOREIGN KEY (post_id)
        REFERENCES posts(id)
        ON DELETE CASCADE,
    UNIQUE (post_id, synced_at),
    likes INTEGER,
    reposts INTEGER,
    views INTEGER
);


-- +goose Down
DROP TABLE posts_reactions_history;
-- +goose Up
ALTER TABLE posts_on_target ALTER COLUMN post_id DROP NOT NULL;

ALTER TABLE posts_on_target DROP CONSTRAINT fk_posts;

ALTER TABLE posts_on_target
ADD CONSTRAINT fk_posts FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE posts_on_target DROP CONSTRAINT fk_posts;

ALTER TABLE posts_on_target
ADD CONSTRAINT fk_posts FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE;

ALTER TABLE posts_on_target ALTER COLUMN post_id SET NOT NULL;
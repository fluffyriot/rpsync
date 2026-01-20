-- +goose Up
CREATE TABLE logs (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    source_id UUID,
    target_id UUID,
    message TEXT NOT NULL,
    CONSTRAINT fk_source_logs FOREIGN KEY (source_id) REFERENCES sources (id) ON DELETE CASCADE,
    CONSTRAINT fk_target_logs FOREIGN KEY (target_id) REFERENCES targets (id) ON DELETE CASCADE,
    CONSTRAINT source_or_target_check CHECK (
        source_id IS NOT NULL
        OR target_id IS NOT NULL
    )
);

-- +goose Down
DROP TABLE logs;
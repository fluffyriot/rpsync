-- +goose Up
CREATE TABLE targets (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    target_type TEXT NOT NULL,
    CONSTRAINT type_check
        CHECK (target_type IN ('NocoDB', 'Notion', 'CSV', 'None')),
    user_id UUID NOT NULL,
    CONSTRAINT fk_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,
    db_id TEXT,
    is_active BOOLEAN NOT NULL,
    sync_frequency TEXT NOT NULL,
    sync_status TEXT NOT NULL,
    status_reason TEXT,
    last_synced TIMESTAMP
);

-- +goose Down
DROP TABLE targets;
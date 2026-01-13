-- +goose Up
CREATE TABLE exports (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP NOT NULL,
    export_status TEXT NOT NULL,
    status_message TEXT,
    user_id UUID NOT NULL,
    CONSTRAINT fk_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,
    download_url TEXT,
    export_method TEXT NOT NULL
);


-- +goose Down
DROP TABLE exports;


-- +goose Up
CREATE TABLE app_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO
    app_config (key, value)
VALUES (
        'allow_new_user_creation',
        'true'
    );

INSERT INTO
    app_config (key, value)
VALUES (
        'enable_worker_on_startup',
        'true'
    );

ALTER TABLE users DROP COLUMN enabled_on_startup;

-- +goose Down
ALTER TABLE users
ADD COLUMN enabled_on_startup BOOLEAN NOT NULL DEFAULT TRUE;

DROP TABLE app_config;
-- +goose Up
ALTER TABLE users ADD COLUMN password_hash VARCHAR(255);

ALTER TABLE users ADD COLUMN totp_secret VARCHAR(255);

ALTER TABLE users ADD COLUMN totp_enabled BOOLEAN DEFAULT FALSE;

CREATE TABLE webauthn_credentials (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL,
    public_key BYTEA NOT NULL,
    attestation_type VARCHAR(255) NOT NULL,
    aaguid UUID,
    sign_count BIGINT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (credential_id)
);

CREATE INDEX idx_webauthn_user_id ON webauthn_credentials (user_id);

-- +goose Down
ALTER TABLE users DROP COLUMN password_hash;

ALTER TABLE users DROP COLUMN totp_enabled;

ALTER TABLE users DROP COLUMN totp_secret;

DROP TABLE webauthn_credentials;
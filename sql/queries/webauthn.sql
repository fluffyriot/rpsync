-- name: CreateWebAuthnCredential :one
INSERT INTO
    webauthn_credentials (
        id,
        user_id,
        credential_id,
        public_key,
        attestation_type,
        aaguid,
        sign_count
    )
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    *;

-- name: GetWebAuthnCredentialsByUserID :many
SELECT * FROM webauthn_credentials WHERE user_id = $1;

-- name: GetWebAuthnCredentialByCredentialID :one
SELECT * FROM webauthn_credentials WHERE credential_id = $1;

-- name: UpdateWebAuthnCredentialSignCount :exec
UPDATE webauthn_credentials
SET
    sign_count = $2,
    updated_at = NOW()
WHERE
    id = $1;
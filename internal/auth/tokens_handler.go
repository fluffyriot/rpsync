package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

func encrypt(plaintext, key []byte) (ciphertext, nonce []byte, err error) {
	if len(key) != 32 {
		return nil, nil, errors.New("encryption key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return
}

func decrypt(ciphertext, nonce, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}

func insertToken(
	ctx context.Context,
	db *database.Queries,
	accessToken, pid string,
	encryptionKey []byte,
	params database.CreateTokenParams,
) error {

	payload, err := normalizeAccessTokenPayload(accessToken)
	if err != nil {
		return err
	}

	ciphertext, nonce, err := encrypt(payload, encryptionKey)
	if err != nil {
		return err
	}

	params.ID = uuid.New()
	params.EncryptedAccessToken = ciphertext
	params.Nonce = nonce
	params.CreatedAt = time.Now()
	params.UpdatedAt = time.Now()
	params.ProfileID = sql.NullString{String: pid, Valid: true}

	_, err = db.CreateToken(ctx, params)
	return err
}

func InsertSourceToken(
	ctx context.Context,
	db *database.Queries,
	sid uuid.UUID,
	accessToken, pid string,
	encryptionKey []byte,
) error {

	return insertToken(ctx, db, accessToken, pid, encryptionKey,
		database.CreateTokenParams{
			SourceID: uuid.NullUUID{UUID: sid, Valid: true},
		},
	)
}

func InsertTargetToken(
	ctx context.Context,
	db *database.Queries,
	tid uuid.UUID,
	accessToken, pid string,
	encryptionKey []byte,
) error {

	return insertToken(ctx, db, accessToken, pid, encryptionKey,
		database.CreateTokenParams{
			TargetID: uuid.NullUUID{UUID: tid, Valid: true},
		},
	)
}

func decryptToken(
	dbToken database.Token,
	encryptionKey []byte,
) (string, error) {

	plaintext, err := decrypt(
		dbToken.EncryptedAccessToken,
		dbToken.Nonce,
		encryptionKey,
	)
	if err != nil {
		return "", err
	}

	var tr TokenResponse
	if err := json.Unmarshal(plaintext, &tr); err != nil {
		return "", err
	}

	return tr.AccessToken, nil
}

func GetSourceToken(
	ctx context.Context,
	db *database.Queries,
	encryptionKey []byte,
	sid uuid.UUID,
) (accessToken, profileID string, tokenID uuid.UUID, err error) {

	dbToken, err := db.GetTokenBySource(ctx, uuid.NullUUID{UUID: sid, Valid: true})
	if err != nil {
		return "", "", uuid.UUID{}, err
	}

	accessToken, err = decryptToken(dbToken, encryptionKey)
	if err != nil {
		return "", "", uuid.UUID{}, err
	}

	return accessToken, dbToken.ProfileID.String, dbToken.ID, nil
}

func GetTargetToken(
	ctx context.Context,
	db *database.Queries,
	encryptionKey []byte,
	tid uuid.UUID,
) (accessToken, profileID string, tokenID uuid.UUID, err error) {

	dbToken, err := db.GetTokenByTarget(ctx, uuid.NullUUID{UUID: tid, Valid: true})
	if err != nil {
		return "", "", uuid.UUID{}, err
	}

	accessToken, err = decryptToken(dbToken, encryptionKey)
	if err != nil {
		return "", "", uuid.UUID{}, err
	}

	return accessToken, dbToken.ProfileID.String, dbToken.ID, nil
}

func normalizeAccessTokenPayload(input string) ([]byte, error) {
	if input == "" {
		return nil, errors.New("access token is empty")
	}

	var tr TokenResponse
	if err := json.Unmarshal([]byte(input), &tr); err == nil && tr.AccessToken != "" {
		return json.Marshal(tr)
	}

	return json.Marshal(TokenResponse{
		AccessToken: input,
	})
}

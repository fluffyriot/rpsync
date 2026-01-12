package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
	"github.com/google/uuid"
)

type Token struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Cipher    []byte
	Nonce     []byte
	KeyID     int16
	CreatedAt time.Time
	UpdatedAt time.Time
}

func encrypt(plaintext []byte, key []byte) (ciphertext, nonce []byte, err error) {
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

func InsertToken(
	dbQueries *database.Queries,
	sid uuid.UUID,
	accessToken string,
	encryptionKey []byte,
) error {

	ciphertext, nonce, err := encrypt([]byte(accessToken), encryptionKey)
	if err != nil {
		return err
	}

	_, err = dbQueries.CreateToken(context.Background(), database.CreateTokenParams{
		ID:                   uuid.New(),
		EncryptedAccessToken: ciphertext,
		Nonce:                nonce,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		SourceID:             sid,
	})

	return err
}

func GetToken(
	ctx context.Context,
	dbQueries *database.Queries,
	encryptionKey []byte,
	sid uuid.UUID,
) (string, error) {
	var (
		ciphertext []byte
		nonce      []byte
	)

	dbToken, err := dbQueries.GetTokenBySource(context.Background(), sid)
	if err != nil {
		return "", err
	}

	ciphertext = dbToken.EncryptedAccessToken
	nonce = dbToken.Nonce

	plaintext, err := decrypt(ciphertext, nonce, encryptionKey)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// SPDX-License-Identifier: AGPL-3.0-only
package authhelp

import (
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/go-webauthn/webauthn/webauthn"
)

type WebAuthnUser struct {
	User        database.User
	Credentials []webauthn.Credential
}

func (u *WebAuthnUser) WebAuthnID() []byte {
	return u.User.ID[:]
}

func (u *WebAuthnUser) WebAuthnName() string {
	return u.User.Username
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.User.Username
}

func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

func ConvertCredentials(dbCreds []database.WebauthnCredential) []webauthn.Credential {
	wc := make([]webauthn.Credential, len(dbCreds))
	for i, c := range dbCreds {
		wc[i] = webauthn.Credential{
			ID:              c.CredentialID,
			PublicKey:       c.PublicKey,
			AttestationType: c.AttestationType,
			Authenticator: webauthn.Authenticator{
				AAGUID:    c.Aaguid.UUID[:],
				SignCount: uint32(c.SignCount.Int64),
			},
		}
	}
	return wc
}

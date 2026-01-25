package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

func (h *Handler) getWebAuthn(c *gin.Context) (*webauthn.WebAuthn, error) {
	host := c.Request.Host
	rpid := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		rpid = h
	}

	proto := "https"
	if c.Request.TLS == nil && c.Request.Header.Get("X-Forwarded-Proto") != "https" {
		proto = "http"
	}

	origin := fmt.Sprintf("%s://%s", proto, host)

	wConfig := &webauthn.Config{
		RPDisplayName: "RPSync",
		RPID:          rpid,
		RPOrigins:     []string{origin},
	}

	return webauthn.New(wConfig)
}

func (h *Handler) PasskeyRegisterBegin(c *gin.Context) {
	wa, err := h.getWebAuthn(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to init WebAuthn: " + err.Error()})
		return
	}

	session := sessions.Default(c)
	userIDStr := session.Get("user_id")
	if userIDStr == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, _ := uuid.Parse(userIDStr.(string))

	users, _ := h.DB.GetAllUsers(c.Request.Context())
	var user database.User
	for _, u := range users {
		if u.ID == userID {
			user = u
			break
		}
	}

	dbCreds, _ := h.DB.GetWebAuthnCredentialsByUserID(c.Request.Context(), userID)

	wUser := &authhelp.WebAuthnUser{
		User:        user,
		Credentials: authhelp.ConvertCredentials(dbCreds),
	}

	options, sessionData, err := wa.BeginRegistration(wUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	sessionDataBytes, _ := json.Marshal(sessionData)
	session.Set("webauthn_session", string(sessionDataBytes))
	session.Save()

	c.JSON(http.StatusOK, options)
}

func (h *Handler) PasskeyRegisterFinish(c *gin.Context) {
	wa, err := h.getWebAuthn(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to init WebAuthn: " + err.Error()})
		return
	}
	session := sessions.Default(c)
	userIDStr := session.Get("user_id")
	if userIDStr == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, _ := uuid.Parse(userIDStr.(string))

	sessionDataStr := session.Get("webauthn_session")
	if sessionDataStr == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No session data"})
		return
	}

	var sessionData webauthn.SessionData
	json.Unmarshal([]byte(sessionDataStr.(string)), &sessionData)

	users, _ := h.DB.GetAllUsers(c.Request.Context())
	var user database.User
	for _, u := range users {
		if u.ID == userID {
			user = u
			break
		}
	}

	wUser := &authhelp.WebAuthnUser{
		User: user,
	}

	credential, err := wa.FinishRegistration(wUser, sessionData, c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var aaguid uuid.UUID
	copy(aaguid[:], credential.Authenticator.AAGUID)

	_, err = h.DB.CreateWebAuthnCredential(c.Request.Context(), database.CreateWebAuthnCredentialParams{
		ID:              uuid.New(),
		UserID:          userID,
		CredentialID:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		Aaguid:          uuid.NullUUID{UUID: aaguid, Valid: true},
		SignCount:       sql.NullInt64{Int64: int64(credential.Authenticator.SignCount), Valid: true},
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save credential"})
		return
	}

	session.Delete("webauthn_session")
	session.Save()

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) PasskeyLoginBegin(c *gin.Context) {
	wa, err := h.getWebAuthn(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to init WebAuthn: " + err.Error()})
		return
	}

	username := c.Query("username")
	var wUser *authhelp.WebAuthnUser

	if username != "" {
		users, _ := h.DB.GetAllUsers(c.Request.Context())
		var user database.User
		var found bool
		for _, u := range users {
			if u.Username == username {
				user = u
				found = true
				break
			}
		}

		if found {
			dbCreds, _ := h.DB.GetWebAuthnCredentialsByUserID(c.Request.Context(), user.ID)
			wUser = &authhelp.WebAuthnUser{
				User:        user,
				Credentials: authhelp.ConvertCredentials(dbCreds),
			}
		}
	}

	if wUser == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User not found"})
		return
	}

	options, sessionData, err := wa.BeginLogin(wUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	session := sessions.Default(c)
	sessionDataBytes, _ := json.Marshal(sessionData)
	session.Set("webauthn_session", string(sessionDataBytes))
	session.Set("webauthn_user_id", wUser.User.ID.String())
	session.Save()

	c.JSON(http.StatusOK, options)
}

func (h *Handler) PasskeyLoginFinish(c *gin.Context) {
	wa, err := h.getWebAuthn(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to init WebAuthn: " + err.Error()})
		return
	}
	session := sessions.Default(c)
	sessionDataStr := session.Get("webauthn_session")
	userIDStr := session.Get("webauthn_user_id")

	if sessionDataStr == nil || userIDStr == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No session data"})
		return
	}

	var sessionData webauthn.SessionData
	json.Unmarshal([]byte(sessionDataStr.(string)), &sessionData)

	userID, _ := uuid.Parse(userIDStr.(string))

	users, _ := h.DB.GetAllUsers(c.Request.Context())
	var user database.User
	for _, u := range users {
		if u.ID == userID {
			user = u
			break
		}
	}

	dbCreds, _ := h.DB.GetWebAuthnCredentialsByUserID(c.Request.Context(), userID)
	wUser := &authhelp.WebAuthnUser{
		User:        user,
		Credentials: authhelp.ConvertCredentials(dbCreds),
	}

	credential, err := wa.FinishLogin(wUser, sessionData, c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var dbID uuid.UUID
	for _, dc := range dbCreds {
		if string(dc.CredentialID) == string(credential.ID) {
			dbID = dc.ID
			break
		}
	}

	h.DB.UpdateWebAuthnCredentialSignCount(c.Request.Context(), database.UpdateWebAuthnCredentialSignCountParams{
		ID:        dbID,
		SignCount: sql.NullInt64{Int64: int64(credential.Authenticator.SignCount), Valid: true},
	})

	session.Delete("webauthn_session")
	session.Delete("webauthn_user_id")
	session.Set("user_id", userID.String())
	session.Save()

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

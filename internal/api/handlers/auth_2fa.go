package handlers

import (
	"database/sql"
	"net/http"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) TwoFASetupViewHandler(c *gin.Context) {
	session := sessions.Default(c)
	userIDStr := session.Get("user_id")
	if userIDStr == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	userID, _ := uuid.Parse(userIDStr.(string))

	username := "User"
	users, _ := h.DB.GetAllUsers(c.Request.Context())
	for _, u := range users {
		if u.ID == userID {
			username = u.Username
			break
		}
	}

	key, err := authhelp.GenerateTOTP(username)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{"error": "Failed to generate 2FA key"}))
		return
	}

	qrCode, err := authhelp.GenerateQRCode(key)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{"error": "Failed to generate QR code"}))
		return
	}

	c.HTML(http.StatusOK, "setup-2fa.html", h.CommonData(c, gin.H{
		"title":        "Setup 2FA",
		"qr_code":      qrCode,
		"secret":       key.Secret(),
		"is_auth_page": true,
	}))
}

func (h *Handler) TwoFASetupSubmitHandler(c *gin.Context) {
	session := sessions.Default(c)
	userIDStr := session.Get("user_id")
	if userIDStr == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	userID, _ := uuid.Parse(userIDStr.(string))

	secret := c.PostForm("secret")
	code := c.PostForm("code")

	if !authhelp.ValidateTOTP(code, secret) {
		c.HTML(http.StatusOK, "error.html", h.CommonData(c, gin.H{"error": "Invalid code. Please try again.", "title": "Setup Failed", "is_auth_page": true}))
		return
	}

	_, err := h.DB.UpdateUserTOTP(c.Request.Context(), database.UpdateUserTOTPParams{
		ID:          userID,
		TotpSecret:  sql.NullString{String: secret, Valid: true},
		TotpEnabled: sql.NullBool{Bool: true, Valid: true},
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{"error": "Database error: " + err.Error(), "is_auth_page": true}))
		return
	}

	c.Redirect(http.StatusFound, "/settings/sync")
}

func (h *Handler) TwoFALoginViewHandler(c *gin.Context) {
	session := sessions.Default(c)
	pendingID := session.Get("2fa_pending_user_id")
	if pendingID == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "login-2fa.html", h.CommonData(c, gin.H{
		"title":        "Two-Factor Authentication",
		"is_auth_page": true,
	}))
}

func (h *Handler) TwoFALoginSubmitHandler(c *gin.Context) {
	session := sessions.Default(c)
	pendingID := session.Get("2fa_pending_user_id")
	if pendingID == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	userID, _ := uuid.Parse(pendingID.(string))
	code := c.PostForm("code")

	users, _ := h.DB.GetAllUsers(c.Request.Context())
	var user *database.User
	for _, u := range users {
		if u.ID == userID {
			user = &u
			break
		}
	}

	if user == nil || !user.TotpEnabled.Bool || !user.TotpSecret.Valid {
		session.Delete("2fa_pending_user_id")
		session.Save()
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if !authhelp.ValidateTOTP(code, user.TotpSecret.String) {
		c.HTML(http.StatusOK, "login-2fa.html", h.CommonData(c, gin.H{
			"title":        "Two-Factor Authentication",
			"error":        "Invalid code",
			"is_auth_page": true,
		}))
		return
	}

	session.Delete("2fa_pending_user_id")
	session.Set("user_id", userID.String())
	session.Save()
	c.Redirect(http.StatusFound, "/")
}

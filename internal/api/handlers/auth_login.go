// SPDX-License-Identifier: AGPL-3.0-only
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

func (h *Handler) LoginViewHandler(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	users, err := h.DB.GetAllUsers(c.Request.Context())
	if err == nil && len(users) == 1 {
		u := users[0]
		if !u.PasswordHash.Valid || u.PasswordHash.String == "" {
			session.Set("user_id", u.ID.String())
			session.Save()
			c.Redirect(http.StatusFound, "/setup/password")
			return
		}
	}

	allowCreateUserConfig, _ := h.DB.GetAppConfig(c.Request.Context(), "allow_new_user_creation")
	allowCreateUser := allowCreateUserConfig == "true"

	c.HTML(http.StatusOK, "login.html", h.CommonData(c, gin.H{
		"title":              "Login",
		"is_auth_page":       true,
		"allow_registration": allowCreateUser,
	}))
}

func (h *Handler) LoginSubmitHandler(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == "" {
		c.HTML(http.StatusOK, "login.html", h.CommonData(c, gin.H{"error": "Username is required", "title": "Login", "is_auth_page": true}))
		return
	}

	users, err := h.DB.GetAllUsers(c.Request.Context())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", h.CommonData(c, gin.H{"error": "Database error", "title": "Login", "is_auth_page": true}))
		return
	}

	var foundUser *database.User
	for _, u := range users {
		if u.Username == username {
			foundUser = &u
			break
		}
	}

	if foundUser == nil {
		c.HTML(http.StatusUnauthorized, "login.html", h.CommonData(c, gin.H{"error": "Invalid credentials", "title": "Login", "is_auth_page": true}))
		return
	}

	if !foundUser.PasswordHash.Valid || foundUser.PasswordHash.String == "" {

		session := sessions.Default(c)
		session.Set("user_id", foundUser.ID.String())
		session.Save()

		c.Redirect(http.StatusFound, "/setup/password")
		return
	}

	if !authhelp.CheckPasswordHash(foundUser.PasswordHash.String, password) {
		c.HTML(http.StatusUnauthorized, "login.html", h.CommonData(c, gin.H{"error": "Invalid credentials", "title": "Login", "is_auth_page": true}))
		return
	}

	session := sessions.Default(c)

	if foundUser.TotpEnabled.Valid && foundUser.TotpEnabled.Bool {
		session.Set("2fa_pending_user_id", foundUser.ID.String())
		session.Save()
		c.Redirect(http.StatusFound, "/login/2fa")
		return
	}

	session.Set("user_id", foundUser.ID.String())
	session.Set("username", foundUser.Username)
	hasAvatar := false
	if foundUser.ProfileImage.Valid && foundUser.ProfileImage.String != "" {
		hasAvatar = true
	}
	session.Set("has_avatar", hasAvatar)

	session.Save()
	c.Redirect(http.StatusFound, "/")
}

func (h *Handler) PasswordSetupViewHandler(c *gin.Context) {
	session := sessions.Default(c)
	userIDStr := session.Get("user_id")
	if userIDStr == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	userID, _ := uuid.Parse(userIDStr.(string))
	users, _ := h.DB.GetAllUsers(c.Request.Context())
	var username string
	for _, u := range users {
		if u.ID == userID {
			username = u.Username
			break
		}
	}

	c.HTML(http.StatusOK, "setup-password.html", h.CommonData(c, gin.H{
		"title":        "Setup Password",
		"username":     username,
		"is_auth_page": true,
	}))
}

func (h *Handler) PasswordSetupSubmitHandler(c *gin.Context) {
	session := sessions.Default(c)
	userIDStr := session.Get("user_id")
	if userIDStr == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	userID, _ := uuid.Parse(userIDStr.(string))

	password := c.PostForm("password")
	confirm := c.PostForm("confirm_password")

	if err := authhelp.ValidatePasswordStrength(password); err != nil {
		c.HTML(http.StatusOK, "setup-password.html", h.CommonData(c, gin.H{"error": err.Error(), "title": "Setup Password", "is_auth_page": true}))
		return
	}

	if password != confirm {
		c.HTML(http.StatusOK, "setup-password.html", h.CommonData(c, gin.H{"error": "Passwords do not match", "title": "Setup Password", "is_auth_page": true}))
		return
	}

	hash, err := authhelp.HashPassword(password)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "setup-password.html", h.CommonData(c, gin.H{"error": "Encryption error", "title": "Setup Password", "is_auth_page": true}))
		return
	}

	_, err = h.DB.UpdateUserPassword(c.Request.Context(), database.UpdateUserPasswordParams{
		ID:           userID,
		PasswordHash: sql.NullString{String: hash, Valid: true},
	})

	if err != nil {
		c.HTML(http.StatusInternalServerError, "setup-password.html", h.CommonData(c, gin.H{"error": "Failed to save password: " + err.Error(), "title": "Setup Password", "is_auth_page": true}))
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func (h *Handler) LogoutHandler(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}

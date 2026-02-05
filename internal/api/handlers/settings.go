package handlers

import (
	"net/http"
	"time"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (h *Handler) UserSetupViewHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "user-setup.html", h.CommonData(c, gin.H{
		"title": "Welcome - Setup Admin User",
	}))
}

func (h *Handler) UserSetupHandler(c *gin.Context) {
	allowCreateUserConfig, _ := h.DB.GetAppConfig(c.Request.Context(), "allow_new_user_creation")
	if allowCreateUserConfig != "true" {
		c.HTML(http.StatusForbidden, "error.html", h.CommonData(c, gin.H{
			"error": "New user creation is disabled by administrator.",
			"title": "Registration Disabled",
		}))
		return
	}

	username := c.PostForm("username")
	if username == "" {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(c, gin.H{
			"error": "username is required",
			"title": "Error",
		}))
		return
	}

	_, userID, err := config.CreateUserFromForm(h.DB, username)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	session := sessions.Default(c)
	session.Set("user_id", userID)
	session.Set("username", username)
	session.Set("has_avatar", false)
	session.Save()

	c.Redirect(http.StatusSeeOther, "/setup/password")
}

func (h *Handler) SyncSettingsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	isSecure := c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https"
	if h.Config.ClientIP == "localhost" {
		isSecure = true
	}

	isWebauthnConfigured := h.Config.WebAuthn != nil
	isPasskeySupported := isSecure && isWebauthnConfigured

	allowCreateUserConfig, _ := h.DB.GetAppConfig(c.Request.Context(), "allow_new_user_creation")
	allowCreateUser := allowCreateUserConfig == "true"

	enableWorkerConfig, _ := h.DB.GetAppConfig(c.Request.Context(), "enable_worker_on_startup")
	enableWorker := enableWorkerConfig == "true"

	c.HTML(http.StatusOK, "sync-settings.html", h.CommonData(c, gin.H{
		"sync_period":              user.SyncPeriod,
		"allow_new_user_creation":  allowCreateUser,
		"enable_worker_on_startup": enableWorker,
		"worker_running":           h.Worker.IsActive(),
		"title":                    "Sync Settings",
		"is_2fa_enabled":           user.TotpEnabled.Bool,
		"is_webauthn_configured":   isWebauthnConfigured,
		"is_secure_context":        isSecure,
		"is_passkey_supported":     isPasskeySupported,
	}))
}

func (h *Handler) UpdateSyncSettingsHandler(c *gin.Context) {
	periodStr := c.PostForm("sync_period")

	_, err := time.ParseDuration(periodStr)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(c, gin.H{
			"error": "Invalid duration format",
			"title": "Error",
		}))
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	_, err = h.DB.UpdateUserSyncSettings(c, database.UpdateUserSyncSettingsParams{
		ID:         user.ID,
		SyncPeriod: periodStr,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	if h.Worker.IsActive() {
		h.Worker.Restart()
	}

	c.Redirect(http.StatusSeeOther, "/settings/sync")
}

func (h *Handler) UpdateServerSettingsHandler(c *gin.Context) {
	allowCreateStr := c.PostForm("allow_new_user_creation")
	allowCreate := "false"
	if allowCreateStr == "on" {
		allowCreate = "true"
	}

	enableWorkerStr := c.PostForm("enabled_on_startup")
	enableWorker := "false"
	if enableWorkerStr == "on" {
		enableWorker = "true"
	}

	err := h.DB.SetAppConfig(c.Request.Context(), database.SetAppConfigParams{
		Key:   "allow_new_user_creation",
		Value: allowCreate,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": "Failed to update allow_new_user_creation: " + err.Error(),
			"title": "Error",
		}))
		return
	}

	err = h.DB.SetAppConfig(c.Request.Context(), database.SetAppConfigParams{
		Key:   "enable_worker_on_startup",
		Value: enableWorker,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": "Failed to update enable_worker_on_startup: " + err.Error(),
			"title": "Error",
		}))
		return
	}

	c.Redirect(http.StatusSeeOther, "/settings/sync")
}

func (h *Handler) ResetSyncSettingsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	_, err := h.DB.UpdateUserSyncSettings(c, database.UpdateUserSyncSettingsParams{
		ID:         user.ID,
		SyncPeriod: "30m",
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	h.Worker.Restart()
	c.Redirect(http.StatusSeeOther, "/settings/sync")
}

func (h *Handler) StartWorkerHandler(c *gin.Context) {
	_, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	h.Worker.Start()
	c.Redirect(http.StatusSeeOther, "/settings/sync")
}

func (h *Handler) StopWorkerHandler(c *gin.Context) {
	h.Worker.Stop()
	c.Redirect(http.StatusSeeOther, "/settings/sync")
}

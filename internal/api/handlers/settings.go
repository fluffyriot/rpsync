package handlers

import (
	"net/http"
	"time"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gin-gonic/gin"
)

func (h *Handler) UserSetupViewHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "user-setup.html", h.CommonData(gin.H{
		"title": "Welcome - Setup Admin User",
	}))
}

func (h *Handler) UserSetupHandler(c *gin.Context) {
	username := c.PostForm("username")
	if username == "" {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(gin.H{
			"error": "username is required",
			"title": "Error",
		}))
		return
	}

	_, _, err := config.CreateUserFromForm(h.DB, username)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) SyncSettingsHandler(c *gin.Context) {
	users, err := h.DB.GetAllUsers(c)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	if len(users) == 0 {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	user := users[0]

	isSecure := c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https"
	if h.Config.ClientIP == "localhost" {
		isSecure = true
	}

	isWebauthnConfigured := h.Config.WebAuthn != nil
	isPasskeySupported := isSecure && isWebauthnConfigured

	c.HTML(http.StatusOK, "sync-settings.html", h.CommonData(gin.H{
		"sync_period":            user.SyncPeriod,
		"enabled_on_startup":     user.EnabledOnStartup,
		"worker_running":         h.Worker.IsActive(),
		"title":                  "Sync Settings",
		"is_2fa_enabled":         user.TotpEnabled.Bool,
		"is_webauthn_configured": isWebauthnConfigured,
		"is_secure_context":      isSecure,
		"is_passkey_supported":   isPasskeySupported,
	}))
}

func (h *Handler) UpdateSyncSettingsHandler(c *gin.Context) {
	periodStr := c.PostForm("sync_period")
	enabledStr := c.PostForm("enabled_on_startup")
	enabled := enabledStr == "on"

	duration, err := time.ParseDuration(periodStr)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(gin.H{
			"error": "Invalid duration format",
			"title": "Error",
		}))
		return
	}

	users, err := h.DB.GetAllUsers(c)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}
	if len(users) == 0 {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	user := users[0]

	_, err = h.DB.UpdateUserSyncSettings(c, database.UpdateUserSyncSettingsParams{
		ID:               user.ID,
		SyncPeriod:       periodStr,
		EnabledOnStartup: enabled,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	if enabled {
		h.Worker.Restart(duration)
	} else {
		h.Worker.Stop()
	}

	c.Redirect(http.StatusSeeOther, "/settings/sync")
}

func (h *Handler) ResetSyncSettingsHandler(c *gin.Context) {
	users, err := h.DB.GetAllUsers(c)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}
	if len(users) == 0 {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	user := users[0]

	_, err = h.DB.UpdateUserSyncSettings(c, database.UpdateUserSyncSettingsParams{
		ID:               user.ID,
		SyncPeriod:       "30m",
		EnabledOnStartup: true,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	h.Worker.Restart(30 * time.Minute)
	c.Redirect(http.StatusSeeOther, "/settings/sync")
}

func (h *Handler) StartWorkerHandler(c *gin.Context) {
	users, err := h.DB.GetAllUsers(c)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}
	if len(users) == 0 {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	user := users[0]

	duration, err := time.ParseDuration(user.SyncPeriod)
	if err != nil {
		duration = 30 * time.Minute
	}

	h.Worker.Start(duration)
	c.Redirect(http.StatusSeeOther, "/settings/sync")
}

func (h *Handler) StopWorkerHandler(c *gin.Context) {
	h.Worker.Stop()
	c.Redirect(http.StatusSeeOther, "/settings/sync")
}

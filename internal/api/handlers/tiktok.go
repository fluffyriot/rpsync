package handlers

import (
	"encoding/base64"
	"net/http"

	"github.com/fluffyriot/commission-tracker/internal/fetcher"
	"github.com/gin-gonic/gin"
)

// TikTokLoginHandler initiates the login session and shows the QR code page
func (h *Handler) TikTokLoginHandler(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "username is required",
		})
		return
	}

	// Start or get session
	// Note: StartLoginSession is blocking until QR code is captured, so we might want to do it in a goroutine
	// or handle it gracefully. However, we need the QR code to render the page.
	// If the user refreshes, we might want to return existing QR or start new.
	// For simplicity, let's just start a new one or use existing if status is "initiating".

	// Since StartLoginSession waits for QR code, it might take a few seconds.
	qrCode, err := fetcher.GlobalTikTokManager.StartLoginSession(username)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to start TikTok login session: " + err.Error(),
		})
		return
	}

	// Helper to display the image. We'll pass it as base64 string
	qrBase64 := base64.StdEncoding.EncodeToString(qrCode)

	c.HTML(http.StatusOK, "tiktok_login.html", gin.H{
		"Username": username,
		"QRCode":   qrBase64,
	})
}

// TikTokCheckHandler checks the status of the login
func (h *Handler) TikTokCheckHandler(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	status, msg, err := fetcher.GlobalTikTokManager.CheckStatus(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  status,
		"message": msg,
	})
}

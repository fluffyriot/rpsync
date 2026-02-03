package handlers

import (
	"encoding/base64"
	"net/http"

	"github.com/fluffyriot/rpsync/internal/fetcher"
	"github.com/gin-gonic/gin"
)

func (h *Handler) TikTokLoginHandler(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(c, gin.H{
			"error": "username is required",
			"title": "Error",
		}))
		return
	}

	qrCode, err := fetcher.GlobalTikTokManager.StartLoginSession(username)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": "Failed to start TikTok login session: " + err.Error(),
			"title": "Error",
		}))
		return
	}

	qrBase64 := base64.StdEncoding.EncodeToString(qrCode)

	c.HTML(http.StatusOK, "tiktok_login.html", h.CommonData(c, gin.H{
		"Username": username,
		"QRCode":   qrBase64,
		"title":    "TikTok Login",
	}))
}

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

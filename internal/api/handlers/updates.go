// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"fmt"
	"net/http"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (h *Handler) GetReleaseNotesHandler(c *gin.Context) {
	currentVersion := c.Query("version")
	if currentVersion == "" {
		currentVersion = config.AppVersion
	}

	lastSeenVersion := c.Query("last_seen")
	if lastSeenVersion == "" {
		lastSeenVersion = "0.0.0"
	}

	limit := 0
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	notes, err := h.Updater.GetReleaseNotes(lastSeenVersion, currentVersion, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notes":    notes.Body,
		"version":  notes.TagName,
		"name":     notes.Name,
		"html_url": notes.HtmlUrl,
	})
}

func (h *Handler) UpdateLastSeenVersionHandler(c *gin.Context) {
	var req struct {
		Version string `json:"version" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, authenticated := h.GetAuthenticatedUser(c)
	if !authenticated {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	_, err := h.DB.UpdateUserLastSeenVersion(c.Request.Context(), database.UpdateUserLastSeenVersionParams{
		ID:              user.ID,
		LastSeenVersion: req.Version,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user version"})
		return
	}

	session := sessions.Default(c)
	session.Set("last_seen_version", req.Version)
	session.Save()

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/fluffyriot/rpsync/internal/exports"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ExportsHandler(c *gin.Context) {

	if h.Config.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": h.Config.DBInitErr.Error(),
			"title": "Error",
		}))
		return
	}

	ctx := c.Request.Context()

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	exports, err := h.DB.GetLast20ExportsByUserId(ctx, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}
	c.HTML(http.StatusOK, "exports.html", h.CommonData(c, gin.H{
		"username": user.Username,
		"user_id":  user.ID,
		"exports":  exports,
		"title":    "Exports",
	}))
}

func (h *Handler) ExportDeleteAllHandler(c *gin.Context) {
	userId, err := uuid.Parse(c.PostForm("user_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(c, gin.H{
			"error": err.Error(),
			"title": "Error",
		}))
		return
	}

	go func(uid uuid.UUID) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in background sync: %v", r)
			}
		}()
		exports.DeleteAllExports(uid, h.DB)
	}(userId)

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) DownloadExportHandler(c *gin.Context) {
	ctx := c.Request.Context()
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	requestedFilename := c.Param("filepath")[1:]
	requestedFilename = filepath.Clean(requestedFilename)

	parts := strings.Split(requestedFilename, "_")
	if len(parts) < 3 || parts[0] != "export" || parts[1] != "id" {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(c, gin.H{
			"error": "Invalid filename format",
			"title": "Error",
		}))
		return
	}

	exportIDStr := parts[2]
	exportID, err := uuid.Parse(exportIDStr)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", h.CommonData(c, gin.H{
			"error": "Invalid export ID",
			"title": "Error",
		}))
		return
	}

	export, err := h.DB.GetExportById(ctx, exportID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.HTML(http.StatusNotFound, "error.html", h.CommonData(c, gin.H{
				"error": "Export not found",
				"title": "Error",
			}))
		} else {
			c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
				"error": "Internal server error",
				"title": "Error",
			}))
		}
		return
	}

	if export.UserID != user.ID {
		c.HTML(http.StatusForbidden, "error.html", h.CommonData(c, gin.H{
			"error": "Access denied",
			"title": "Error",
		}))
		return
	}

	if !export.DownloadUrl.Valid {
		c.HTML(http.StatusNotFound, "error.html", h.CommonData(c, gin.H{
			"error": "Export file info missing",
			"title": "Error",
		}))
		return
	}

	storedPath := export.DownloadUrl.String
	storedFilename := filepath.Base(storedPath)

	if storedFilename != requestedFilename {
		c.HTML(http.StatusForbidden, "error.html", h.CommonData(c, gin.H{
			"error": "Access denied",
			"title": "Error",
		}))
		return
	}

	baseDir, err := filepath.Abs("./outputs")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": "Internal server error resolving base path",
			"title": "Error",
		}))
		return
	}

	fullPath := filepath.Join(baseDir, requestedFilename)

	if !strings.HasPrefix(fullPath, baseDir) {
		c.HTML(http.StatusForbidden, "error.html", h.CommonData(c, gin.H{
			"error": "Access denied",
			"title": "Error",
		}))
		return
	}

	c.FileAttachment(fullPath, requestedFilename)
}

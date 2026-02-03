package handlers

import (
	"log"
	"net/http"
	"path/filepath"

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
	p := c.Param("filepath")[1:]
	c.FileAttachment(filepath.Join("./outputs", p), filepath.Base(p))
}

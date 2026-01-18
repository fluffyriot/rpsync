package handlers

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/fluffyriot/commission-tracker/internal/exports"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ExportsHandler(c *gin.Context) {

	if h.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": h.DBInitErr.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	users, err := h.DB.GetAllUsers(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	if len(users) == 0 {
		c.HTML(http.StatusOK, "user-setup.html", nil)
		return
	}

	user := users[0]

	exports, err := h.DB.GetLast20ExportsByUserId(ctx, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}
	c.HTML(http.StatusOK, "exports.html", gin.H{
		"username": user.Username,
		"user_id":  user.ID,
		"exports":  exports,
	})
}

func (h *Handler) ExportDeleteAllHandler(c *gin.Context) {
	userId, err := uuid.Parse(c.PostForm("user_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": err.Error(),
		})
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

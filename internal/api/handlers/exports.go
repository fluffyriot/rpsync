package handlers

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/fluffyriot/rpsync/internal/config"
	"github.com/fluffyriot/rpsync/internal/exports"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ExportsHandler(c *gin.Context) {

	if h.Config.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       h.Config.DBInitErr.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	ctx := c.Request.Context()

	users, err := h.DB.GetAllUsers(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	if len(users) == 0 {
		c.HTML(http.StatusOK, "user-setup.html", gin.H{
			"app_version": config.AppVersion,
		})
		return
	}

	user := users[0]

	exports, err := h.DB.GetLast20ExportsByUserId(ctx, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}
	c.HTML(http.StatusOK, "exports.html", gin.H{
		"username":    user.Username,
		"user_id":     user.ID,
		"exports":     exports,
		"app_version": config.AppVersion,
	})
}

func (h *Handler) ExportDeleteAllHandler(c *gin.Context) {
	userId, err := uuid.Parse(c.PostForm("user_id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
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

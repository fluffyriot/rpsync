package handlers

import (
	"net/http"

	"github.com/fluffyriot/commission-tracker/internal/config"
	"github.com/gin-gonic/gin"
)

func (h *Handler) UserSetupHandler(c *gin.Context) {
	username := c.PostForm("username")
	if username == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":       "username is required",
			"app_version": config.AppVersion,
		})
		return
	}

	_, _, err := config.CreateUserFromForm(h.DB, username)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

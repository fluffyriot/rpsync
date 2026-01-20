package handlers

import (
	"net/http"

	"github.com/fluffyriot/commission-tracker/internal/config"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RootHandler(c *gin.Context) {

	if h.Config.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       h.Config.DBInitErr.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	if h.Config.KeyB64Err1 != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       h.Config.KeyB64Err1.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	if h.Config.KeyB64Err2 != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       h.Config.KeyB64Err2.Error(),
			"app_version": config.AppVersion,
		})
		return
	}

	if h.Config.InstVerErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       h.Config.InstVerErr.Error(),
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

	sources, err := h.DB.GetUserSources(ctx, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":       err.Error(),
			"app_version": config.AppVersion,
		})
		return
	}
	c.HTML(http.StatusOK, "index.html", gin.H{
		"username":    user.Username,
		"user_id":     user.ID,
		"sources":     sources,
		"app_version": config.AppVersion,
	})
}

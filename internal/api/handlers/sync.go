package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) TriggerSyncHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User not logged in",
		})
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in manual sync trigger: %v", r)
			}
		}()
		h.Worker.SyncUserManual(user.ID)
	}()

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Sync triggered successfully",
	})
}

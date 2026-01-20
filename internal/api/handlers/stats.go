package handlers

import (
	"log"
	"net/http"

	"github.com/fluffyriot/commission-tracker/internal/stats"
	"github.com/gin-gonic/gin"
)

func (h *Handler) StatsHandler(c *gin.Context) {

	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.Config.DBInitErr.Error()})
		return
	}

	ctx := c.Request.Context()

	users, err := h.DB.GetAllUsers(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(users) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No user found"})
		return
	}

	user := users[0]

	statsData, err := stats.GetStats(h.DB, user.ID)
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, statsData)
}

func (h *Handler) AnalyticsPageHandler(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": h.Config.DBInitErr.Error(),
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
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	user := users[0]

	c.HTML(http.StatusOK, "analytics.html", gin.H{
		"username": user.Username,
	})
}

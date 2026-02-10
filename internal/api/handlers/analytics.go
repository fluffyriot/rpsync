// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"log"
	"net/http"

	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/fluffyriot/rpsync/internal/stats"
	"github.com/gin-gonic/gin"
)

func (h *Handler) AnalyticsEngagementHandler(c *gin.Context) {

	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	statsData, err := stats.GetStats(h.DB, user.ID)
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, statsData)
}

func (h *Handler) AnalyticsWebsiteHandler(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	statsData, err := stats.GetAnalyticsStats(h.DB, user.ID)
	if err != nil {
		log.Printf("Error getting analytics stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, statsData)
}

func (h *Handler) AnalyticsDashboardSummaryHandler(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	summary, err := stats.GetDashboardSummary(h.DB, user.ID)
	if err != nil {
		log.Printf("Error getting dashboard summary: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *Handler) AnalyticsTopSourcesHandler(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.Config.DBInitErr.Error()})
		return
	}

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	topSourcesDB, err := h.DB.GetRestTopSources(c.Request.Context(), user.ID)
	if err != nil {
		log.Printf("Error getting rest of top sources: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var topSources []TopSourceViewModel
	for _, src := range topSourcesDB {
		profileURL, _ := helpers.ConvNetworkToURL(src.Network, src.UserName)
		topSources = append(topSources, TopSourceViewModel{
			ID:                src.ID,
			UserName:          src.UserName,
			Network:           src.Network,
			TotalInteractions: int64(src.TotalInteractions),
			FollowersCount:    int64(src.FollowersCount),
			ProfileURL:        profileURL,
		})
	}

	if topSources == nil {
		topSources = []TopSourceViewModel{}
	}

	c.JSON(http.StatusOK, topSources)
}

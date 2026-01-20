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

	activeSources, _ := h.DB.GetActiveSourcesCount(ctx)
	activeTargets, _ := h.DB.GetActiveTargetsCount(ctx)
	totalPosts, _ := h.DB.GetTotalPostsCount(ctx)
	reactions, _ := h.DB.GetTotalReactions(ctx)
	siteStats, _ := h.DB.GetTotalSiteStats(ctx)
	pageViews, _ := h.DB.GetTotalPageViews(ctx)
	syncErrors30d, _ := h.DB.GetSyncErrorsCountLast30Days(ctx)
	recentLogs, _ := h.DB.GetRecentLogs(ctx)

	c.HTML(http.StatusOK, "index.html", gin.H{
		"username":         user.Username,
		"user_id":          user.ID,
		"app_version":      config.AppVersion,
		"active_sources":   activeSources,
		"active_targets":   activeTargets,
		"total_posts":      totalPosts,
		"total_likes":      reactions.TotalLikes,
		"total_shares":     reactions.TotalShares,
		"total_views":      reactions.TotalViews,
		"total_visitors":   siteStats,
		"total_page_views": pageViews,
		"sync_errors_30d":  syncErrors30d,
		"recent_logs":      recentLogs,
	})
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) RootHandler(c *gin.Context) {

	if h.Config.DBInitErr != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": h.Config.DBInitErr.Error(),
			"title": "Error",
		}))
		return
	}

	if h.Config.KeyB64Err1 != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": h.Config.KeyB64Err1.Error(),
			"title": "Error",
		}))
		return
	}

	if h.Config.KeyB64Err2 != nil {
		c.HTML(http.StatusInternalServerError, "error.html", h.CommonData(c, gin.H{
			"error": h.Config.KeyB64Err2.Error(),
			"title": "Error",
		}))
		return
	}

	ctx := c.Request.Context()

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		users, err := h.DB.GetAllUsers(ctx)
		if err == nil && len(users) == 0 {
			c.HTML(http.StatusOK, "user-setup.html", h.CommonData(c, gin.H{
				"title": "Setup",
			}))
			return
		}
		c.Redirect(http.StatusFound, "/login")
		return
	}

	activeSources, _ := h.DB.GetActiveSourcesCount(ctx, user.ID)
	activeTargets, _ := h.DB.GetActiveTargetsCount(ctx, user.ID)
	totalPosts, _ := h.DB.GetTotalPostsCount(ctx, user.ID)
	reactions, _ := h.DB.GetTotalReactions(ctx, user.ID)
	siteStats, _ := h.DB.GetTotalSiteStats(ctx, user.ID)
	pageViews, _ := h.DB.GetTotalPageViews(ctx, user.ID)
	siteAvSession, _ := h.DB.GetAverageWebsiteSession(ctx, user.ID)
	syncErrors30d, _ := h.DB.GetSyncErrorsCountLast30Days(ctx, user.ID)
	recentLogs, _ := h.DB.GetRecentLogs(ctx, user.ID)

	workerStatus := "Off"
	workerIsOff := true
	if h.Worker.IsActive() {
		workerStatus = "On"
		workerIsOff = false
	}

	c.HTML(http.StatusOK, "index.html", h.CommonData(c, gin.H{
		"username":                user.Username,
		"user_id":                 user.ID,
		"active_sources":          activeSources,
		"active_targets":          activeTargets,
		"total_posts":             totalPosts,
		"total_likes":             reactions.TotalLikes,
		"total_shares":            reactions.TotalShares,
		"total_views":             reactions.TotalViews,
		"total_visitors":          siteStats,
		"total_page_views":        pageViews,
		"average_website_session": siteAvSession,
		"sync_errors_30d":         syncErrors30d,
		"recent_logs":             recentLogs,
		"worker_status":           workerStatus,
		"worker_is_off":           workerIsOff,
		"sync_period":             user.SyncPeriod,
		"title":                   "Dashboard",
	}))
}

func (h *Handler) DismissLogHandler(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	_ = h.DB.DismissLog(c.Request.Context(), id)

	c.Redirect(http.StatusFound, "/#recent-logs")
}

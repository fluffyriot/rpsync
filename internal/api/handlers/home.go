// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"log"
	"net/http"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/helpers"
	"github.com/fluffyriot/rpsync/internal/stats"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
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

	if !user.PasswordHash.Valid || user.PasswordHash.String == "" {
		c.Redirect(http.StatusFound, "/setup/password")
		return
	}

	var (
		activeSources int64
		activeTargets int64
		totalPosts    int64
		reactions     database.GetTotalReactionsRow
		siteStats     int64
		pageViews     int64
		siteAvSession int64
		syncErrors30d int64
		recentLogs    []database.GetRecentLogsRow
		topSourcesDB  []database.GetTopSourcesRow
		dashSummary   *stats.DashboardSummary
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		activeSources, err = h.DB.GetActiveSourcesCount(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		activeTargets, err = h.DB.GetActiveTargetsCount(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		totalPosts, err = h.DB.GetTotalPostsCount(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		reactions, err = h.DB.GetTotalReactions(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		var count int
		count, err = h.DB.GetTotalSiteStats(ctx, user.ID)
		siteStats = int64(count)
		return err
	})

	g.Go(func() error {
		var err error
		var count int
		count, err = h.DB.GetTotalPageViews(ctx, user.ID)
		pageViews = int64(count)
		return err
	})

	g.Go(func() error {
		var err error
		var count int
		count, err = h.DB.GetAverageWebsiteSession(ctx, user.ID)
		siteAvSession = int64(count)
		return err
	})

	g.Go(func() error {
		var err error
		syncErrors30d, err = h.DB.GetSyncErrorsCountLast30Days(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		recentLogs, err = h.DB.GetRecentLogs(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		topSourcesDB, err = h.DB.GetTopSources(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		dashSummary, err = stats.GetDashboardSummary(h.DB, user.ID)
		return err
	})

	if err := g.Wait(); err != nil {
		log.Printf("Error getting dashboard data: %v", err)
	}

	workerStatus := "Off"
	workerIsOff := true
	if h.Worker.IsActive() {
		workerStatus = "On"
		workerIsOff = false
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
		"top_sources":             topSources,
		"dashboard_summary":       dashSummary,
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

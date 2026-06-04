// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/helpers"
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

	workerStatus := "Off"
	workerIsOff := true
	if h.Worker.IsActive() {
		workerStatus = "On"
		workerIsOff = false
	}

	c.HTML(http.StatusOK, "index.html", h.CommonData(c, gin.H{
		"worker_status": workerStatus,
		"worker_is_off": workerIsOff,
		"sync_period":   user.SyncPeriod,
		"title":         "Dashboard",
	}))
}

func (h *Handler) DashboardStatsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx := c.Request.Context()

	var (
		resp         DashboardStatsResponse
		topSourcesDB []database.GetTopSourcesRow
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		resp.ActiveSources, err = h.DB.GetActiveSourcesCount(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		resp.ActiveTargets, err = h.DB.GetActiveTargetsCount(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		resp.TotalPosts, err = h.DB.GetTotalPostsCount(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		reactions, err := h.DB.GetTotalReactions(ctx, user.ID)
		if err != nil {
			return err
		}
		resp.TotalLikes = reactions.TotalLikes
		resp.TotalShares = reactions.TotalShares
		resp.TotalViews = reactions.TotalViews
		return nil
	})

	g.Go(func() error {
		var err error
		resp.TotalVisitors, err = h.DB.GetTotalSiteStats(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		resp.TotalPageViews, err = h.DB.GetTotalPageViews(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		resp.AverageWebsiteSession, err = h.DB.GetAverageWebsiteSession(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		resp.SyncErrors30d, err = h.DB.GetSyncErrorsCountLast30Days(ctx, user.ID)
		return err
	})

	g.Go(func() error {
		var err error
		topSourcesDB, err = h.DB.GetTopSources(ctx, user.ID)
		return err
	})

	if err := g.Wait(); err != nil {
		log.Printf("Error getting dashboard stats: %v", err)
	}

	for _, src := range topSourcesDB {
		caps := helpers.GetSourceByName(src.Network)
		if caps != nil && !caps.EngagementSupported && !caps.ViewsSupported && !caps.FollowersTracked {
			continue
		}
		profileURL, _ := helpers.ConvNetworkToURL(src.Network, src.UserName)
		vm := TopSourceViewModel{
			ID:                src.ID,
			UserName:          src.UserName,
			Network:           src.Network,
			TotalInteractions: int64(src.TotalInteractions),
			TotalViews:        int64(src.TotalViews),
			FollowersCount:    int64(src.FollowersCount),
			ProfileURL:        profileURL,
		}
		if caps != nil {
			vm.EngagementSupported = caps.EngagementSupported
			vm.ViewsSupported = caps.ViewsSupported
			vm.FollowersTracked = caps.FollowersTracked
		}
		resp.TopSources = append(resp.TopSources, vm)
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DashboardLogsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rows, err := h.DB.GetRecentLogs(c.Request.Context(), user.ID)
	if err != nil {
		log.Printf("Error getting recent logs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch logs"})
		return
	}

	var items []DashboardLogItem
	for _, r := range rows {
		item := DashboardLogItem{
			ID:        r.ID.String(),
			CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
			Message:   r.Message,
		}
		if r.SourceNetwork.Valid {
			item.SourceNetwork = r.SourceNetwork.String
		}
		if r.SourceUsername.Valid {
			item.SourceUsername = r.SourceUsername.String
		}
		if r.TargetType.Valid {
			item.TargetType = r.TargetType.String
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, items)
}

func (h *Handler) DismissAllLogsHandler(c *gin.Context) {
	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	_ = h.DB.DismissAllLogs(c.Request.Context(), user.ID)
	c.Redirect(http.StatusFound, "/#recent-logs")
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

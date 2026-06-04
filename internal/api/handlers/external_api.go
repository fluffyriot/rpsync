// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) getAPIUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDStr, exists := c.Get("api_user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.UUID{}, false
	}
	uid, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return uuid.UUID{}, false
	}
	return uid, true
}

type statsRequest struct {
	CompareDate *string `json:"compare_date"`
}

func (h *Handler) ExternalAPIStatsHandler(c *gin.Context) {
	userID, ok := h.getAPIUserID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	var req statsRequest
	// Body is optional — ignore bind errors
	_ = c.ShouldBindJSON(&req)

	var compareDate *time.Time
	if req.CompareDate != nil && *req.CompareDate != "" {
		parsed, err := time.Parse("2006-01-02", *req.CompareDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid compare_date format, expected YYYY-MM-DD"})
			return
		}
		compareDate = &parsed
	}

	currentStats, err := h.DB.GetCurrentTotalStats(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get current stats"})
		return
	}

	currentFollowers, err := h.DB.GetCurrentTotalFollowers(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get follower count"})
		return
	}

	currentWebsite, err := h.DB.GetCurrentWebsiteStats(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get website stats"})
		return
	}

	currentEngagement := currentStats.TotalLikes + currentStats.TotalReposts

	likes := gin.H{"current": currentStats.TotalLikes}
	reposts := gin.H{"current": currentStats.TotalReposts}
	engagement := gin.H{"current": currentEngagement}
	postsViews := gin.H{"current": currentStats.TotalViews}
	followers := gin.H{"current": currentFollowers}
	websiteViews := gin.H{"current": currentWebsite.TotalPageViews}
	websiteVisitors := gin.H{"current": currentWebsite.TotalVisitors}
	websiteImpressions := gin.H{"current": currentWebsite.TotalImpressions}

	if compareDate != nil {
		previousStats, err := h.DB.GetTotalStatsAtDate(ctx, database.GetTotalStatsAtDateParams{
			UserID:   userID,
			SyncedAt: compareDate.Add(24*time.Hour - time.Nanosecond),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get historical stats"})
			return
		}

		previousFollowers, err := h.DB.GetTotalFollowersAtDate(ctx, database.GetTotalFollowersAtDateParams{
			UserID: userID,
			Date:   *compareDate,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get historical follower count"})
			return
		}

		previousWebsite, err := h.DB.GetWebsiteStatsAtDate(ctx, database.GetWebsiteStatsAtDateParams{
			UserID: userID,
			Date:   *compareDate,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get historical website stats"})
			return
		}

		previousEngagement := previousStats.TotalLikes + previousStats.TotalReposts

		likes["delta"] = currentStats.TotalLikes - previousStats.TotalLikes
		reposts["delta"] = currentStats.TotalReposts - previousStats.TotalReposts
		engagement["delta"] = currentEngagement - previousEngagement
		postsViews["delta"] = currentStats.TotalViews - previousStats.TotalViews
		followers["delta"] = currentFollowers - previousFollowers
		websiteViews["delta"] = currentWebsite.TotalPageViews - previousWebsite.TotalPageViews
		websiteVisitors["delta"] = currentWebsite.TotalVisitors - previousWebsite.TotalVisitors
		websiteImpressions["delta"] = currentWebsite.TotalImpressions - previousWebsite.TotalImpressions
	}

	c.JSON(http.StatusOK, gin.H{
		"likes":               likes,
		"reposts":             reposts,
		"engagement":          engagement,
		"posts_views":         postsViews,
		"followers":           followers,
		"website_views":       websiteViews,
		"website_visitors":    websiteVisitors,
		"website_impressions": websiteImpressions,
	})
}

type startSyncRequest struct {
	SourceIDs []string `json:"sourceIds"`
	TargetIDs []string `json:"targetIds"`
}

func (h *Handler) ExternalAPISyncHandler(c *gin.Context) {
	userID, ok := h.getAPIUserID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	var req startSyncRequest
	_ = c.ShouldBindJSON(&req)

	var sourceIDs []uuid.UUID
	for _, raw := range req.SourceIDs {
		sid, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source id: " + raw})
			return
		}
		source, err := h.DB.GetSourceById(ctx, sid)
		if err != nil || source.UserID != userID {
			c.JSON(http.StatusNotFound, gin.H{"error": "source not found: " + raw})
			return
		}
		sourceIDs = append(sourceIDs, sid)
	}

	var targetIDs []uuid.UUID
	for _, raw := range req.TargetIDs {
		tid, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target id: " + raw})
			return
		}
		target, err := h.DB.GetTargetById(ctx, tid)
		if err != nil || target.UserID != userID {
			c.JSON(http.StatusNotFound, gin.H{"error": "target not found: " + raw})
			return
		}
		targetIDs = append(targetIDs, tid)
	}

	if started := h.Worker.TrySyncSelective(userID, sourceIDs, targetIDs); !started {
		c.JSON(http.StatusConflict, gin.H{"status": "error", "message": "Sync already running"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Sync started"})
}

type extCreateExclusionRequest struct {
	SourceID           string   `json:"sourceId" binding:"required"`
	NetworkInternalIDs []string `json:"networkInternalIDs" binding:"required"`
}

func (h *Handler) ExternalAPICreateExclusionHandler(c *gin.Context) {
	userID, ok := h.getAPIUserID(c)
	if !ok {
		return
	}

	var req extCreateExclusionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	sourceID, err := uuid.Parse(req.SourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sourceId format"})
		return
	}

	ctx := c.Request.Context()

	source, err := h.DB.GetSourceById(ctx, sourceID)
	if err != nil || source.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}

	var created []database.Exclusion

	for _, raw := range req.NetworkInternalIDs {
		nid := strings.TrimSpace(raw)
		if nid == "" {
			continue
		}

		exclusion, err := h.createExclusionForNetworkID(ctx, sourceID, nid)
		if err != nil {
			if strings.Contains(err.Error(), "unique_exclusion") {
				continue
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to exclude " + nid + ": " + err.Error()})
			return
		}

		created = append(created, exclusion)
	}

	c.JSON(http.StatusCreated, created)
}

type extCreateRedirectRequest struct {
	SourceIDs []string `json:"sourceIDs" binding:"required"`
	FromPage  string   `json:"fromPage" binding:"required"`
	ToPage    string   `json:"toPage" binding:"required"`
}

func (h *Handler) ExternalAPICreateRedirectHandler(c *gin.Context) {
	userID, ok := h.getAPIUserID(c)
	if !ok {
		return
	}

	var req extCreateRedirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Validate all source IDs and ownership before creating anything.
	var sourceIDs []uuid.UUID
	for _, raw := range req.SourceIDs {
		sid, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sourceId: " + raw})
			return
		}
		source, err := h.DB.GetSourceById(ctx, sid)
		if err != nil || source.UserID != userID {
			c.JSON(http.StatusNotFound, gin.H{"error": "source not found: " + raw})
			return
		}
		sourceIDs = append(sourceIDs, sid)
	}

	var result []database.Redirect

	for _, sid := range sourceIDs {
		redirect, err := h.DB.CreateRedirect(ctx, database.CreateRedirectParams{
			ID:        uuid.New(),
			SourceID:  sid,
			FromPath:  req.FromPage,
			ToPath:    req.ToPage,
			CreatedAt: time.Now(),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create redirect for source " + sid.String() + ": " + err.Error()})
			return
		}

		go h.mergeRedirectPageStats(sid, req.FromPage, req.ToPage)

		result = append(result, redirect)
	}

	c.JSON(http.StatusCreated, result)
}

func (h *Handler) ExternalAPIStartWorkerHandler(c *gin.Context) {
	_, ok := h.getAPIUserID(c)
	if !ok {
		return
	}

	h.Worker.Start()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Worker started"})
}

func (h *Handler) ExternalAPIStopWorkerHandler(c *gin.Context) {
	_, ok := h.getAPIUserID(c)
	if !ok {
		return
	}

	h.Worker.Stop()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Worker stopped"})
}

func (h *Handler) ExternalAPISourcesHandler(c *gin.Context) {
	userID, ok := h.getAPIUserID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	sources, err := h.DB.GetUserSources(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get sources"})
		return
	}

	c.JSON(http.StatusOK, sources)
}

func (h *Handler) ExternalAPITargetsHandler(c *gin.Context) {
	userID, ok := h.getAPIUserID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	targets, err := h.DB.GetUserTargets(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get targets"})
		return
	}

	c.JSON(http.StatusOK, targets)
}

func (h *Handler) ExternalAPIStatusHandler(c *gin.Context) {
	userID, ok := h.getAPIUserID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	sourceCounts, err := h.DB.GetSourceStatusCounts(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get source status"})
		return
	}

	targetCounts, err := h.DB.GetTargetStatusCounts(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get target status"})
		return
	}

	user, err := h.DB.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	workerStatus := "off"
	if h.Worker.IsActive() {
		workerStatus = "on"
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": gin.H{"healthy": sourceCounts.HealthyCount, "enabled": sourceCounts.EnabledCount, "disabled": sourceCounts.DisabledCount},
		"targets": gin.H{"healthy": targetCounts.HealthyCount, "enabled": targetCounts.EnabledCount, "disabled": targetCounts.DisabledCount},
		"user": gin.H{
			"username":      user.Username,
			"sync_period":   user.SyncPeriod,
			"worker_status": workerStatus,
		},
	})
}

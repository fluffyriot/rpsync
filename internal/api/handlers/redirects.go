// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/fluffyriot/rpsync/internal/fetcher/sources"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RedirectResponse struct {
	ID        string `json:"id"`
	SourceID  string `json:"source_id"`
	FromPath  string `json:"from_path"`
	ToPath    string `json:"to_path"`
	CreatedAt string `json:"created_at"`
	Network   string `json:"network"`
	UserName  string `json:"user_name"`
}

type CreateRedirectRequest struct {
	SourceID string `json:"source_id" binding:"required"`
	FromPath string `json:"from_path" binding:"required"`
	ToPath   string `json:"to_path" binding:"required"`
}

func (h *Handler) HandleGetRedirects(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	sourceIDStr := c.Query("source_id")
	if sourceIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "source_id is required"})
		return
	}

	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid source_id format"})
		return
	}

	ctx := c.Request.Context()
	redirects, err := h.DB.GetRedirectsForSource(ctx, sourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	source, err := h.DB.GetSourceById(ctx, sourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get source details"})
		return
	}

	response := make([]RedirectResponse, len(redirects))
	for i, r := range redirects {
		response[i] = RedirectResponse{
			ID:        r.ID.String(),
			SourceID:  r.SourceID.String(),
			FromPath:  r.FromPath,
			ToPath:    r.ToPath,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
			Network:   source.Network,
			UserName:  source.UserName,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleCreateRedirect(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	var req CreateRedirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	sourceID, err := uuid.Parse(req.SourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid source_id format"})
		return
	}

	ctx := context.Background()

	_, err = h.DB.GetSourceById(ctx, sourceID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Source not found"})
		return
	}

	redirect, err := h.DB.CreateRedirect(ctx, database.CreateRedirectParams{
		ID:        uuid.New(),
		SourceID:  sourceID,
		FromPath:  req.FromPath,
		ToPath:    req.ToPath,
		CreatedAt: time.Now(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to create redirect: %v", err)})
		return
	}

	go h.mergeRedirectPageStats(sourceID, req.FromPath, req.ToPath)

	c.JSON(http.StatusCreated, RedirectResponse{
		ID:        redirect.ID.String(),
		SourceID:  redirect.SourceID.String(),
		FromPath:  redirect.FromPath,
		ToPath:    redirect.ToPath,
		CreatedAt: redirect.CreatedAt.Format(time.RFC3339),
	})
}

func (h *Handler) mergeRedirectPageStats(sourceID uuid.UUID, fromPath, toPath string) {
	ctx := context.Background()
	stats, err := h.DB.GetAnalyticsPageStatsBySource(ctx, sourceID)
	if err != nil {
		log.Printf("Error fetching stats for merge: %v", err)
		return
	}

	for _, stat := range stats {
		if stat.UrlPath != fromPath {
			continue
		}

		newViews := stat.Views
		newImpressions := stat.Impressions
		analyticsType := stat.AnalyticsType

		targetStat, found := findStatByDateAndPath(stats, stat.Date, toPath)
		if found {
			newViews = targetStat.Views + stat.Views
			if targetStat.Impressions.Valid || stat.Impressions.Valid {
				newImpressions = sql.NullInt64{
					Int64: targetStat.Impressions.Int64 + stat.Impressions.Int64,
					Valid: true,
				}
			}

			if err := h.DB.DeleteAnalyticsPageStat(ctx, targetStat.ID); err != nil {
				log.Printf("Error deleting to_path stat during merge: %v", err)
			}
		}

		if err := h.DB.DeleteAnalyticsPageStat(ctx, stat.ID); err != nil {
			log.Printf("Error deleting from_path stat: %v", err)
			continue
		}

		if _, err = h.DB.CreateAnalyticsPageStat(ctx, database.CreateAnalyticsPageStatParams{
			ID:            uuid.New(),
			Date:          stat.Date,
			UrlPath:       toPath,
			Views:         newViews,
			SourceID:      sourceID,
			AnalyticsType: analyticsType,
			Impressions:   newImpressions,
		}); err != nil {
			log.Printf("Error creating stat at new path: %v", err)
		}
	}
}

func findStatByDateAndPath(stats []database.AnalyticsPageStat, date time.Time, path string) (database.AnalyticsPageStat, bool) {
	for _, s := range stats {
		if s.Date.Equal(date) && s.UrlPath == path {
			return s, true
		}
	}
	return database.AnalyticsPageStat{}, false
}

func (h *Handler) HandleDeleteRedirect(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid ID format"})
		return
	}

	ctx := context.Background()

	redirect, err := h.DB.GetRedirectById(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Redirect not found"})
		return
	}

	err = h.DB.DeleteRedirect(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to delete redirect: %v", err)})
		return
	}

	go func() {
		bgCtx := context.Background()

		err := h.DB.DeleteAnalyticsPageStatsByPathAndSource(bgCtx, database.DeleteAnalyticsPageStatsByPathAndSourceParams{
			SourceID: redirect.SourceID,
			UrlPath:  redirect.FromPath,
		})
		if err != nil {
			log.Printf("Error clearing from_path stats during restore: %v", err)
		}

		err = h.DB.DeleteAnalyticsPageStatsByPathAndSource(bgCtx, database.DeleteAnalyticsPageStatsByPathAndSourceParams{
			SourceID: redirect.SourceID,
			UrlPath:  redirect.ToPath,
		})
		if err != nil {
			log.Printf("Error clearing to_path stats during restore: %v", err)
		}

		source, err := h.DB.GetSourceById(bgCtx, redirect.SourceID)
		if err != nil {
			log.Printf("Error fetching source for restore: %v", err)
			return
		}

		daysSinceCreation := int(time.Since(source.CreatedAt).Hours() / 24)
		totalDays := 730 + daysSinceCreation

		var fetchErr error
		if source.Network == "Google Search Console" {
			startDate := time.Now().AddDate(0, 0, -totalDays).Format("2006-01-02")
			endDate := time.Now().Format("2006-01-02")
			fetchErr = sources.FetchGoogleSearchConsoleStatsWithRange(h.DB, redirect.SourceID, h.Config.TokenEncryptionKey, startDate, endDate)
		} else {
			startDate := fmt.Sprintf("%ddaysAgo", totalDays)
			endDate := "today"
			fetchErr = sources.FetchGoogleAnalyticsStatsWithRange(h.DB, redirect.SourceID, h.Config.TokenEncryptionKey, startDate, endDate)
		}
		if fetchErr != nil {
			log.Printf("Error re-fetching stats after redirect deletion: %v", fetchErr)
		}
	}()

	c.JSON(http.StatusOK, SuccessResponse{Message: "Redirect deleted and safe restore triggered"})
}

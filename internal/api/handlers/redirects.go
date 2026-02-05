package handlers

import (
	"context"
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

	go func() {
		bgCtx := context.Background()
		stats, err := h.DB.GetAnalyticsPageStatsBySource(bgCtx, sourceID)
		if err != nil {
			log.Printf("Error fetching stats for merge: %v", err)
			return
		}

		for _, stat := range stats {
			if stat.UrlPath == req.FromPath {
				targetStat, found := findStatByDateAndPath(stats, stat.Date, req.ToPath)
				if found {
					newViews := targetStat.Views + stat.Views
					_, err := h.DB.CreateAnalyticsPageStat(bgCtx, database.CreateAnalyticsPageStatParams{
						ID:       targetStat.ID,
						Date:     targetStat.Date,
						UrlPath:  req.ToPath,
						Views:    newViews,
						SourceID: sourceID,
					})
					if err != nil {
						log.Printf("Error updating target stat during merge: %v", err)
						continue
					}

					err = h.DB.DeleteAnalyticsPageStat(bgCtx, stat.ID)
					if err != nil {
						log.Printf("Error deleting old stat after merge: %v", err)
					}
				} else {
					err = h.DB.UpdateAnalyticsPageStatPath(bgCtx, database.UpdateAnalyticsPageStatPathParams{
						ID:      stat.ID,
						UrlPath: req.ToPath,
					})
					if err != nil {
						log.Printf("Error renaming stat path: %v", err)
					}
				}
			}
		}
	}()

	c.JSON(http.StatusCreated, RedirectResponse{
		ID:        redirect.ID.String(),
		SourceID:  redirect.SourceID.String(),
		FromPath:  redirect.FromPath,
		ToPath:    redirect.ToPath,
		CreatedAt: redirect.CreatedAt.Format(time.RFC3339),
	})
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
		startDate := fmt.Sprintf("%ddaysAgo", totalDays)
		endDate := "today"

		err = sources.FetchGoogleAnalyticsStatsWithRange(h.DB, redirect.SourceID, h.Config.TokenEncryptionKey, startDate, endDate)
		if err != nil {
			log.Printf("Error re-fetching stats after redirect deletion: %v", err)
		}
	}()

	c.JSON(http.StatusOK, SuccessResponse{Message: "Redirect deleted and safe restore triggered"})
}

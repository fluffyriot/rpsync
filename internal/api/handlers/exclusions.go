// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fluffyriot/rpsync/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ExclusionResponse struct {
	ID                string `json:"id"`
	CreatedAt         string `json:"created_at"`
	SourceID          string `json:"source_id"`
	NetworkInternalID string `json:"network_internal_id"`
	Network           string `json:"network"`
	UserName          string `json:"user_name"`
}

type CreateExclusionRequest struct {
	SourceID          string `json:"source_id" binding:"required"`
	NetworkInternalID string `json:"network_internal_id" binding:"required"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}

func (h *Handler) HandleGetExclusions(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	ctx := c.Request.Context()

	user, loggedIn := h.GetAuthenticatedUser(c)
	if !loggedIn {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	exclusions, err := h.DB.GetExclusionsForUser(ctx, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]ExclusionResponse, len(exclusions))
	for i, excl := range exclusions {
		response[i] = ExclusionResponse{
			ID:                excl.ID.String(),
			CreatedAt:         excl.CreatedAt.Format(time.RFC3339),
			SourceID:          excl.SourceID.String(),
			NetworkInternalID: excl.NetworkInternalID,
			Network:           excl.Network,
			UserName:          excl.UserName,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleCreateExclusion(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	var req CreateExclusionRequest
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

	source, err := h.DB.GetSourceById(ctx, sourceID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Source not found"})
		return
	}

	rawIDs := strings.Split(req.NetworkInternalID, ",")
	var createdExclusions []ExclusionResponse
	var errors []string

	for _, rawID := range rawIDs {
		networkInternalID := strings.TrimSpace(rawID)
		if networkInternalID == "" {
			continue
		}

		exclusion, err := h.DB.CreateExclusion(ctx, database.CreateExclusionParams{
			ID:                uuid.New(),
			CreatedAt:         time.Now(),
			SourceID:          sourceID,
			NetworkInternalID: networkInternalID,
		})
		if err != nil {
			if err.Error() == "pq: duplicate key value violates unique constraint \"unique_exclusion\"" {
				log.Printf("Info: Exclusion already exists for %s", networkInternalID)
				continue
			}
			errors = append(errors, fmt.Sprintf("Failed to exclude %s: %v", networkInternalID, err))
			continue
		}

		err = h.DB.DeletePostBySourceAndNetworkId(ctx, database.DeletePostBySourceAndNetworkIdParams{
			SourceID:          sourceID,
			NetworkInternalID: networkInternalID,
		})
		if err != nil {
			log.Printf("Warning: Failed to delete post for exclusion: %v", err)
		}

		createdExclusions = append(createdExclusions, ExclusionResponse{
			ID:                exclusion.ID.String(),
			CreatedAt:         exclusion.CreatedAt.Format(time.RFC3339),
			SourceID:          exclusion.SourceID.String(),
			NetworkInternalID: exclusion.NetworkInternalID,
			Network:           source.Network,
			UserName:          source.UserName,
		})
	}

	if len(errors) > 0 && len(createdExclusions) == 0 {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: strings.Join(errors, "; ")})
		return
	}
	c.JSON(http.StatusCreated, createdExclusions)
}

func (h *Handler) HandleDeleteExclusion(c *gin.Context) {
	if h.Config.DBInitErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: h.Config.DBInitErr.Error()})
		return
	}

	exclusionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid exclusion ID format"})
		return
	}

	ctx := context.Background()

	err = h.DB.DeleteExclusion(ctx, exclusionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{Message: "Exclusion deleted successfully"})
}

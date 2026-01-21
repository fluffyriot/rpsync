package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/fluffyriot/commission-tracker/internal/database"
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

	users, err := h.DB.GetAllUsers(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if len(users) == 0 {
		c.JSON(http.StatusOK, []ExclusionResponse{})
		return
	}

	user := users[0]

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

	exclusion, err := h.DB.CreateExclusion(ctx, database.CreateExclusionParams{
		ID:                uuid.New(),
		CreatedAt:         time.Now(),
		SourceID:          sourceID,
		NetworkInternalID: req.NetworkInternalID,
	})
	if err != nil {
		if err.Error() == "pq: duplicate key value violates unique constraint \"unique_exclusion\"" {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "This exclusion already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	err = h.DB.DeletePostBySourceAndNetworkId(ctx, database.DeletePostBySourceAndNetworkIdParams{
		SourceID:          sourceID,
		NetworkInternalID: req.NetworkInternalID,
	})
	if err != nil {
		log.Printf("Warning: Failed to delete post for exclusion: %v", err)
	}

	response := ExclusionResponse{
		ID:                exclusion.ID.String(),
		CreatedAt:         exclusion.CreatedAt.Format(time.RFC3339),
		SourceID:          exclusion.SourceID.String(),
		NetworkInternalID: exclusion.NetworkInternalID,
		Network:           source.Network,
		UserName:          source.UserName,
	}

	c.JSON(http.StatusCreated, response)
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

// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"context"
	"strings"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UpdateSourceChannelsRequest struct {
	ChannelIDs []string `json:"channel_ids" binding:"required"`
}

type GetSourceChannelsResponse struct {
	Network  string   `json:"network"`
	Channels []string `json:"channels"`
}

type UpdateChannelsRequest struct {
	Channels []string `json:"channels" binding:"required"`
}

func (h *Handler) UpdateSourceChannelsHandler(c *gin.Context) {
	sourceIDStr := c.Param("source_id")
	if sourceIDStr == "" {
		c.JSON(400, gin.H{"error": "source_id is required"})
		return
	}

	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid source ID"})
		return
	}

	source, err := h.DB.GetSourceById(c.Request.Context(), sourceID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Source not found"})
		return
	}

	if source.Network != "Discord" {
		c.JSON(400, gin.H{
			"error": "Channel management not supported for this network",
			"hint":  "Currently only Discord sources support channel management",
		})
		return
	}

	var req UpdateChannelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if len(req.Channels) == 0 {
		c.JSON(400, gin.H{"error": "At least one channel ID is required"})
		return
	}

	_, profileID, _, _, err := authhelp.GetSourceToken(
		c.Request.Context(),
		h.DB,
		h.Config.TokenEncryptionKey,
		sourceID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get source configuration"})
		return
	}

	parts := strings.Split(profileID, ":::")
	if len(parts) != 2 {
		c.JSON(500, gin.H{"error": "Invalid source configuration"})
		return
	}
	serverID := parts[0]

	cleanedChannels := make([]string, 0, len(req.Channels))
	for _, ch := range req.Channels {
		ch = strings.TrimSpace(ch)
		if ch != "" {
			cleanedChannels = append(cleanedChannels, ch)
		}
	}

	newProfileID := serverID + ":::" + strings.Join(cleanedChannels, ",")
	err = authhelp.UpdateSourceProfile(
		c.Request.Context(),
		h.DB,
		h.Config.TokenEncryptionKey,
		sourceID,
		newProfileID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to update channels: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":  true,
		"message":  "Channels updated successfully",
		"channels": cleanedChannels,
	})
}

func (h *Handler) GetSourceChannelsHandler(c *gin.Context) {
	sourceIDStr := c.Param("source_id")
	if sourceIDStr == "" {
		c.JSON(400, gin.H{"error": "source_id is required"})
		return
	}

	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid source ID"})
		return
	}

	source, err := h.DB.GetSourceById(context.Background(), sourceID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Source not found"})
		return
	}

	switch source.Network {
	case "Discord":
		channels, err := h.getDiscordChannels(c.Request.Context(), sourceID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, GetSourceChannelsResponse{
			Network:  "Discord",
			Channels: channels,
		})

	default:
		c.JSON(400, gin.H{
			"error": "This source type does not support multiple channels",
		})
	}
}

func (h *Handler) getDiscordChannels(ctx context.Context, sourceID uuid.UUID) ([]string, error) {
	source, err := h.DB.GetSourceById(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	if source.Network != "Discord" {
		return []string{}, nil
	}

	_, profileID, _, _, err := authhelp.GetSourceToken(
		ctx,
		h.DB,
		h.Config.TokenEncryptionKey,
		sourceID,
	)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(profileID, ":::")
	if len(parts) != 2 {
		return []string{}, nil
	}
	channelIDs := strings.Split(parts[1], ",")
	for i, id := range channelIDs {
		channelIDs[i] = strings.TrimSpace(id)
	}

	return channelIDs, nil
}

// SPDX-License-Identifier: AGPL-3.0-only
package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) HealthCheckHandler(c *gin.Context) {
	if h.DBConn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "failure", "details": "database connection not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.DBConn.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "failure", "details": "database ping failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

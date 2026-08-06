package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GetDedicatedImageMetrics returns bounded, process-local worker counters.
// Durable task state remains in PostgreSQL and is not replaced by this view.
// GET /api/v1/admin/ops/dedicated-image
func (h *OpsHandler) GetDedicatedImageMetrics(c *gin.Context) {
	if h == nil || h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, service.GetImageGenerationMetricsSnapshot())
}

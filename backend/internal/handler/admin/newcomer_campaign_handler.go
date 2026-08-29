package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// NewcomerCampaignHandler exposes the repeatable operator repair for the
// time-boxed newcomer campaign. Normal payment/redeem hooks remain the
// primary path; this endpoint is for backfill and reconciliation after an
// outage or a migration catch-up.
type NewcomerCampaignHandler struct {
	campaign *service.NewcomerCampaignService
}

func NewNewcomerCampaignHandler(campaign *service.NewcomerCampaignService) *NewcomerCampaignHandler {
	return &NewcomerCampaignHandler{campaign: campaign}
}

// Reconcile reruns payment-fact backfill and campaign state repair for all
// non-admin users. Every operation is keyed/idempotent and safe to repeat.
// POST /api/v1/admin/campaigns/newcomer/reconcile
func (h *NewcomerCampaignHandler) Reconcile(c *gin.Context) {
	if h == nil || h.campaign == nil {
		response.ErrorFrom(c, service.ErrServiceUnavailable)
		return
	}
	repaired, err := h.campaign.ReconcileAll(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"repaired_users": repaired})
}

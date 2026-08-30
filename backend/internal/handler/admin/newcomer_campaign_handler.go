package admin

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
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

// GetConfig returns the current operator-controlled tier settings.
// GET /api/v1/admin/campaigns/newcomer/config
func (h *NewcomerCampaignHandler) GetConfig(c *gin.Context) {
	if h == nil || h.campaign == nil {
		response.ErrorFrom(c, service.ErrServiceUnavailable)
		return
	}
	config, err := h.campaign.AdminGetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

type updateNewcomerCampaignConfigRequest struct {
	Tiers    []service.NewcomerCampaignTier `json:"tiers" binding:"required"`
	StartsAt *time.Time                     `json:"starts_at"`
	EndsAt   *time.Time                     `json:"ends_at"`
}

// UpdateConfig changes only future automatic grants. Existing grants keep
// their copied factor and expiry, so this endpoint cannot retroactively alter
// a user's historical entitlement.
// PUT /api/v1/admin/campaigns/newcomer/config
func (h *NewcomerCampaignHandler) UpdateConfig(c *gin.Context) {
	if h == nil || h.campaign == nil {
		response.ErrorFrom(c, service.ErrServiceUnavailable)
		return
	}
	var req updateNewcomerCampaignConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	actorID, ok := adminActorID(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	config, err := h.campaign.AdminUpdateConfigWithWindow(c.Request.Context(), actorID, req.Tiers, req.StartsAt, req.EndsAt)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

type setNewcomerCampaignMembershipRequest struct {
	TierKey      string     `json:"tier_key" binding:"required"`
	Factor       *float64   `json:"factor"`
	StartsAt     *time.Time `json:"starts_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
	DurationDays *int       `json:"duration_days"`
	Reason       string     `json:"reason"`
}

func parseNewcomerCampaignUserID(c *gin.Context) (int64, bool) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return 0, false
	}
	return userID, true
}

func adminActorID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	return subject.UserID, ok && subject.UserID > 0
}

// GetUserMembership returns the selected user's manual and effective
// membership state, including the valid invitation count.
// GET /api/v1/admin/campaigns/newcomer/users/:user_id/membership
func (h *NewcomerCampaignHandler) GetUserMembership(c *gin.Context) {
	if h == nil || h.campaign == nil {
		response.ErrorFrom(c, service.ErrServiceUnavailable)
		return
	}
	userID, ok := parseNewcomerCampaignUserID(c)
	if !ok {
		return
	}
	membership, err := h.campaign.AdminGetUserMembership(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, membership)
}

// SetUserMembership creates an auditable manual grant. It is independent of
// invitation qualification and takes precedence over an automatic grant while
// it is effective.
// PUT /api/v1/admin/campaigns/newcomer/users/:user_id/membership
func (h *NewcomerCampaignHandler) SetUserMembership(c *gin.Context) {
	if h == nil || h.campaign == nil {
		response.ErrorFrom(c, service.ErrServiceUnavailable)
		return
	}
	userID, ok := parseNewcomerCampaignUserID(c)
	if !ok {
		return
	}
	var req setNewcomerCampaignMembershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	actorID, ok := adminActorID(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	membership, err := h.campaign.AdminSetUserMembership(c.Request.Context(), actorID, userID, service.NewcomerCampaignAdminMembershipInput{
		TierKey:      req.TierKey,
		Factor:       req.Factor,
		StartsAt:     req.StartsAt,
		ExpiresAt:    req.ExpiresAt,
		DurationDays: req.DurationDays,
		Reason:       req.Reason,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, membership)
}

// ClearUserMembership revokes the active manual grant while retaining its
// history for audit. It does not remove or rewrite automatic campaign grants.
// DELETE /api/v1/admin/campaigns/newcomer/users/:user_id/membership
func (h *NewcomerCampaignHandler) ClearUserMembership(c *gin.Context) {
	if h == nil || h.campaign == nil {
		response.ErrorFrom(c, service.ErrServiceUnavailable)
		return
	}
	userID, ok := parseNewcomerCampaignUserID(c)
	if !ok {
		return
	}
	actorID, ok := adminActorID(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	membership, err := h.campaign.AdminClearUserMembership(c.Request.Context(), actorID, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, membership)
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

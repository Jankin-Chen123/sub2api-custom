package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CheckinHandler manages the complete lucky-wheel prize configuration.
type CheckinHandler struct {
	service *service.CheckinService
}

func NewCheckinHandler(checkinService *service.CheckinService) *CheckinHandler {
	return &CheckinHandler{service: checkinService}
}

func (h *CheckinHandler) ListPrizes(c *gin.Context) {
	prizes, err := h.service.ListPrizes(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prizes)
}

func (h *CheckinHandler) GetConfig(c *gin.Context) {
	config, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

type updateCheckinConfigRequest struct {
	// A pointer lets the validator distinguish an explicitly configured zero
	// (which disables the seven-day bonus) from an omitted field.
	StreakBonusAmount *float64 `json:"streak_bonus_amount" binding:"required,gte=0,lte=1000000"`
}

func (h *CheckinHandler) UpdateConfig(c *gin.Context) {
	var req updateCheckinConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	config, err := h.service.UpdateConfig(c.Request.Context(), *req.StreakBonusAmount)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

type replaceCheckinPrizesRequest struct {
	Prizes []service.CheckinPrize `json:"prizes" binding:"required"`
}

func (h *CheckinHandler) ReplacePrizes(c *gin.Context) {
	var req replaceCheckinPrizesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	prizes, err := h.service.ReplacePrizes(c.Request.Context(), req.Prizes)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prizes)
}

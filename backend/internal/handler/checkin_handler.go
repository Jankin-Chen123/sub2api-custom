package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CheckinHandler exposes the authenticated daily lucky-wheel APIs.
type CheckinHandler struct {
	service *service.CheckinService
}

func NewCheckinHandler(checkinService *service.CheckinService) *CheckinHandler {
	return &CheckinHandler{service: checkinService}
}

// GetStatus returns today's check-in state and wheel segments.
func (h *CheckinHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.service.GetStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// GetHistory returns the user's most recent check-in rewards.
// GET /api/v1/check-in/history
func (h *CheckinHandler) GetHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	history, err := h.service.ListHistory(c.Request.Context(), subject.UserID, 25)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, history)
}

// Draw performs today's single server-side draw and credits the reward.
func (h *CheckinHandler) Draw(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	record, err := h.service.Draw(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, record)
}

package handler

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/docsso"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// DocumentationTicket runs behind the existing admin authentication middleware.
// The browser keeps the main JWT; only this scoped, one-minute ticket goes to docs.
func (h *AuthHandler) DocumentationTicket(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if c.GetHeader("x-api-key") != "" || !strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") {
		response.Forbidden(c, "请使用主站管理员账号登录")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	role, _ := middleware.GetUserRoleFromContext(c)
	if !ok || h.cfg == nil {
		response.Unauthorized(c, "请重新登录主站")
		return
	}
	if role != "admin" || c.GetString("auth_method") != "jwt" {
		response.Forbidden(c, "请使用主站管理员账号登录")
		return
	}
	ticket, err := docsso.Issue(h.cfg.JWT.Secret, subject.UserID, time.Now())
	if err != nil {
		response.Error(c, 503, "文档免登录尚未配置")
		return
	}
	response.Success(c, gin.H{"ticket": ticket, "expires_in": int(docsso.Lifetime.Seconds())})
}

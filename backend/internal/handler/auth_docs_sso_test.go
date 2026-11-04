package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func TestDocumentationTicketRequiresAdminJWT(t *testing.T) {
	for _, tc := range []struct {
		name, role, method, key, secret string
		userID                          int64
		want                            int
	}{
		{"admin", "admin", "jwt", "", "docs-sso-fixture-secret-at-least-32-characters", 42, 200},
		{"user", "user", "jwt", "", "docs-sso-fixture-secret-at-least-32-characters", 42, 403},
		{"anonymous", "", "", "", "docs-sso-fixture-secret-at-least-32-characters", 0, 401},
		{"api key", "admin", "api_key", "example-key", "docs-sso-fixture-secret-at-least-32-characters", 42, 403},
		{"weak signing key", "admin", "jwt", "", "short", 42, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/api/v1/admin/integrations/docs-session", nil)
			c.Request.Header.Set("Authorization", "Bearer test-main-jwt")
			c.Request.Header.Set("x-api-key", tc.key)
			if tc.userID > 0 {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: tc.userID})
			}
			c.Set(string(middleware.ContextKeyUserRole), tc.role)
			c.Set("auth_method", tc.method)
			h := &AuthHandler{cfg: &config.Config{JWT: config.JWTConfig{Secret: tc.secret}}}
			h.DocumentationTicket(c)
			if w.Code != tc.want {
				t.Fatalf("status %d, want %d", w.Code, tc.want)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("credential response can be cached")
			}
		})
	}
}

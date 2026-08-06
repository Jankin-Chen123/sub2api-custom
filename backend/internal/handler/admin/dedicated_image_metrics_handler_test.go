package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDedicatedImageMetricsUnavailableWithoutOpsService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/dedicated-image", nil)

	(&OpsHandler{}).GetDedicatedImageMetrics(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

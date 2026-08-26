//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountHealthViewRepoStub struct {
	service.ChannelMonitorRepository
	monitor   *service.ChannelMonitor
	snapshots []*service.ChannelMonitorAccountHealthSnapshot
}

func (r *accountHealthViewRepoStub) GetByID(_ context.Context, id int64) (*service.ChannelMonitor, error) {
	if r.monitor == nil || r.monitor.ID != id {
		return nil, service.ErrChannelMonitorNotFound
	}
	return r.monitor, nil
}

func (r *accountHealthViewRepoStub) ListAccountHealthSnapshotsForMonitor(_ context.Context, _ int64, _ string, _ []string, _ string, _ int) ([]*service.ChannelMonitorAccountHealthSnapshot, error) {
	return r.snapshots, nil
}

func setupAccountHealthRouter(t *testing.T) *gin.Engine {
	t.Helper()
	expires := time.Now().UTC().Add(-time.Minute)
	repo := &accountHealthViewRepoStub{
		monitor: &service.ChannelMonitor{
			ID:           42,
			Provider:     service.MonitorProviderOpenAI,
			PrimaryModel: "gpt-test",
			ExtraModels:  []string{"gpt-extra"},
		},
		snapshots: []*service.ChannelMonitorAccountHealthSnapshot{
			{
				GroupID: 9, AccountID: 17, AccountName: "upstream-a", Provider: service.PlatformOpenAI,
				Model: "gpt-test", Score: 82.5, HealthState: service.ChannelMonitorHealthStateHealthy,
				EWMASuccessRate: 0.95, SampleCount: 8, LastStatus: service.MonitorStatusOperational,
				LastProbeAt: expires, UpdatedAt: expires, ExpiresAt: expires,
			},
		},
	}
	handler := NewChannelMonitorHandler(service.NewChannelMonitorService(repo, nil))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/admin/channel-monitors/:id/account-health", handler.AccountHealth)
	return router
}

func TestChannelMonitorAccountHealthHandlerMarksExpiredSnapshotUnknown(t *testing.T) {
	router := setupAccountHealthRouter(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/channel-monitors/42/account-health", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"account_name":"upstream-a"`)
	require.Contains(t, recorder.Body.String(), `"score":82.5`)
	require.Contains(t, recorder.Body.String(), `"health_state":"unknown"`)
	require.Contains(t, recorder.Body.String(), `"stale":true`)
}

func TestChannelMonitorAccountHealthHandlerRejectsUnknownModel(t *testing.T) {
	router := setupAccountHealthRouter(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/channel-monitors/42/account-health?model=not-configured", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"items":[]`)
}

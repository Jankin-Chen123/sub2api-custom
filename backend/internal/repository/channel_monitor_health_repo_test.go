package repository

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorRepositoryApplyAccountProbeResultsAdvancesSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	checkedAt := time.Now().UTC().Truncate(time.Millisecond)
	row := &service.ChannelMonitorAccountProbeResult{
		MonitorID: 1,
		GroupID:   9,
		AccountID: 17,
		Model:     "gpt-test",
		Provider:  service.PlatformOpenAI,
		Status:    service.MonitorStatusOperational,
		LatencyMs: intPtrForHealthRepoTest(250),
		CheckedAt: checkedAt,
	}

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO channel_monitor_account_probe_results").
		ExpectExec().
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT group_id, account_id, provider, model, score, health_state").
		WithArgs(int64(9), int64(17), service.PlatformOpenAI, "gpt-test").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO channel_monitor_account_health_snapshots").
		WithArgs(int64(9), int64(17), service.PlatformOpenAI, "gpt-test", sqlmock.AnyArg(), service.ChannelMonitorHealthStateUnknown, sqlmock.AnyArg(), sqlmock.AnyArg(), 1, 1, 0, service.MonitorStatusOperational, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := &channelMonitorRepository{db: db}
	snapshots, err := repo.ApplyAccountProbeResults(t.Context(), []*service.ChannelMonitorAccountProbeResult{row})
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Equal(t, 1, snapshots[0].SampleCount)
	require.Equal(t, service.ChannelMonitorHealthStateUnknown, snapshots[0].HealthState)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelMonitorRepositoryListAccountHealthSnapshots(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	checkedAt := time.Now().UTC()
	mock.ExpectQuery("SELECT group_id, account_id, provider, model, score, health_state").
		WithArgs(int64(9), service.PlatformOpenAI, "gpt-test", 100).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id", "account_id", "provider", "model", "score", "health_state",
			"ewma_success_rate", "ewma_latency_ms", "sample_count", "consecutive_successes",
			"consecutive_failures", "last_status", "last_probe_at", "updated_at", "expires_at",
		}).AddRow(9, 17, service.PlatformOpenAI, "gpt-test", 75.0, service.ChannelMonitorHealthStateHealthy,
			0.9, 250, 5, 4, 0, service.MonitorStatusOperational, checkedAt, checkedAt, checkedAt.Add(time.Minute)))

	repo := &channelMonitorRepository{db: db}
	snapshots, err := repo.ListAccountHealthSnapshots(t.Context(), int64PtrForHealthRepoTest(9), service.PlatformOpenAI, "gpt-test", 100)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Equal(t, int64(17), snapshots[0].AccountID)
	require.Equal(t, service.ChannelMonitorHealthStateHealthy, snapshots[0].HealthState)
	require.Equal(t, 250, *snapshots[0].EWMALatencyMs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelMonitorRepositoryListAccountHealthSnapshotsForMonitorScopesProbe(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	checkedAt := time.Now().UTC()
	mock.ExpectQuery("SELECT s.group_id, s.account_id, COALESCE\\(a.name, ''\\)").
		WithArgs(int64(42), service.PlatformOpenAI, sqlmock.AnyArg(), "", 1000).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id", "account_id", "account_name", "provider", "model", "score", "health_state",
			"ewma_success_rate", "ewma_latency_ms", "sample_count", "consecutive_successes",
			"consecutive_failures", "last_status", "last_probe_at", "updated_at", "expires_at",
		}).AddRow(9, 17, "upstream-a", service.PlatformOpenAI, "gpt-test", 82.5, service.ChannelMonitorHealthStateHealthy,
			0.95, 180, 8, 7, 0, service.MonitorStatusOperational, checkedAt, checkedAt, checkedAt.Add(time.Minute)))

	repo := &channelMonitorRepository{db: db}
	snapshots, err := repo.ListAccountHealthSnapshotsForMonitor(t.Context(), 42, service.PlatformOpenAI, []string{"gpt-test"}, "", 1000)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Equal(t, "upstream-a", snapshots[0].AccountName)
	require.Equal(t, int64(17), snapshots[0].AccountID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func intPtrForHealthRepoTest(value int) *int {
	return &value
}

func int64PtrForHealthRepoTest(value int64) *int64 {
	return &value
}

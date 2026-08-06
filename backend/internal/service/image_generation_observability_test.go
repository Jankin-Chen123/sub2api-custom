package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestImageGenerationMetricsSnapshotContainsOnlyOperationalCounters(t *testing.T) {
	resetImageGenerationMetricsForTest()
	t.Cleanup(resetImageGenerationMetricsForTest)

	recordImageGenerationCreated(false)
	recordImageGenerationCreated(true)
	recordImageGenerationClaimed()
	recordImageGenerationSubmission()
	recordImageGenerationPoll()
	recordImageGenerationRetry()
	recordImageGenerationProviderUnavailable()
	recordImageGenerationAccountSelection(true)
	recordImageGenerationAccountSelection(false)
	recordImageGenerationUpstreamError()
	recordImageGenerationUpstreamLatency(time.Now().Add(-time.Millisecond))
	recordImageGenerationStorageFailure()
	recordImageGenerationSettlementFailure()
	recordImageGenerationClaimLeaseRenewalFailure()
	recordImageGenerationTerminal(ImageGenerationJobStatusCompleted)
	recordImageGenerationTerminal(ImageGenerationJobStatusFailed)
	recordImageGenerationTerminal(ImageGenerationJobStatusSubmissionUnknown)
	recordImageGenerationClaimFinished()

	snapshot := GetImageGenerationMetricsSnapshot()
	require.Equal(t, uint64(1), snapshot.CreatedTotal)
	require.Equal(t, uint64(1), snapshot.ReplayTotal)
	require.Equal(t, int64(0), snapshot.ActiveJobs)
	require.Equal(t, int64(0), snapshot.PendingJobsObserved)
	require.Equal(t, uint64(1), snapshot.ClaimsTotal)
	require.Equal(t, uint64(1), snapshot.SubmissionsTotal)
	require.Equal(t, uint64(1), snapshot.PollsTotal)
	require.Equal(t, uint64(1), snapshot.RetriesTotal)
	require.Equal(t, uint64(1), snapshot.ProviderUnavailableTotal)
	require.Equal(t, uint64(1), snapshot.ImageOnlySelectionsTotal)
	require.Equal(t, uint64(1), snapshot.GeneralFallbackSelectionsTotal)
	require.Equal(t, uint64(1), snapshot.UpstreamErrorsTotal)
	require.Equal(t, uint64(1), snapshot.UpstreamLatencyCount)
	require.Greater(t, snapshot.UpstreamLatencyMicrosTotal, uint64(0))
	require.Equal(t, uint64(1), snapshot.StorageFailuresTotal)
	require.Equal(t, uint64(1), snapshot.SettlementFailuresTotal)
	require.Equal(t, uint64(1), snapshot.ClaimLeaseRenewalFailures)
	require.Equal(t, uint64(1), snapshot.CompletedTotal)
	require.Equal(t, uint64(1), snapshot.FailedTotal)
	require.Equal(t, uint64(1), snapshot.SubmissionUnknownTotal)

	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, strings.ToLower(string(raw)), "prompt")
	require.NotContains(t, strings.ToLower(string(raw)), "authorization")
	require.NotContains(t, strings.ToLower(string(raw)), "task_id")
}

func TestImageGenerationMetricsTerminalDoesNotCreateNegativeObservedQueue(t *testing.T) {
	resetImageGenerationMetricsForTest()
	t.Cleanup(resetImageGenerationMetricsForTest)

	recordImageGenerationTerminal(ImageGenerationJobStatusFailed)

	require.Zero(t, GetImageGenerationMetricsSnapshot().PendingJobsObserved)
}

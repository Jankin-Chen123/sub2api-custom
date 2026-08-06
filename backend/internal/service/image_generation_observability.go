package service

import (
	"sync/atomic"
	"time"
)

// ImageGenerationMetricsSnapshot is a process-local operational view of the
// durable image worker. It intentionally contains only bounded counters and
// timings: prompts, credentials, object references, upstream URLs and task
// IDs must never be added here.
type ImageGenerationMetricsSnapshot struct {
	CreatedTotal                   uint64 `json:"created_total"`
	ReplayTotal                    uint64 `json:"replay_total"`
	ActiveJobs                     int64  `json:"active_jobs"`
	PendingJobsObserved            int64  `json:"pending_jobs_observed"`
	ClaimsTotal                    uint64 `json:"claims_total"`
	ClaimLeaseRenewalFailures      uint64 `json:"claim_lease_renewal_failures"`
	SubmissionsTotal               uint64 `json:"submissions_total"`
	PollsTotal                     uint64 `json:"polls_total"`
	RetriesTotal                   uint64 `json:"retries_total"`
	ProviderUnavailableTotal       uint64 `json:"provider_unavailable_total"`
	ImageOnlySelectionsTotal       uint64 `json:"image_only_selections_total"`
	GeneralFallbackSelectionsTotal uint64 `json:"general_fallback_selections_total"`
	UpstreamErrorsTotal            uint64 `json:"upstream_errors_total"`
	UpstreamLatencyCount           uint64 `json:"upstream_latency_count"`
	UpstreamLatencyMicrosTotal     uint64 `json:"upstream_latency_micros_total"`
	StorageFailuresTotal           uint64 `json:"storage_failures_total"`
	SettlementFailuresTotal        uint64 `json:"settlement_failures_total"`
	CompletedTotal                 uint64 `json:"completed_total"`
	FailedTotal                    uint64 `json:"failed_total"`
	SubmissionUnknownTotal         uint64 `json:"submission_unknown_total"`
}

type imageGenerationMetrics struct {
	createdTotal                   atomic.Uint64
	replayTotal                    atomic.Uint64
	activeJobs                     atomic.Int64
	pendingJobsObserved            atomic.Int64
	claimsTotal                    atomic.Uint64
	claimLeaseRenewalFailures      atomic.Uint64
	submissionsTotal               atomic.Uint64
	pollsTotal                     atomic.Uint64
	retriesTotal                   atomic.Uint64
	providerUnavailableTotal       atomic.Uint64
	imageOnlySelectionsTotal       atomic.Uint64
	generalFallbackSelectionsTotal atomic.Uint64
	upstreamErrorsTotal            atomic.Uint64
	upstreamLatencyCount           atomic.Uint64
	upstreamLatencyMicros          atomic.Uint64
	storageFailuresTotal           atomic.Uint64
	settlementFailuresTotal        atomic.Uint64
	completedTotal                 atomic.Uint64
	failedTotal                    atomic.Uint64
	submissionUnknownTotal         atomic.Uint64
}

var defaultImageGenerationMetrics imageGenerationMetrics

// GetImageGenerationMetricsSnapshot returns process-local counters. The
// durable job table remains authoritative after a restart or metrics loss.
func GetImageGenerationMetricsSnapshot() ImageGenerationMetricsSnapshot {
	return ImageGenerationMetricsSnapshot{
		CreatedTotal:                   defaultImageGenerationMetrics.createdTotal.Load(),
		ReplayTotal:                    defaultImageGenerationMetrics.replayTotal.Load(),
		ActiveJobs:                     defaultImageGenerationMetrics.activeJobs.Load(),
		PendingJobsObserved:            defaultImageGenerationMetrics.pendingJobsObserved.Load(),
		ClaimsTotal:                    defaultImageGenerationMetrics.claimsTotal.Load(),
		ClaimLeaseRenewalFailures:      defaultImageGenerationMetrics.claimLeaseRenewalFailures.Load(),
		SubmissionsTotal:               defaultImageGenerationMetrics.submissionsTotal.Load(),
		PollsTotal:                     defaultImageGenerationMetrics.pollsTotal.Load(),
		RetriesTotal:                   defaultImageGenerationMetrics.retriesTotal.Load(),
		ProviderUnavailableTotal:       defaultImageGenerationMetrics.providerUnavailableTotal.Load(),
		ImageOnlySelectionsTotal:       defaultImageGenerationMetrics.imageOnlySelectionsTotal.Load(),
		GeneralFallbackSelectionsTotal: defaultImageGenerationMetrics.generalFallbackSelectionsTotal.Load(),
		UpstreamErrorsTotal:            defaultImageGenerationMetrics.upstreamErrorsTotal.Load(),
		UpstreamLatencyCount:           defaultImageGenerationMetrics.upstreamLatencyCount.Load(),
		UpstreamLatencyMicrosTotal:     defaultImageGenerationMetrics.upstreamLatencyMicros.Load(),
		StorageFailuresTotal:           defaultImageGenerationMetrics.storageFailuresTotal.Load(),
		SettlementFailuresTotal:        defaultImageGenerationMetrics.settlementFailuresTotal.Load(),
		CompletedTotal:                 defaultImageGenerationMetrics.completedTotal.Load(),
		FailedTotal:                    defaultImageGenerationMetrics.failedTotal.Load(),
		SubmissionUnknownTotal:         defaultImageGenerationMetrics.submissionUnknownTotal.Load(),
	}
}

func recordImageGenerationCreated(replayed bool) {
	if replayed {
		defaultImageGenerationMetrics.replayTotal.Add(1)
		return
	}
	defaultImageGenerationMetrics.createdTotal.Add(1)
	defaultImageGenerationMetrics.pendingJobsObserved.Add(1)
}

func recordImageGenerationClaimed() {
	defaultImageGenerationMetrics.claimsTotal.Add(1)
	defaultImageGenerationMetrics.activeJobs.Add(1)
}

func recordImageGenerationClaimFinished() {
	defaultImageGenerationMetrics.activeJobs.Add(-1)
}

func recordImageGenerationClaimLeaseRenewalFailure() {
	defaultImageGenerationMetrics.claimLeaseRenewalFailures.Add(1)
}

func recordImageGenerationSubmission() {
	defaultImageGenerationMetrics.submissionsTotal.Add(1)
}

func recordImageGenerationPoll() {
	defaultImageGenerationMetrics.pollsTotal.Add(1)
}

func recordImageGenerationRetry() {
	defaultImageGenerationMetrics.retriesTotal.Add(1)
}

func recordImageGenerationProviderUnavailable() {
	defaultImageGenerationMetrics.providerUnavailableTotal.Add(1)
}

func recordImageGenerationAccountSelection(imageOnly bool) {
	if imageOnly {
		defaultImageGenerationMetrics.imageOnlySelectionsTotal.Add(1)
		return
	}
	defaultImageGenerationMetrics.generalFallbackSelectionsTotal.Add(1)
}

func recordImageGenerationUpstreamError() {
	defaultImageGenerationMetrics.upstreamErrorsTotal.Add(1)
}

func recordImageGenerationUpstreamLatency(start time.Time) {
	duration := time.Since(start)
	if duration < 0 {
		return
	}
	defaultImageGenerationMetrics.upstreamLatencyCount.Add(1)
	defaultImageGenerationMetrics.upstreamLatencyMicros.Add(uint64(duration.Microseconds()))
}

func recordImageGenerationStorageFailure() {
	defaultImageGenerationMetrics.storageFailuresTotal.Add(1)
}

func recordImageGenerationSettlementFailure() {
	defaultImageGenerationMetrics.settlementFailuresTotal.Add(1)
}

func recordImageGenerationTerminal(status string) {
	for {
		pending := defaultImageGenerationMetrics.pendingJobsObserved.Load()
		if pending <= 0 || defaultImageGenerationMetrics.pendingJobsObserved.CompareAndSwap(pending, pending-1) {
			break
		}
	}
	switch status {
	case ImageGenerationJobStatusCompleted:
		defaultImageGenerationMetrics.completedTotal.Add(1)
	case ImageGenerationJobStatusFailed:
		defaultImageGenerationMetrics.failedTotal.Add(1)
	case ImageGenerationJobStatusSubmissionUnknown:
		defaultImageGenerationMetrics.submissionUnknownTotal.Add(1)
	}
}

// resetImageGenerationMetricsForTest is intentionally private so production
// code cannot reset operational history through an HTTP endpoint.
func resetImageGenerationMetricsForTest() {
	defaultImageGenerationMetrics.createdTotal.Store(0)
	defaultImageGenerationMetrics.replayTotal.Store(0)
	defaultImageGenerationMetrics.activeJobs.Store(0)
	defaultImageGenerationMetrics.pendingJobsObserved.Store(0)
	defaultImageGenerationMetrics.claimsTotal.Store(0)
	defaultImageGenerationMetrics.claimLeaseRenewalFailures.Store(0)
	defaultImageGenerationMetrics.submissionsTotal.Store(0)
	defaultImageGenerationMetrics.pollsTotal.Store(0)
	defaultImageGenerationMetrics.retriesTotal.Store(0)
	defaultImageGenerationMetrics.providerUnavailableTotal.Store(0)
	defaultImageGenerationMetrics.imageOnlySelectionsTotal.Store(0)
	defaultImageGenerationMetrics.generalFallbackSelectionsTotal.Store(0)
	defaultImageGenerationMetrics.upstreamErrorsTotal.Store(0)
	defaultImageGenerationMetrics.upstreamLatencyCount.Store(0)
	defaultImageGenerationMetrics.upstreamLatencyMicros.Store(0)
	defaultImageGenerationMetrics.storageFailuresTotal.Store(0)
	defaultImageGenerationMetrics.settlementFailuresTotal.Store(0)
	defaultImageGenerationMetrics.completedTotal.Store(0)
	defaultImageGenerationMetrics.failedTotal.Store(0)
	defaultImageGenerationMetrics.submissionUnknownTotal.Store(0)
}

package service

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// monitorAccountProbeConcurrency bounds upstream fan-out inside one model
	// round. It is intentionally independent from the monitor worker pool.
	monitorAccountProbeConcurrency = 4
	// monitorAccountProbeTimeout keeps a stuck account from holding a batch
	// forever while retaining a generous single-account network budget.
	monitorAccountProbeTimeout = monitorRequestTimeout
	// monitorAccountProbeRoundTimeout bounds the whole account round. The
	// account count is not used as an unbounded multiplier: accounts run in
	// fixed-size batches and the round has one explicit deadline.
	monitorAccountProbeRoundTimeout = 90 * time.Second
)

const monitorAccountProbeStatusSkipped = "skipped"

type channelMonitorAccountAttempt func(
	context.Context,
	*Account,
	ChannelMonitorAccountProbeRequest,
) *CheckResult

// runChannelMonitorAccountProbes probes every applicable account exactly once
// in bounded batches. It is intentionally independent from endpoint/key
// resolution and forwarding so the selection/aggregation semantics can be
// tested without network calls.
func runChannelMonitorAccountProbes(
	ctx context.Context,
	request ChannelMonitorAccountProbeRequest,
	accounts []Account,
	attempt channelMonitorAccountAttempt,
) *ChannelMonitorAccountProbeRun {
	ordered := append([]Account(nil), accounts...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].ID < ordered[j].ID
	})
	if len(ordered) > 1 {
		unique := ordered[:0]
		seen := make(map[int64]struct{}, len(ordered))
		for i := range ordered {
			if _, exists := seen[ordered[i].ID]; exists {
				continue
			}
			seen[ordered[i].ID] = struct{}{}
			unique = append(unique, ordered[i])
		}
		ordered = unique
	}

	started := time.Now()
	rows := make([]*ChannelMonitorAccountProbeResult, len(ordered))
	applicable := 0

	for batchStart := 0; batchStart < len(ordered); batchStart += monitorAccountProbeConcurrency {
		batchEnd := batchStart + monitorAccountProbeConcurrency
		if batchEnd > len(ordered) {
			batchEnd = len(ordered)
		}

		var wg sync.WaitGroup
		for i := batchStart; i < batchEnd; i++ {
			account := &ordered[i]
			if reason := accountProbeSkipReason(account, request.Model); reason != "" {
				rows[i] = skippedAccountProbeResult(request, account, reason)
				continue
			}
			if err := ctx.Err(); err != nil {
				rows[i] = skippedAccountProbeResult(request, account, "context canceled before probe")
				continue
			}

			applicable++
			wg.Add(1)
			go func(index int, candidate *Account) {
				defer wg.Done()
				attemptCtx, cancel := context.WithTimeout(ctx, monitorAccountProbeTimeout)
				defer cancel()
				result := attempt(attemptCtx, candidate, request)
				if result == nil {
					result = &CheckResult{
						Model:     request.Model,
						Status:    MonitorStatusError,
						CheckedAt: time.Now(),
						Message:   "account probe returned no result",
					}
				}
				rows[index] = accountProbeResultFromCheck(request, candidate, result)
			}(i, account)
		}
		wg.Wait()

		if err := ctx.Err(); err != nil {
			for i := batchEnd; i < len(ordered); i++ {
				if rows[i] == nil {
					rows[i] = skippedAccountProbeResult(request, &ordered[i], "context canceled before probe")
				}
			}
			break
		}
	}

	roundDurationMs := int(time.Since(started) / time.Millisecond)
	for _, row := range rows {
		if row != nil {
			row.RoundDurationMs = roundDurationMs
		}
	}

	aggregate := aggregateChannelMonitorAccountResults(request.Model, rows, applicable)
	return &ChannelMonitorAccountProbeRun{Results: rows, Aggregate: aggregate}
}

func accountProbeSkipReason(account *Account, model string) string {
	if account == nil {
		return "account not found"
	}
	if !account.IsActive() {
		return "account is not active"
	}
	if !account.Schedulable {
		return "account scheduling is disabled"
	}
	if !account.IsModelSupported(model) {
		return "model is not supported by account"
	}
	if !account.IsSchedulable() {
		return "account is temporarily unavailable"
	}
	return ""
}

func skippedAccountProbeResult(
	request ChannelMonitorAccountProbeRequest,
	account *Account,
	reason string,
) *ChannelMonitorAccountProbeResult {
	return &ChannelMonitorAccountProbeResult{
		MonitorID:  monitorIDFromProbeRequest(request),
		GroupID:    groupIDFromProbeRequest(request),
		AccountID:  accountIDFromProbeAccount(account),
		Model:      request.Model,
		Provider:   probeProviderFromRequest(request),
		Status:     monitorAccountProbeStatusSkipped,
		CheckedAt:  time.Now(),
		Skipped:    true,
		SkipReason: reason,
	}
}

func accountProbeResultFromCheck(
	request ChannelMonitorAccountProbeRequest,
	account *Account,
	result *CheckResult,
) *ChannelMonitorAccountProbeResult {
	return &ChannelMonitorAccountProbeResult{
		MonitorID: monitorIDFromProbeRequest(request),
		GroupID:   groupIDFromProbeRequest(request),
		AccountID: accountIDFromProbeAccount(account),
		Model:     request.Model,
		Provider:  probeProviderFromRequest(request),
		Status:    result.Status,
		LatencyMs: result.LatencyMs,
		Message:   truncateMessage(sanitizeErrorMessage(result.Message)),
		CheckedAt: result.CheckedAt,
	}
}

func aggregateChannelMonitorAccountResults(model string, rows []*ChannelMonitorAccountProbeResult, applicable int) *CheckResult {
	aggregate := &CheckResult{
		Model:     model,
		Status:    MonitorStatusError,
		CheckedAt: time.Now(),
	}
	if applicable == 0 {
		aggregate.Message = "no applicable accounts for model"
		return aggregate
	}

	var bestOperational *ChannelMonitorAccountProbeResult
	var bestDegraded *ChannelMonitorAccountProbeResult
	var bestFailure *ChannelMonitorAccountProbeResult
	for _, row := range rows {
		if row == nil || row.Skipped {
			continue
		}
		switch row.Status {
		case MonitorStatusOperational:
			if bestOperational == nil || probeLatencyLess(row, bestOperational) {
				bestOperational = row
			}
		case MonitorStatusDegraded:
			if bestDegraded == nil || probeLatencyLess(row, bestDegraded) {
				bestDegraded = row
			}
		case MonitorStatusFailed, MonitorStatusError:
			if bestFailure == nil || moreDiagnosticProbeFailure(row, bestFailure) {
				bestFailure = row
			}
		}
	}

	bestSuccess := bestOperational
	if bestSuccess == nil {
		bestSuccess = bestDegraded
	}
	if bestSuccess != nil {
		aggregate.Status = bestSuccess.Status
		aggregate.LatencyMs = bestSuccess.LatencyMs
		aggregate.Message = bestSuccess.Message
		return aggregate
	}
	if bestFailure != nil {
		aggregate.Status = bestFailure.Status
		aggregate.LatencyMs = bestFailure.LatencyMs
		aggregate.Message = bestFailure.Message
		if strings.TrimSpace(aggregate.Message) == "" {
			aggregate.Message = "all applicable account probes failed"
		}
		return aggregate
	}

	aggregate.Message = "no account probe completed"
	return aggregate
}

func probeLatencyLess(a, b *ChannelMonitorAccountProbeResult) bool {
	if a == nil || a.LatencyMs == nil {
		return false
	}
	if b == nil || b.LatencyMs == nil {
		return true
	}
	return *a.LatencyMs < *b.LatencyMs
}

func moreDiagnosticProbeFailure(a, b *ChannelMonitorAccountProbeResult) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	if a.Status == MonitorStatusError && b.Status != MonitorStatusError {
		return true
	}
	if a.Status != MonitorStatusError && b.Status == MonitorStatusError {
		return false
	}
	return len(a.Message) > len(b.Message)
}

func monitorIDFromProbeRequest(request ChannelMonitorAccountProbeRequest) int64 {
	if request.Monitor == nil {
		return 0
	}
	return request.Monitor.ID
}

func groupIDFromProbeRequest(request ChannelMonitorAccountProbeRequest) int64 {
	return request.GroupID
}

func probeProviderFromRequest(request ChannelMonitorAccountProbeRequest) string {
	if request.Monitor == nil {
		return ""
	}
	return request.Monitor.Provider
}

func accountIDFromProbeAccount(account *Account) int64 {
	if account == nil {
		return 0
	}
	return account.ID
}

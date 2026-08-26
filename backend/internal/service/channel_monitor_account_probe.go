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
	// monitorAccountProbeRoundBuffer covers queueing, response aggregation and
	// the shared ping without making the budget depend on unbounded account
	// count. The account portion is calculated from the number of concurrent
	// waves below.
	monitorAccountProbeRoundBuffer = 10 * time.Second
	// monitorAccountProbeRunnerWatchdog is only a final safety net for a
	// malformed/non-cooperative service implementation. Normal account rounds
	// use monitorAccountProbeRoundTimeoutForCount instead.
	monitorAccountProbeRunnerWatchdog = 10 * time.Minute
)

const monitorAccountProbeStatusSkipped = "skipped"

type channelMonitorAccountAttempt func(
	context.Context,
	*Account,
	ChannelMonitorAccountProbeRequest,
) *CheckResult

type channelMonitorAccountAttemptResult struct {
	index  int
	result *CheckResult
}

// runChannelMonitorAccountProbes probes every applicable account exactly once
// with a bounded worker queue. A completed worker immediately takes the next
// account; there is deliberately no "wait for the whole batch" barrier, so a
// slow account cannot starve every account after it. It is intentionally
// independent from endpoint/key resolution and forwarding so the
// selection/aggregation semantics can be tested without network calls.
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
	pending := make([]int, 0, len(ordered))
	for i := range ordered {
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
		pending = append(pending, i)
	}

	if len(pending) > 0 {
		attemptResults := make(chan channelMonitorAccountAttemptResult, len(pending))
		work := struct {
			sync.Mutex
			next    int
			stopped bool
			started map[int]struct{}
		}{started: make(map[int]struct{}, len(pending))}

		workerCount := monitorAccountProbeConcurrency
		if workerCount > len(pending) {
			workerCount = len(pending)
		}
		for worker := 0; worker < workerCount; worker++ {
			go func() {
				for {
					work.Lock()
					if work.stopped || work.next >= len(pending) || ctx.Err() != nil {
						work.stopped = true
						work.Unlock()
						return
					}
					index := pending[work.next]
					work.next++
					work.started[index] = struct{}{}
					work.Unlock()

					result := runChannelMonitorAccountAttempt(ctx, request, &ordered[index], attempt)
					attemptResults <- channelMonitorAccountAttemptResult{index: index, result: result}
				}
			}()
		}

		completed := 0
		for completed < len(pending) {
			select {
			case attemptResult := <-attemptResults:
				rows[attemptResult.index] = accountProbeResultFromCheck(request, &ordered[attemptResult.index], attemptResult.result)
				completed++
			case <-ctx.Done():
				work.Lock()
				work.stopped = true
				startedIndexes := make(map[int]struct{}, len(work.started))
				for index := range work.started {
					startedIndexes[index] = struct{}{}
				}
				work.Unlock()

				// Keep results that were already delivered before cancellation.
				for {
					select {
					case attemptResult := <-attemptResults:
						if rows[attemptResult.index] == nil {
							rows[attemptResult.index] = accountProbeResultFromCheck(request, &ordered[attemptResult.index], attemptResult.result)
						}
					default:
						for _, index := range pending {
							if rows[index] != nil {
								continue
							}
							if _, started := startedIndexes[index]; started {
								rows[index] = canceledAccountProbeResult(request, &ordered[index], ctx.Err())
							} else {
								rows[index] = skippedAccountProbeResult(request, &ordered[index], "context canceled before probe")
							}
						}
						completed = len(pending)
					}
					if completed == len(pending) {
						break
					}
				}
				break
			}
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

func runChannelMonitorAccountAttempt(
	ctx context.Context,
	request ChannelMonitorAccountProbeRequest,
	account *Account,
	attempt channelMonitorAccountAttempt,
) (result *CheckResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = &CheckResult{
				Model:     request.Model,
				Status:    MonitorStatusError,
				CheckedAt: time.Now(),
				Message:   "account probe panicked",
			}
		}
		if result == nil {
			result = &CheckResult{
				Model:     request.Model,
				Status:    MonitorStatusError,
				CheckedAt: time.Now(),
				Message:   "account probe returned no result",
			}
		}
	}()

	attemptCtx, cancel := context.WithTimeout(ctx, monitorAccountProbeTimeout)
	defer cancel()
	if attempt == nil {
		return &CheckResult{
			Model:     request.Model,
			Status:    MonitorStatusError,
			CheckedAt: time.Now(),
			Message:   "account probe function is not configured",
		}
	}
	return attempt(attemptCtx, account, request)
}

func monitorAccountProbeRoundTimeoutForCount(accountCount int) time.Duration {
	if accountCount <= 0 {
		return monitorAccountProbeRoundBuffer
	}
	waves := (accountCount + monitorAccountProbeConcurrency - 1) / monitorAccountProbeConcurrency
	return time.Duration(waves)*monitorAccountProbeTimeout + monitorAccountProbeRoundBuffer
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

func canceledAccountProbeResult(
	request ChannelMonitorAccountProbeRequest,
	account *Account,
	err error,
) *ChannelMonitorAccountProbeResult {
	message := "account probe canceled"
	if err != nil {
		message += ": " + err.Error()
	}
	return &ChannelMonitorAccountProbeResult{
		MonitorID: monitorIDFromProbeRequest(request),
		GroupID:   groupIDFromProbeRequest(request),
		AccountID: accountIDFromProbeAccount(account),
		Model:     request.Model,
		Provider:  probeProviderFromRequest(request),
		Status:    MonitorStatusError,
		CheckedAt: time.Now(),
		Message:   message,
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

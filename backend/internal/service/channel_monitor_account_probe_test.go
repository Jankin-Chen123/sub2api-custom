package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestRunChannelMonitorAccountProbes_ProbesEachDistinctAccountAndAggregatesOperational(t *testing.T) {
	monitor := &ChannelMonitor{ID: 7, Provider: MonitorProviderOpenAI}
	request := ChannelMonitorAccountProbeRequest{Monitor: monitor, GroupID: 9, Model: "gpt-test"}
	accounts := []Account{
		probeTestAccount(4), probeTestAccount(2), probeTestAccount(3), probeTestAccount(1), probeTestAccount(2),
	}
	statuses := map[int64]string{
		1: MonitorStatusFailed,
		2: MonitorStatusError,
		3: MonitorStatusOperational,
		4: MonitorStatusFailed,
	}
	var mu sync.Mutex
	called := make(map[int64]int)
	attempt := func(_ context.Context, account *Account, _ ChannelMonitorAccountProbeRequest) *CheckResult {
		mu.Lock()
		called[account.ID]++
		mu.Unlock()
		latency := int(account.ID) * 10
		return &CheckResult{Model: "gpt-test", Status: statuses[account.ID], LatencyMs: &latency, CheckedAt: time.Now()}
	}

	run := runChannelMonitorAccountProbes(context.Background(), request, accounts, attempt)
	if run.Aggregate.Status != MonitorStatusOperational {
		t.Fatalf("aggregate status = %q, want operational", run.Aggregate.Status)
	}
	if run.Aggregate.LatencyMs == nil || *run.Aggregate.LatencyMs != 30 {
		t.Fatalf("aggregate latency = %v, want 30ms from the best successful account", run.Aggregate.LatencyMs)
	}
	if len(run.Results) != 4 {
		t.Fatalf("result rows = %d, want four distinct accounts", len(run.Results))
	}
	for id, count := range called {
		if count != 1 {
			t.Errorf("account %d called %d times, want once", id, count)
		}
	}
}

func TestRunChannelMonitorAccountProbes_SlowAccountDoesNotBlockFollowingAccounts(t *testing.T) {
	request := ChannelMonitorAccountProbeRequest{
		Monitor: &ChannelMonitor{ID: 12, Provider: MonitorProviderOpenAI},
		Model:   "gpt-test",
	}
	accounts := make([]Account, 8)
	for i := range accounts {
		accounts[i] = probeTestAccount(int64(i + 1))
	}

	started := make(chan int64, len(accounts))
	releaseSlow := make(chan struct{})
	runDone := make(chan *ChannelMonitorAccountProbeRun, 1)
	var mu sync.Mutex
	calls := make(map[int64]int)
	go func() {
		runDone <- runChannelMonitorAccountProbes(context.Background(), request, accounts, func(_ context.Context, account *Account, _ ChannelMonitorAccountProbeRequest) *CheckResult {
			started <- account.ID
			mu.Lock()
			calls[account.ID]++
			mu.Unlock()
			if account.ID == 1 {
				<-releaseSlow
			}
			return &CheckResult{Model: request.Model, Status: MonitorStatusOperational, CheckedAt: time.Now()}
		})
	}()

	seen := make(map[int64]struct{}, len(accounts))
	timer := time.NewTimer(2 * time.Second)
	timedOut := false
	for len(seen) < len(accounts) {
		select {
		case id := <-started:
			seen[id] = struct{}{}
		case <-timer.C:
			timedOut = true
		}
		if timedOut {
			break
		}
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	close(releaseSlow)
	run := <-runDone
	if timedOut {
		t.Fatalf("slow account blocked later accounts; started %d/%d", len(seen), len(accounts))
	}
	if run.Aggregate.Status != MonitorStatusOperational {
		t.Fatalf("aggregate status = %q, want operational", run.Aggregate.Status)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, account := range accounts {
		if calls[account.ID] != 1 {
			t.Errorf("account %d called %d times, want once", account.ID, calls[account.ID])
		}
	}
}

func TestMonitorAccountProbeRoundTimeoutForCountUsesConcurrentWaves(t *testing.T) {
	for _, tt := range []struct {
		count int
		want  time.Duration
	}{
		{count: 0, want: monitorAccountProbeRoundBuffer},
		{count: 1, want: monitorAccountProbeTimeout + monitorAccountProbeRoundBuffer},
		{count: 4, want: monitorAccountProbeTimeout + monitorAccountProbeRoundBuffer},
		{count: 5, want: 2*monitorAccountProbeTimeout + monitorAccountProbeRoundBuffer},
		{count: 8, want: 2*monitorAccountProbeTimeout + monitorAccountProbeRoundBuffer},
		{count: 9, want: 3*monitorAccountProbeTimeout + monitorAccountProbeRoundBuffer},
	} {
		t.Run(string(rune('0'+tt.count)), func(t *testing.T) {
			if got := monitorAccountProbeRoundTimeoutForCount(tt.count); got != tt.want {
				t.Fatalf("round timeout for %d accounts = %s, want %s", tt.count, got, tt.want)
			}
		})
	}
}

func TestRunChannelMonitorAccountProbes_DegradedAndFailureAggregation(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []string
		wantStatus string
		wantMsg    string
	}{
		{name: "only degraded", statuses: []string{MonitorStatusDegraded, MonitorStatusFailed}, wantStatus: MonitorStatusDegraded},
		{name: "all failed keeps diagnostic error", statuses: []string{MonitorStatusFailed, MonitorStatusError}, wantStatus: MonitorStatusError, wantMsg: "credential rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := &ChannelMonitor{ID: 8, Provider: MonitorProviderOpenAI}
			request := ChannelMonitorAccountProbeRequest{Monitor: monitor, Model: "gpt-test"}
			accounts := []Account{probeTestAccount(1), probeTestAccount(2)}
			attempt := func(_ context.Context, account *Account, _ ChannelMonitorAccountProbeRequest) *CheckResult {
				message := ""
				if account.ID == 2 && tt.wantMsg != "" {
					message = tt.wantMsg
				}
				latency := int(account.ID) * 100
				return &CheckResult{Model: request.Model, Status: tt.statuses[account.ID-1], Message: message, LatencyMs: &latency, CheckedAt: time.Now()}
			}
			run := runChannelMonitorAccountProbes(context.Background(), request, accounts, attempt)
			if run.Aggregate.Status != tt.wantStatus {
				t.Fatalf("aggregate status = %q, want %q", run.Aggregate.Status, tt.wantStatus)
			}
			if tt.wantMsg != "" && run.Aggregate.Message != tt.wantMsg {
				t.Fatalf("aggregate message = %q, want %q", run.Aggregate.Message, tt.wantMsg)
			}
		})
	}
}

func TestRunChannelMonitorAccountProbes_SkippedAccountsDoNotLowerAvailability(t *testing.T) {
	monitor := &ChannelMonitor{ID: 9, Provider: MonitorProviderOpenAI}
	request := ChannelMonitorAccountProbeRequest{Monitor: monitor, Model: "gpt-test"}
	unsupported := probeTestAccount(1)
	unsupported.Credentials = map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}
	disabled := probeTestAccount(2)
	disabled.Status = StatusDisabled
	called := 0
	run := runChannelMonitorAccountProbes(context.Background(), request, []Account{unsupported, disabled}, func(context.Context, *Account, ChannelMonitorAccountProbeRequest) *CheckResult {
		called++
		return &CheckResult{Status: MonitorStatusOperational, CheckedAt: time.Now()}
	})
	if called != 0 {
		t.Fatalf("skipped accounts invoked attempt %d times", called)
	}
	if run.Aggregate.Status != MonitorStatusError || run.Aggregate.Message != "no applicable accounts for model" {
		t.Fatalf("aggregate = %#v, want unavailable error", run.Aggregate)
	}
	for _, row := range run.Results {
		if !row.Skipped || row.Status != monitorAccountProbeStatusSkipped || row.SkipReason == "" {
			t.Errorf("row was not recorded as skipped: %#v", row)
		}
	}
}

func TestRunChannelMonitorAccountProbes_ContextCancellationStopsNextBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	monitor := &ChannelMonitor{ID: 10, Provider: MonitorProviderOpenAI}
	request := ChannelMonitorAccountProbeRequest{Monitor: monitor, Model: "gpt-test"}
	accounts := make([]Account, 8)
	for i := range accounts {
		accounts[i] = probeTestAccount(int64(i + 1))
	}
	var mu sync.Mutex
	called := 0
	run := runChannelMonitorAccountProbes(ctx, request, accounts, func(ctx context.Context, _ *Account, _ ChannelMonitorAccountProbeRequest) *CheckResult {
		mu.Lock()
		called++
		mu.Unlock()
		cancel()
		<-ctx.Done()
		return &CheckResult{Status: MonitorStatusError, Message: ctx.Err().Error(), CheckedAt: time.Now()}
	})
	mu.Lock()
	started := called
	mu.Unlock()
	if started > monitorAccountProbeConcurrency {
		t.Fatalf("started %d attempts after cancellation, want at most one batch", started)
	}
	if len(run.Results) != len(accounts) {
		t.Fatalf("result rows = %d, want one row per account including skipped", len(run.Results))
	}
	if run.Aggregate.Status != MonitorStatusError {
		t.Fatalf("aggregate status = %q, want error after cancellation", run.Aggregate.Status)
	}
}

func TestRunChannelMonitorAccountProbes_ContextCancellationDoesNotWaitForAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	runDone := make(chan *ChannelMonitorAccountProbeRun, 1)
	request := ChannelMonitorAccountProbeRequest{
		Monitor: &ChannelMonitor{ID: 11, Provider: MonitorProviderOpenAI},
		Model:   "gpt-test",
	}
	accounts := []Account{probeTestAccount(1)}

	go func() {
		runDone <- runChannelMonitorAccountProbes(ctx, request, accounts, func(context.Context, *Account, ChannelMonitorAccountProbeRequest) *CheckResult {
			close(started)
			<-release
			return &CheckResult{Status: MonitorStatusOperational, CheckedAt: time.Now()}
		})
	}()

	<-started
	cancel()
	select {
	case run := <-runDone:
		if run.Aggregate.Status != MonitorStatusError {
			t.Fatalf("aggregate status = %q, want error after cancellation", run.Aggregate.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("account probe waited for an attempt that ignored context cancellation")
	}
	close(release)
}

func TestChannelMonitorPersistenceContextSurvivesExpiredProbeContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	persistCtx, persistCancel := channelMonitorPersistenceContext(ctx)
	defer persistCancel()
	if err := persistCtx.Err(); err != nil {
		t.Fatalf("persistence context is already canceled: %v", err)
	}
	if _, ok := persistCtx.Deadline(); !ok {
		t.Fatal("persistence context must retain an explicit deadline")
	}
}

func TestPersistAccountProbeResultsUsesIndependentContext(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	repo := &capturingChannelMonitorHealthRepository{}
	svc := &ChannelMonitorService{accountProbeResults: repo}

	svc.persistAccountProbeResults(parent, []*ChannelMonitorAccountProbeResult{{
		GroupID:   1,
		AccountID: 2,
		Model:     "gpt-test",
		Provider:  MonitorProviderOpenAI,
		Status:    MonitorStatusError,
		CheckedAt: time.Now(),
	}})

	if repo.ctx == nil {
		t.Fatal("account probe persistence repository was not called")
	}
	if repo.ctxErr != nil {
		t.Fatalf("account probe persistence received canceled context: %v", repo.ctxErr)
	}
}

type capturingChannelMonitorHealthRepository struct {
	ctx    context.Context
	ctxErr error
}

func (*capturingChannelMonitorHealthRepository) InsertAccountProbeResults(context.Context, []*ChannelMonitorAccountProbeResult) error {
	return nil
}

func (r *capturingChannelMonitorHealthRepository) ApplyAccountProbeResults(ctx context.Context, _ []*ChannelMonitorAccountProbeResult) ([]*ChannelMonitorAccountHealthSnapshot, error) {
	r.ctx = ctx
	r.ctxErr = ctx.Err()
	return nil, nil
}

func (*capturingChannelMonitorHealthRepository) ListAccountHealthSnapshots(context.Context, *int64, string, string, int) ([]*ChannelMonitorAccountHealthSnapshot, error) {
	return nil, nil
}

func TestClassifyMonitorProviderResponse_ReplaceModeKeepsNonEmptyAndEmptySemantics(t *testing.T) {
	challenge := monitorChallenge{Expected: "8"}
	fast := 10 * time.Millisecond
	good := &CheckResult{Model: "gpt-test", Status: MonitorStatusError, LatencyMs: probeIntPtr(10), CheckedAt: time.Now()}
	classifyMonitorProviderResponse(good, "not arithmetic but non-empty", "{}", http.StatusOK, nil, fast, challenge, MonitorBodyOverrideModeReplace)
	if good.Status != MonitorStatusOperational {
		t.Fatalf("replace non-empty status = %q, want operational", good.Status)
	}
	empty := &CheckResult{Model: "gpt-test", Status: MonitorStatusError, LatencyMs: probeIntPtr(10), CheckedAt: time.Now()}
	classifyMonitorProviderResponse(empty, "", "{}", http.StatusOK, nil, fast, challenge, MonitorBodyOverrideModeReplace)
	if empty.Status != MonitorStatusFailed {
		t.Fatalf("replace empty status = %q, want failed", empty.Status)
	}
}

func TestSameEndpointOriginRequiresConfiguredLocalOrigin(t *testing.T) {
	settings := probeSettingRepo{value: "https://api.example.test/"}
	if !sameMonitorInstanceEndpoint(context.Background(), settings, "https://api.example.test") {
		t.Fatal("same local origin should be accepted")
	}
	if sameMonitorInstanceEndpoint(context.Background(), settings, "https://other.example.test") {
		t.Fatal("external origin must fall back to group-level probing")
	}
}

func TestChannelMonitorProbeContextSkipsAccountStateSideEffects(t *testing.T) {
	ctx := withChannelMonitorProbe(context.Background())
	if !isChannelMonitorProbe(ctx) || isChannelMonitorProbe(context.Background()) {
		t.Fatal("monitor probe marker must be private to the derived context")
	}

	rateLimits := &RateLimitService{}
	account := &Account{}
	if rateLimits.HandleUpstreamError(ctx, account, http.StatusTooManyRequests, http.Header{}, nil) {
		t.Fatal("monitor probe must not disable an account")
	}
	if rateLimits.HandleTempUnschedulable(ctx, account, http.StatusBadRequest, nil) {
		t.Fatal("monitor probe must not mark an account temporarily unschedulable")
	}
	if rateLimits.HandleOpenAIImageRateLimit(ctx, account, http.StatusTooManyRequests, http.Header{}, nil) {
		t.Fatal("monitor probe must not write an image rate limit")
	}
	if rateLimits.HandleUpstreamModelNotFound(ctx, account, "model", http.StatusNotFound, nil) {
		t.Fatal("monitor probe must not write a model rate limit")
	}
	rateLimits.UpdateSessionWindow(ctx, account, http.Header{
		"anthropic-ratelimit-unified-5h-status": []string{"allowed"},
	})
	if rateLimits.HandleStreamTimeout(ctx, account, "model") {
		t.Fatal("monitor probe must not write a stream timeout state")
	}
}

func probeTestAccount(id int64) Account {
	return Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}
}

func probeIntPtr(value int) *int { return &value }

type probeSettingRepo struct{ value string }

func (r probeSettingRepo) GetValue(context.Context, string) (string, error) { return r.value, nil }

func (r probeSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, errors.New("not implemented")
}

func (r probeSettingRepo) Set(context.Context, string, string) error { return nil }

func (r probeSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func (r probeSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }

func (r probeSettingRepo) GetAll(context.Context) (map[string]string, error) { return nil, nil }

func (r probeSettingRepo) Delete(context.Context, string) error { return nil }

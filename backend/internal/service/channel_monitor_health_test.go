package service

import (
	"context"
	mathrand "math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func healthProbeRow(status string, checkedAt time.Time, latencyMs *int) *ChannelMonitorAccountProbeResult {
	return &ChannelMonitorAccountProbeResult{
		GroupID:   9,
		AccountID: 17,
		Model:     "gpt-test",
		Provider:  PlatformOpenAI,
		Status:    status,
		LatencyMs: latencyMs,
		CheckedAt: checkedAt,
		Skipped:   status == "skipped",
	}
}

func TestAdvanceChannelMonitorAccountHealthSnapshotScoring(t *testing.T) {
	now := time.Now()
	var snapshot *ChannelMonitorAccountHealthSnapshot
	for i := 0; i < 3; i++ {
		snapshot = AdvanceChannelMonitorAccountHealthSnapshot(snapshot, healthProbeRow(MonitorStatusOperational, now.Add(time.Duration(i)*time.Minute), intPointer(250)), now.Add(time.Duration(i)*time.Minute))
	}
	require.Equal(t, ChannelMonitorHealthStateHealthy, snapshot.HealthState)
	require.Greater(t, snapshot.Score, channelMonitorHealthHealthyEnterScore)
	require.Equal(t, 3, snapshot.SampleCount)
	require.Equal(t, 3, snapshot.ConsecutiveSuccesses)
	require.Equal(t, 0, snapshot.ConsecutiveFailures)
	require.NotNil(t, snapshot.EWMALatencyMs)

	degraded := (*ChannelMonitorAccountHealthSnapshot)(nil)
	for i := 0; i < 3; i++ {
		degraded = AdvanceChannelMonitorAccountHealthSnapshot(degraded, healthProbeRow(MonitorStatusDegraded, now.Add(time.Duration(i)*time.Minute), intPointer(7000)), now.Add(time.Duration(i)*time.Minute))
	}
	require.Equal(t, ChannelMonitorHealthStateDegraded, degraded.HealthState)
	require.Less(t, degraded.Score, snapshot.Score)
	require.Equal(t, 3, degraded.SampleCount)
	require.NotNil(t, degraded.EWMALatencyMs)
	require.Greater(t, *degraded.EWMALatencyMs, 0)
}

func TestAdvanceChannelMonitorAccountHealthSnapshotFailureRecoveryAndSkip(t *testing.T) {
	now := time.Now()
	var snapshot *ChannelMonitorAccountHealthSnapshot
	for i := 0; i < 3; i++ {
		snapshot = AdvanceChannelMonitorAccountHealthSnapshot(snapshot, healthProbeRow(MonitorStatusFailed, now.Add(time.Duration(i)*time.Minute), nil), now.Add(time.Duration(i)*time.Minute))
	}
	require.Equal(t, ChannelMonitorHealthStateUnhealthy, snapshot.HealthState)
	require.Equal(t, 3, snapshot.SampleCount)
	require.Equal(t, 3, snapshot.ConsecutiveFailures)
	require.Less(t, snapshot.Score, 10.0)

	beforeSkip := cloneChannelMonitorHealthSnapshot(snapshot)
	skipped := AdvanceChannelMonitorAccountHealthSnapshot(snapshot, healthProbeRow("skipped", now.Add(4*time.Minute), nil), now.Add(4*time.Minute))
	require.Equal(t, beforeSkip, skipped)

	// A single recovery sample cannot immediately erase high-confidence failure
	// history; the score rises gradually and the state remains unhealthy.
	recovered := AdvanceChannelMonitorAccountHealthSnapshot(snapshot, healthProbeRow(MonitorStatusOperational, now.Add(5*time.Minute), intPointer(200)), now.Add(5*time.Minute))
	require.Equal(t, ChannelMonitorHealthStateUnhealthy, recovered.HealthState)
	require.Greater(t, recovered.Score, snapshot.Score)
}

func TestAdvanceChannelMonitorAccountHealthSnapshotStaleResetsConfidence(t *testing.T) {
	now := time.Now()
	old := &ChannelMonitorAccountHealthSnapshot{
		GroupID:              9,
		AccountID:            17,
		Provider:             PlatformOpenAI,
		Model:                "gpt-test",
		Score:                95,
		HealthState:          ChannelMonitorHealthStateHealthy,
		EWMASuccessRate:      1,
		SampleCount:          20,
		ConsecutiveSuccesses: 20,
		LastProbeAt:          now.Add(-channelMonitorHealthStaleTTL - time.Minute),
		ExpiresAt:            now.Add(-time.Minute),
	}
	next := AdvanceChannelMonitorAccountHealthSnapshot(old, healthProbeRow(MonitorStatusOperational, now, intPointer(100)), now)
	require.Equal(t, 1, next.SampleCount)
	require.Equal(t, ChannelMonitorHealthStateUnknown, next.HealthState)
	require.Equal(t, 60.0, next.Score)
}

func TestChannelMonitorHealthCacheIsolationAndBoundedLoad(t *testing.T) {
	resetChannelMonitorHealthCacheForTest()
	now := time.Now()
	cacheChannelMonitorHealthSnapshots([]*ChannelMonitorAccountHealthSnapshot{
		{GroupID: 9, AccountID: 1, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateHealthy, Score: 80, ExpiresAt: now.Add(time.Minute)},
		{GroupID: 9, AccountID: 2, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateUnknown, Score: 50, ExpiresAt: now.Add(time.Minute)},
	})

	ctx := withChannelMonitorHealthMode(context.Background(), ChannelMonitorHealthGateEnabled)
	candidates := []accountWithLoad{
		{account: &Account{ID: 1, Priority: 1}, loadInfo: &AccountLoadInfo{AccountID: 1, LoadRate: 15}},
		{account: &Account{ID: 2, Priority: 1}, loadInfo: &AccountLoadInfo{AccountID: 2, LoadRate: 18}},
	}
	selected := filterByChannelMonitorHealthLoad(ChannelMonitorHealthGateEnabled, candidates)
	require.Len(t, selected, 2)
	require.Equal(t, []int64{1, 2}, []int64{selected[0].account.ID, selected[1].account.ID})
	require.Equal(t, filterByMinLoadRate(candidates), filterByChannelMonitorHealthLoad(ChannelMonitorHealthGateOff, candidates))
	require.Equal(t, filterByMinLoadRate(candidates), filterByChannelMonitorHealthLoad(ChannelMonitorHealthGateShadow, candidates))

	otherModel := channelMonitorHealthStateForSelection(ctx, ChannelMonitorHealthGateEnabled, healthInt64Pointer(9), 1, PlatformOpenAI, "other-model")
	require.Equal(t, ChannelMonitorHealthStateUnknown, otherModel)
	expired := &ChannelMonitorAccountHealthSnapshot{GroupID: 9, AccountID: 3, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateHealthy, ExpiresAt: now.Add(-time.Second)}
	cacheChannelMonitorHealthSnapshots([]*ChannelMonitorAccountHealthSnapshot{expired})
	require.Equal(t, ChannelMonitorHealthStateUnknown, channelMonitorHealthStateForSelection(ctx, ChannelMonitorHealthGateEnabled, healthInt64Pointer(9), 3, PlatformOpenAI, "gpt-test"))
}

func TestChannelMonitorHealthGateAndStickyConfidence(t *testing.T) {
	require.Equal(t, ChannelMonitorHealthGateOff, normalizeChannelMonitorHealthMode(""))
	require.Equal(t, ChannelMonitorHealthGateShadow, normalizeChannelMonitorHealthMode(" SHADOW "))
	require.Equal(t, ChannelMonitorHealthGateEnabled, normalizeChannelMonitorHealthMode("enabled"))
	require.Equal(t, ChannelMonitorHealthGateOff, normalizeChannelMonitorHealthMode("invalid"))

	groupID := int64(9)
	healthySticky := &ChannelMonitorAccountHealthSnapshot{GroupID: groupID, AccountID: 1, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateDegraded, Score: 40, SampleCount: 10, ConsecutiveFailures: 1, ExpiresAt: time.Now().Add(time.Minute)}
	unhealthySticky := &ChannelMonitorAccountHealthSnapshot{GroupID: groupID, AccountID: 2, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateUnhealthy, Score: 20, SampleCount: 6, ConsecutiveFailures: 4, ExpiresAt: time.Now().Add(time.Minute)}
	resetChannelMonitorHealthCacheForTest()
	cacheChannelMonitorHealthSnapshots([]*ChannelMonitorAccountHealthSnapshot{healthySticky, unhealthySticky})
	ctx := context.Background()
	require.False(t, shouldBypassChannelMonitorSticky(ctx, ChannelMonitorHealthGateEnabled, &groupID, 1, PlatformOpenAI, "gpt-test"))
	require.True(t, shouldBypassChannelMonitorSticky(ctx, ChannelMonitorHealthGateEnabled, &groupID, 2, PlatformOpenAI, "gpt-test"))
	require.False(t, shouldBypassChannelMonitorSticky(ctx, ChannelMonitorHealthGateShadow, &groupID, 2, PlatformOpenAI, "gpt-test"))
}

func TestChannelMonitorHealthSchedulerModesAndPriorityBoundary(t *testing.T) {
	resetChannelMonitorHealthCacheForTest()
	groupID := int64(9)
	now := time.Now()
	cacheChannelMonitorHealthSnapshots([]*ChannelMonitorAccountHealthSnapshot{
		{GroupID: groupID, AccountID: 1, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateHealthy, Score: 85, SampleCount: 8, ExpiresAt: now.Add(time.Minute)},
		{GroupID: groupID, AccountID: 2, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateUnknown, Score: 50, SampleCount: 0, ExpiresAt: now.Add(time.Minute)},
	})
	past := time.Now().Add(-time.Hour)
	candidate := &Account{ID: 1, Priority: 1}
	current := &Account{ID: 2, Priority: 1, LastUsedAt: &past}
	ctx := context.Background()
	openAI := &OpenAIGatewayService{}
	require.True(t, openAI.isBetterAccount(withChannelMonitorHealthMode(ctx, ChannelMonitorHealthGateEnabled), &groupID, PlatformOpenAI, "gpt-test", candidate, current))
	require.True(t, openAI.isBetterAccount(withChannelMonitorHealthMode(ctx, ChannelMonitorHealthGateShadow), &groupID, PlatformOpenAI, "gpt-test", candidate, current))
}

func TestChannelMonitorHealthWeightedSelectionKeepsExplorationAndPriority(t *testing.T) {
	resetChannelMonitorHealthCacheForTest()
	groupID := int64(9)
	now := time.Now()
	cacheChannelMonitorHealthSnapshots([]*ChannelMonitorAccountHealthSnapshot{
		{GroupID: groupID, AccountID: 1, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateHealthy, Score: 85, SampleCount: 8, ExpiresAt: now.Add(time.Minute)},
		{GroupID: groupID, AccountID: 2, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateUnknown, Score: 50, ExpiresAt: now.Add(time.Minute)},
		{GroupID: groupID, AccountID: 3, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateDegraded, Score: 55, SampleCount: 4, ExpiresAt: now.Add(time.Minute)},
		{GroupID: groupID, AccountID: 4, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateHealthy, Score: 85, SampleCount: 8, ExpiresAt: now.Add(time.Minute)},
	})
	accounts := []*Account{
		{ID: 1, Priority: 1},
		{ID: 2, Priority: 1},
		{ID: 3, Priority: 1},
		{ID: 4, Priority: 2},
	}
	random := mathrand.New(mathrand.NewSource(7))
	ctx := withChannelMonitorHealthRandomSource(withChannelMonitorHealthMode(context.Background(), ChannelMonitorHealthGateEnabled), random)
	counts := map[int64]int{}
	for i := 0; i < 4000; i++ {
		selected := channelMonitorHealthWeightedAccount(ctx, ChannelMonitorHealthGateEnabled, &groupID, accounts, PlatformOpenAI, "gpt-test", false)
		require.NotNil(t, selected)
		counts[selected.ID]++
	}
	require.Greater(t, counts[1], counts[2], "healthy should be favored over unknown")
	require.Greater(t, counts[2], counts[3], "unknown should retain a larger exploration share than degraded")
	require.Greater(t, counts[2], 0, "unknown must remain selectable")
	require.Less(t, counts[1], 4000, "healthy must not be a permanent winner")
	require.Zero(t, counts[4], "health must not cross the static priority boundary")
}

func TestChannelMonitorHealthWeightedLoadUsesRawLoadOnce(t *testing.T) {
	resetChannelMonitorHealthCacheForTest()
	groupID := int64(9)
	now := time.Now()
	cacheChannelMonitorHealthSnapshots([]*ChannelMonitorAccountHealthSnapshot{
		{GroupID: groupID, AccountID: 11, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateHealthy, Score: 85, SampleCount: 8, ExpiresAt: now.Add(time.Minute)},
		{GroupID: groupID, AccountID: 12, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateUnknown, Score: 50, ExpiresAt: now.Add(time.Minute)},
		{GroupID: groupID, AccountID: 13, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateDegraded, Score: 55, SampleCount: 4, ExpiresAt: now.Add(time.Minute)},
		{GroupID: groupID, AccountID: 14, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateUnhealthy, Score: 20, SampleCount: 6, ConsecutiveFailures: 4, ExpiresAt: now.Add(time.Minute)},
		{GroupID: groupID, AccountID: 15, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateHealthy, Score: 85, SampleCount: 8, ExpiresAt: now.Add(time.Minute)},
	})
	priorityCandidates := []accountWithLoad{
		{account: &Account{ID: 11, Priority: 1}, loadInfo: &AccountLoadInfo{AccountID: 11, LoadRate: 30}},
		{account: &Account{ID: 12, Priority: 1}, loadInfo: &AccountLoadInfo{AccountID: 12, LoadRate: 30}},
		{account: &Account{ID: 13, Priority: 1}, loadInfo: &AccountLoadInfo{AccountID: 13, LoadRate: 30}},
		{account: &Account{ID: 14, Priority: 1}, loadInfo: &AccountLoadInfo{AccountID: 14, LoadRate: 30}},
	}
	ctx := withChannelMonitorHealthMode(context.Background(), ChannelMonitorHealthGateEnabled)
	require.Len(t, filterByChannelMonitorHealthLoad(ChannelMonitorHealthGateEnabled, priorityCandidates), len(priorityCandidates), "same raw load must not be removed by health state")
	require.Zero(t, channelMonitorHealthLoadSelectionWeight(ctx, ChannelMonitorHealthGateEnabled, &groupID, priorityCandidates[3], priorityCandidates, PlatformOpenAI, "gpt-test", 30, false), "high-confidence unhealthy is only a last fallback")

	differentLoads := []accountWithLoad{
		{account: &Account{ID: 11, Priority: 1}, loadInfo: &AccountLoadInfo{AccountID: 11, LoadRate: 10}},
		{account: &Account{ID: 12, Priority: 1}, loadInfo: &AccountLoadInfo{AccountID: 12, LoadRate: 20}},
		{account: &Account{ID: 13, Priority: 1}, loadInfo: &AccountLoadInfo{AccountID: 13, LoadRate: 20}},
	}
	require.Equal(t, []int64{11}, accountIDsFromLoad(filterByChannelMonitorHealthLoad(ChannelMonitorHealthGateOff, differentLoads)))
	require.Equal(t, []int64{11}, accountIDsFromLoad(filterByChannelMonitorHealthLoad(ChannelMonitorHealthGateShadow, differentLoads)))

	random := mathrand.New(mathrand.NewSource(11))
	selectionCtx := withChannelMonitorHealthRandomSource(ctx, random)
	allCandidates := append(append([]accountWithLoad{}, priorityCandidates...), accountWithLoad{
		account: &Account{ID: 15, Priority: 2}, loadInfo: &AccountLoadInfo{AccountID: 15, LoadRate: 30},
	})
	counts := map[int64]int{}
	for i := 0; i < 4000; i++ {
		order := channelMonitorHealthWeightedLoadOrder(selectionCtx, ChannelMonitorHealthGateEnabled, &groupID, allCandidates, PlatformOpenAI, "gpt-test", false)
		require.Len(t, order, len(allCandidates))
		counts[order[0].account.ID]++
		require.Equal(t, int64(15), order[len(order)-1].account.ID, "lower Priority must remain after the higher tier")
	}
	require.Greater(t, counts[11], counts[12], "healthy should be favored over unknown at equal raw load")
	require.Greater(t, counts[12], counts[13], "unknown should be favored over degraded at equal raw load")
	require.Greater(t, counts[11], 0)
	require.Greater(t, counts[12], 0)
	require.Greater(t, counts[13], 0)
	require.Zero(t, counts[14], "high-confidence unhealthy should not win while other candidates exist")
}

func accountIDsFromLoad(candidates []accountWithLoad) []int64 {
	ids := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.account.ID)
	}
	return ids
}

func TestChannelMonitorHealthRandomSourceIsConcurrentSafe(t *testing.T) {
	const workers = 8
	const drawsPerWorker = 200
	var waitGroup sync.WaitGroup
	invalid := make(chan float64, workers*drawsPerWorker)
	waitGroup.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer waitGroup.Done()
			for j := 0; j < drawsPerWorker; j++ {
				draw := channelMonitorHealthRandomFloat64(context.Background())
				if draw < 0 || draw >= 1 {
					invalid <- draw
				}
			}
		}()
	}
	waitGroup.Wait()
	close(invalid)
	require.Empty(t, invalid)
}

func TestAdvancedOpenAISchedulerHealthInfluenceStaysWithinPriorityTier(t *testing.T) {
	resetChannelMonitorHealthCacheForTest()
	now := time.Now()
	groupID := int64(9)
	cacheChannelMonitorHealthSnapshots([]*ChannelMonitorAccountHealthSnapshot{
		{GroupID: groupID, AccountID: 1, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateHealthy, Score: 85, SampleCount: 8, ExpiresAt: now.Add(time.Minute)},
		{GroupID: groupID, AccountID: 2, Provider: PlatformOpenAI, Model: "gpt-test", HealthState: ChannelMonitorHealthStateUnknown, Score: 50, ExpiresAt: now.Add(time.Minute)},
	})
	candidates := []openAIAccountCandidateScore{
		{account: &Account{ID: 1, Priority: 1}, score: 10, priority: 1},
		{account: &Account{ID: 2, Priority: 1}, score: 10, priority: 1},
		{account: &Account{ID: 3, Priority: 2}, score: 100, priority: 2},
	}
	scheduler := &defaultOpenAIAccountScheduler{service: &OpenAIGatewayService{}}
	scheduler.applyChannelMonitorHealthToOpenAICandidates(
		withChannelMonitorHealthMode(context.Background(), ChannelMonitorHealthGateEnabled),
		OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-test"},
		candidates,
	)
	require.Greater(t, candidates[0].score, candidates[1].score)
	// The different Priority tier receives no health snapshot and is not
	// modified by the tier-local influence.
	require.Equal(t, 100.0, candidates[2].score)

	shadow := []openAIAccountCandidateScore{
		{account: &Account{ID: 1, Priority: 1}, score: 10, priority: 1},
		{account: &Account{ID: 2, Priority: 1}, score: 10, priority: 1},
	}
	scheduler.applyChannelMonitorHealthToOpenAICandidates(
		withChannelMonitorHealthMode(context.Background(), ChannelMonitorHealthGateShadow),
		OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-test"},
		shadow,
	)
	require.Equal(t, 10.0, shadow[0].score)
	require.Equal(t, 10.0, shadow[1].score)
}

func healthInt64Pointer(value int64) *int64 {
	return &value
}

package service

import (
	"context"
	mathrand "math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ChannelMonitorHealthGateMode controls whether monitor-derived health is used
// by the real-user scheduler. It is deliberately independent from the V1/V2
// channel-monitor mode.
type ChannelMonitorHealthGateMode string

const (
	ChannelMonitorHealthGateOff     ChannelMonitorHealthGateMode = "off"
	ChannelMonitorHealthGateShadow  ChannelMonitorHealthGateMode = "shadow"
	ChannelMonitorHealthGateEnabled ChannelMonitorHealthGateMode = "enabled"

	ChannelMonitorHealthStateUnknown   = "unknown"
	ChannelMonitorHealthStateHealthy   = "healthy"
	ChannelMonitorHealthStateDegraded  = "degraded"
	ChannelMonitorHealthStateUnhealthy = "unhealthy"
)

const (
	channelMonitorHealthInitialScore        = 50.0
	channelMonitorHealthEWMAAlpha           = 0.20
	channelMonitorHealthOperationalTarget   = 100.0
	channelMonitorHealthDegradedTarget      = 72.0
	channelMonitorHealthFailureMultiplier   = 0.45
	channelMonitorHealthMinSamples          = 3
	channelMonitorHealthHealthyEnterScore   = 70.0
	channelMonitorHealthHealthyExitScore    = 55.0
	channelMonitorHealthUnhealthyEnterScore = 30.0
	channelMonitorHealthUnhealthyExitScore  = 45.0
	channelMonitorHealthFailureHysteresis   = 2
	channelMonitorHealthStaleTTL            = 15 * time.Minute
	channelMonitorHealthModeCacheTTL        = 5 * time.Second
	// Advanced OpenAI scheduling may use health only as a bounded nudge inside
	// one static Priority tier; it never becomes a replacement priority.
	channelMonitorHealthOpenAIScoreInfluence    = 0.20
	channelMonitorHealthOpenAIMinScoreInfluence = 0.05
	// Health is a bounded probability nudge, not an absolute rank. The load
	// and LRU terms below remain part of the same weighted draw.
	channelMonitorHealthHealthyWeight   = 1.20
	channelMonitorHealthUnknownWeight   = 0.80
	channelMonitorHealthDegradedWeight  = 0.55
	channelMonitorHealthUnhealthyWeight = 0.20
	channelMonitorHealthLoadBand        = 12.0
	channelMonitorHealthMinLoadWeight   = 0.35
	channelMonitorHealthMaxLRUBonus     = 0.35
)

// ChannelMonitorAccountHealthSnapshot is the latest health state for one
// account in one local group/provider/model tuple. It is intentionally not an
// account field: health must never rewrite administrator-owned Priority.
type ChannelMonitorAccountHealthSnapshot struct {
	GroupID   int64
	AccountID int64
	// AccountName is populated only for administrative health views. It is
	// intentionally not persisted and is not used by routing or score logic.
	AccountName          string
	Provider             string
	Model                string
	Score                float64
	HealthState          string
	EWMASuccessRate      float64
	EWMALatencyMs        *int
	SampleCount          int
	ConsecutiveSuccesses int
	ConsecutiveFailures  int
	LastStatus           string
	LastProbeAt          time.Time
	UpdatedAt            time.Time
	ExpiresAt            time.Time
}

// AdvanceChannelMonitorAccountHealthSnapshot applies one real, applicable
// probe observation. Skipped observations intentionally return the previous
// snapshot unchanged and therefore do not affect confidence or score.
func AdvanceChannelMonitorAccountHealthSnapshot(
	previous *ChannelMonitorAccountHealthSnapshot,
	observation *ChannelMonitorAccountProbeResult,
	now time.Time,
) *ChannelMonitorAccountHealthSnapshot {
	if observation == nil || observation.Skipped || observation.Status == "skipped" {
		return cloneChannelMonitorHealthSnapshot(previous)
	}
	if now.IsZero() {
		now = time.Now()
	}
	checkedAt := observation.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = now
	}

	// A stale row is not allowed to provide a free confidence boost when the
	// first fresh sample arrives after an outage or a long monitor pause.
	if previous != nil && !previous.LastProbeAt.IsZero() && checkedAt.Sub(previous.LastProbeAt) > channelMonitorHealthStaleTTL {
		previous = nil
	}

	next := &ChannelMonitorAccountHealthSnapshot{
		GroupID:     observation.GroupID,
		AccountID:   observation.AccountID,
		Provider:    normalizeChannelMonitorHealthProvider(observation.Provider),
		Model:       strings.TrimSpace(observation.Model),
		Score:       channelMonitorHealthInitialScore,
		HealthState: ChannelMonitorHealthStateUnknown,
		LastStatus:  strings.TrimSpace(observation.Status),
		LastProbeAt: checkedAt,
		UpdatedAt:   now,
		ExpiresAt:   checkedAt.Add(channelMonitorHealthStaleTTL),
	}
	if previous != nil {
		next.Score = clampChannelMonitorHealthScore(previous.Score)
		next.EWMASuccessRate = clampChannelMonitorHealth01(previous.EWMASuccessRate)
		next.EWMALatencyMs = cloneIntPointer(previous.EWMALatencyMs)
		next.SampleCount = previous.SampleCount
		next.ConsecutiveSuccesses = previous.ConsecutiveSuccesses
		next.ConsecutiveFailures = previous.ConsecutiveFailures
		next.HealthState = previous.HealthState
	}

	success := observation.Status == MonitorStatusOperational || observation.Status == MonitorStatusDegraded
	if success {
		next.ConsecutiveSuccesses++
		next.ConsecutiveFailures = 0
	} else {
		next.ConsecutiveFailures++
		next.ConsecutiveSuccesses = 0
	}
	next.SampleCount++
	result := 0.0
	if success {
		result = 1
	}
	if next.SampleCount == 1 {
		next.EWMASuccessRate = result
	} else {
		next.EWMASuccessRate = channelMonitorHealthEWMAAlpha*result +
			(1-channelMonitorHealthEWMAAlpha)*next.EWMASuccessRate
	}

	if observation.LatencyMs != nil && *observation.LatencyMs >= 0 && success {
		latency := *observation.LatencyMs
		if next.EWMALatencyMs == nil {
			next.EWMALatencyMs = cloneIntPointer(&latency)
		} else {
			ewma := channelMonitorHealthEWMAAlpha*float64(latency) +
				(1-channelMonitorHealthEWMAAlpha)*float64(*next.EWMALatencyMs)
			next.EWMALatencyMs = intPointer(int(ewma + 0.5))
		}
	}

	switch observation.Status {
	case MonitorStatusOperational:
		next.Score += channelMonitorHealthEWMAAlpha *
			(channelMonitorHealthOperationalTarget - next.Score)
	case MonitorStatusDegraded:
		next.Score += channelMonitorHealthEWMAAlpha *
			(channelMonitorHealthDegradedTarget - next.Score)
		if observation.LatencyMs != nil && *observation.LatencyMs > 0 {
			latencyPenalty := float64(*observation.LatencyMs) / 500.0
			if latencyPenalty > 18 {
				latencyPenalty = 18
			}
			next.Score -= latencyPenalty
		}
	default:
		// Failed/error observations are intentionally faster to reduce score
		// than successful observations are allowed to increase it.
		next.Score *= channelMonitorHealthFailureMultiplier
	}
	next.Score = clampChannelMonitorHealthScore(next.Score)
	next.HealthState = channelMonitorHealthState(next, previous, now)
	return next
}

func channelMonitorHealthState(snapshot *ChannelMonitorAccountHealthSnapshot, previous *ChannelMonitorAccountHealthSnapshot, now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	if snapshot == nil || snapshot.SampleCount < channelMonitorHealthMinSamples ||
		!snapshot.ExpiresAt.IsZero() && !now.Before(snapshot.ExpiresAt) {
		return ChannelMonitorHealthStateUnknown
	}

	// Separate enter/exit thresholds provide hysteresis around both edges.
	if previous != nil && previous.HealthState == ChannelMonitorHealthStateHealthy &&
		snapshot.Score >= channelMonitorHealthHealthyExitScore &&
		snapshot.ConsecutiveFailures < channelMonitorHealthFailureHysteresis {
		return ChannelMonitorHealthStateHealthy
	}
	if previous != nil && previous.HealthState == ChannelMonitorHealthStateUnhealthy &&
		(snapshot.Score <= channelMonitorHealthUnhealthyExitScore || snapshot.ConsecutiveFailures > 0) {
		return ChannelMonitorHealthStateUnhealthy
	}
	if snapshot.Score >= channelMonitorHealthHealthyEnterScore &&
		snapshot.ConsecutiveFailures == 0 && snapshot.EWMASuccessRate >= 0.65 {
		return ChannelMonitorHealthStateHealthy
	}
	if snapshot.Score <= channelMonitorHealthUnhealthyEnterScore &&
		snapshot.ConsecutiveFailures >= channelMonitorHealthFailureHysteresis &&
		snapshot.EWMASuccessRate <= 0.5 {
		return ChannelMonitorHealthStateUnhealthy
	}
	return ChannelMonitorHealthStateDegraded
}

func clampChannelMonitorHealthScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func clampChannelMonitorHealth01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func normalizeChannelMonitorHealthProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func cloneChannelMonitorHealthSnapshot(snapshot *ChannelMonitorAccountHealthSnapshot) *ChannelMonitorAccountHealthSnapshot {
	if snapshot == nil {
		return nil
	}
	clone := *snapshot
	clone.EWMALatencyMs = cloneIntPointer(snapshot.EWMALatencyMs)
	return &clone
}

func intPointer(value int) *int {
	return &value
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

type channelMonitorHealthCacheKey struct {
	groupID   int64
	accountID int64
	provider  string
	model     string
}

var channelMonitorHealthCache = struct {
	sync.RWMutex
	items map[channelMonitorHealthCacheKey]*ChannelMonitorAccountHealthSnapshot
}{items: make(map[channelMonitorHealthCacheKey]*ChannelMonitorAccountHealthSnapshot)}

func cacheChannelMonitorHealthSnapshots(snapshots []*ChannelMonitorAccountHealthSnapshot) {
	if len(snapshots) == 0 {
		return
	}
	channelMonitorHealthCache.Lock()
	defer channelMonitorHealthCache.Unlock()
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		key := channelMonitorHealthCacheKey{
			groupID:   snapshot.GroupID,
			accountID: snapshot.AccountID,
			provider:  normalizeChannelMonitorHealthProvider(snapshot.Provider),
			model:     strings.TrimSpace(snapshot.Model),
		}
		channelMonitorHealthCache.items[key] = cloneChannelMonitorHealthSnapshot(snapshot)
	}
}

func cachedChannelMonitorHealthSnapshot(groupID, accountID int64, provider, model string, now time.Time) (*ChannelMonitorAccountHealthSnapshot, bool) {
	key := channelMonitorHealthCacheKey{
		groupID:   groupID,
		accountID: accountID,
		provider:  normalizeChannelMonitorHealthProvider(provider),
		model:     strings.TrimSpace(model),
	}
	channelMonitorHealthCache.RLock()
	snapshot := cloneChannelMonitorHealthSnapshot(channelMonitorHealthCache.items[key])
	channelMonitorHealthCache.RUnlock()
	if snapshot == nil || !snapshot.ExpiresAt.IsZero() && !now.Before(snapshot.ExpiresAt) {
		channelMonitorHealthCacheMisses.Add(1)
		return nil, false
	}
	channelMonitorHealthCacheHits.Add(1)
	return snapshot, true
}

func resetChannelMonitorHealthCacheForTest() {
	channelMonitorHealthCache.Lock()
	channelMonitorHealthCache.items = make(map[channelMonitorHealthCacheKey]*ChannelMonitorAccountHealthSnapshot)
	channelMonitorHealthCache.Unlock()
	channelMonitorHealthModeCache.Range(func(key, _ any) bool {
		channelMonitorHealthModeCache.Delete(key)
		return true
	})
}

type channelMonitorHealthModeContextKey struct{}

func withChannelMonitorHealthMode(ctx context.Context, mode ChannelMonitorHealthGateMode) context.Context {
	return context.WithValue(ctx, channelMonitorHealthModeContextKey{}, mode)
}

type channelMonitorHealthModeCacheEntry struct {
	mu      sync.Mutex
	mode    ChannelMonitorHealthGateMode
	expires time.Time
}

var channelMonitorHealthModeCache sync.Map // *SettingService -> *channelMonitorHealthModeCacheEntry

func channelMonitorHealthMode(ctx context.Context, settings *SettingService) ChannelMonitorHealthGateMode {
	if ctx != nil {
		if mode, ok := ctx.Value(channelMonitorHealthModeContextKey{}).(ChannelMonitorHealthGateMode); ok {
			return mode
		}
	}
	if settings == nil || settings.settingRepo == nil {
		return ChannelMonitorHealthGateOff
	}
	entryValue, _ := channelMonitorHealthModeCache.LoadOrStore(settings, &channelMonitorHealthModeCacheEntry{})
	entry, ok := entryValue.(*channelMonitorHealthModeCacheEntry)
	if !ok || entry == nil {
		entry = &channelMonitorHealthModeCacheEntry{}
		channelMonitorHealthModeCache.Store(settings, entry)
	}
	now := time.Now()
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if now.Before(entry.expires) {
		return entry.mode
	}
	raw, err := settings.settingRepo.GetValue(ctx, SettingKeyChannelMonitorHealthMode)
	if err != nil {
		entry.mode = ChannelMonitorHealthGateOff
	} else {
		entry.mode = normalizeChannelMonitorHealthMode(raw)
	}
	entry.expires = now.Add(channelMonitorHealthModeCacheTTL)
	return entry.mode
}

func normalizeChannelMonitorHealthMode(raw string) ChannelMonitorHealthGateMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ChannelMonitorHealthGateShadow):
		return ChannelMonitorHealthGateShadow
	case string(ChannelMonitorHealthGateEnabled):
		return ChannelMonitorHealthGateEnabled
	default:
		return ChannelMonitorHealthGateOff
	}
}

type channelMonitorHealthStats struct {
	EnabledDecisions  atomic.Uint64
	ShadowDecisions   atomic.Uint64
	ShadowDifferent   atomic.Uint64
	StickyBypass      atomic.Uint64
	UnhealthyFallback atomic.Uint64
	CacheHits         atomic.Uint64
	CacheMisses       atomic.Uint64
}

var monitorHealthStats channelMonitorHealthStats

type channelMonitorHealthRandomSource interface {
	Float64() float64
}

type channelMonitorHealthRandomContextKey struct{}

var (
	channelMonitorHealthRandomMu sync.Mutex
	channelMonitorHealthRandom   = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
)

// withChannelMonitorHealthRandomSource is a test seam. Production draws use
// a mutex-protected process-local source; no shared unsynchronized rand is
// reachable from concurrent request paths.
func withChannelMonitorHealthRandomSource(ctx context.Context, source channelMonitorHealthRandomSource) context.Context {
	return context.WithValue(ctx, channelMonitorHealthRandomContextKey{}, source)
}

func channelMonitorHealthRandomFloat64(ctx context.Context) float64 {
	if ctx != nil {
		if source, ok := ctx.Value(channelMonitorHealthRandomContextKey{}).(channelMonitorHealthRandomSource); ok && source != nil {
			return clampChannelMonitorHealthRandomFloat(source.Float64())
		}
	}
	channelMonitorHealthRandomMu.Lock()
	value := channelMonitorHealthRandom.Float64()
	channelMonitorHealthRandomMu.Unlock()
	return value
}

func clampChannelMonitorHealthRandomFloat(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value >= 1 {
		return 0.999999999999
	}
	return value
}

// ChannelMonitorHealthStats is a low-cardinality in-process diagnostic view.
// It contains counters only; no account IDs, messages, keys, or upstream data.
type ChannelMonitorHealthStats struct {
	EnabledDecisions  uint64
	ShadowDecisions   uint64
	ShadowDifferent   uint64
	StickyBypass      uint64
	UnhealthyFallback uint64
	CacheHits         uint64
	CacheMisses       uint64
}

func GetChannelMonitorHealthStats() ChannelMonitorHealthStats {
	return ChannelMonitorHealthStats{
		EnabledDecisions:  monitorHealthStats.EnabledDecisions.Load(),
		ShadowDecisions:   monitorHealthStats.ShadowDecisions.Load(),
		ShadowDifferent:   monitorHealthStats.ShadowDifferent.Load(),
		StickyBypass:      monitorHealthStats.StickyBypass.Load(),
		UnhealthyFallback: monitorHealthStats.UnhealthyFallback.Load(),
		CacheHits:         channelMonitorHealthCacheHits.Load(),
		CacheMisses:       channelMonitorHealthCacheMisses.Load(),
	}
}

var (
	channelMonitorHealthCacheHits   atomic.Uint64
	channelMonitorHealthCacheMisses atomic.Uint64
)

func channelMonitorHealthStateForSelection(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, accountID int64, provider, model string) string {
	if mode == ChannelMonitorHealthGateOff || groupID == nil || accountID <= 0 {
		return ChannelMonitorHealthStateUnknown
	}
	snapshot, ok := cachedChannelMonitorHealthSnapshot(*groupID, accountID, provider, model, time.Now())
	if !ok || snapshot == nil {
		return ChannelMonitorHealthStateUnknown
	}
	return snapshot.HealthState
}

func channelMonitorHealthSnapshotForSelection(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, accountID int64, provider, model string) *ChannelMonitorAccountHealthSnapshot {
	if mode == ChannelMonitorHealthGateOff || groupID == nil || accountID <= 0 {
		return nil
	}
	snapshot, ok := cachedChannelMonitorHealthSnapshot(*groupID, accountID, provider, model, time.Now())
	if !ok {
		return nil
	}
	return snapshot
}

func channelMonitorHealthRank(state string) int {
	switch state {
	case ChannelMonitorHealthStateHealthy:
		return 3
	case ChannelMonitorHealthStateUnknown:
		return 2
	case ChannelMonitorHealthStateDegraded:
		return 1
	default:
		return 0
	}
}

func channelMonitorHealthHighConfidenceUnhealthy(snapshot *ChannelMonitorAccountHealthSnapshot) bool {
	return snapshot != nil && snapshot.HealthState == ChannelMonitorHealthStateUnhealthy &&
		snapshot.SampleCount >= 5 && snapshot.ConsecutiveFailures >= 3 && snapshot.Score <= 35
}

func shouldBypassChannelMonitorSticky(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, accountID int64, provider, model string) bool {
	if mode != ChannelMonitorHealthGateEnabled {
		return false
	}
	snapshot := channelMonitorHealthSnapshotForSelection(ctx, mode, groupID, accountID, provider, model)
	if !channelMonitorHealthHighConfidenceUnhealthy(snapshot) {
		return false
	}
	monitorHealthStats.StickyBypass.Add(1)
	return true
}

func channelMonitorHealthWeight(snapshot *ChannelMonitorAccountHealthSnapshot) float64 {
	if snapshot == nil {
		return channelMonitorHealthUnknownWeight
	}
	switch snapshot.HealthState {
	case ChannelMonitorHealthStateHealthy:
		return channelMonitorHealthHealthyWeight
	case ChannelMonitorHealthStateDegraded:
		return channelMonitorHealthDegradedWeight
	case ChannelMonitorHealthStateUnhealthy:
		if channelMonitorHealthHighConfidenceUnhealthy(snapshot) {
			return channelMonitorHealthUnhealthyWeight / 2
		}
		return channelMonitorHealthUnhealthyWeight
	default:
		return channelMonitorHealthUnknownWeight
	}
}

func channelMonitorHealthAccountSnapshot(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, account *Account, provider, model string) *ChannelMonitorAccountHealthSnapshot {
	if account == nil {
		return nil
	}
	return channelMonitorHealthSnapshotForSelection(ctx, mode, groupID, account.ID, provider, model)
}

func channelMonitorHealthAccountOlder(left, right *Account) bool {
	if left == nil || right == nil {
		return false
	}
	if left.LastUsedAt == nil {
		return right.LastUsedAt != nil
	}
	if right.LastUsedAt == nil {
		return false
	}
	return left.LastUsedAt.Before(*right.LastUsedAt)
}

func channelMonitorHealthAccountLRUWeight(account *Account, peers []*Account) float64 {
	if account == nil || len(peers) <= 1 {
		return 1
	}
	newer := 0
	for _, peer := range peers {
		if peer != nil && peer.ID != account.ID && channelMonitorHealthAccountOlder(account, peer) {
			newer++
		}
	}
	return 1 + channelMonitorHealthMaxLRUBonus*float64(newer)/float64(len(peers)-1)
}

func channelMonitorHealthHasNonHighConfidenceCandidate(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, accounts []*Account, provider, model string) bool {
	for _, account := range accounts {
		if !channelMonitorHealthHighConfidenceUnhealthy(channelMonitorHealthAccountSnapshot(ctx, mode, groupID, account, provider, model)) {
			return true
		}
	}
	return false
}

func channelMonitorHealthAccountSelectionWeight(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, account *Account, peers []*Account, provider, model string, preferOAuth bool) float64 {
	snapshot := channelMonitorHealthAccountSnapshot(ctx, mode, groupID, account, provider, model)
	if channelMonitorHealthHighConfidenceUnhealthy(snapshot) && channelMonitorHealthHasNonHighConfidenceCandidate(ctx, mode, groupID, peers, provider, model) {
		return 0
	}
	weight := channelMonitorHealthWeight(snapshot) * channelMonitorHealthAccountLRUWeight(account, peers)
	if preferOAuth && account != nil && account.Type == AccountTypeOAuth {
		weight *= 1.10
	}
	return weight
}

func peersForChannelMonitorHealth(accounts []*Account, priority int) []*Account {
	peers := make([]*Account, 0, len(accounts))
	for _, account := range accounts {
		if account != nil && account.Priority == priority {
			peers = append(peers, account)
		}
	}
	return peers
}

func channelMonitorHealthWeightedAccount(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, accounts []*Account, provider, model string, preferOAuth bool) *Account {
	if len(accounts) == 0 {
		return nil
	}
	if mode == ChannelMonitorHealthGateEnabled {
		monitorHealthStats.EnabledDecisions.Add(1)
	}
	minPriority := 0
	hasPriority := false
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if !hasPriority || account.Priority < minPriority {
			minPriority = account.Priority
			hasPriority = true
		}
	}
	peers := peersForChannelMonitorHealth(accounts, minPriority)
	if len(peers) == 0 {
		return nil
	}
	total := 0.0
	weights := make([]float64, len(peers))
	for i, account := range peers {
		weights[i] = channelMonitorHealthAccountSelectionWeight(ctx, mode, groupID, account, peers, provider, model, preferOAuth)
		total += weights[i]
	}
	if total <= 0 {
		// All candidates are high-confidence unhealthy; preserve availability as
		// a last fallback rather than turning health into a hard ban.
		monitorHealthStats.UnhealthyFallback.Add(1)
		for i := range weights {
			weights[i] = 1
			total++
		}
	}
	draw := channelMonitorHealthRandomFloat64(ctx) * total
	for i, weight := range weights {
		if draw < weight {
			return peers[i]
		}
		draw -= weight
	}
	return peers[len(peers)-1]
}

func channelMonitorHealthWeightedAccountOrder(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, accounts []*Account, provider, model string, preferOAuth bool) []*Account {
	remaining := make([]*Account, 0, len(accounts))
	for _, account := range accounts {
		if account != nil {
			remaining = append(remaining, account)
		}
	}
	ordered := make([]*Account, 0, len(remaining))
	for len(remaining) > 0 {
		minPriority := remaining[0].Priority
		for _, account := range remaining[1:] {
			if account != nil && account.Priority < minPriority {
				minPriority = account.Priority
			}
		}
		pool := make([]*Account, 0, len(remaining))
		rest := make([]*Account, 0, len(remaining))
		for _, account := range remaining {
			if account != nil && account.Priority == minPriority {
				pool = append(pool, account)
			} else {
				rest = append(rest, account)
			}
		}
		for len(pool) > 0 {
			selected := channelMonitorHealthWeightedAccount(ctx, mode, groupID, pool, provider, model, preferOAuth)
			if selected == nil {
				break
			}
			ordered = append(ordered, selected)
			nextPool := make([]*Account, 0, len(pool)-1)
			for _, account := range pool {
				if account.ID != selected.ID {
					nextPool = append(nextPool, account)
				}
			}
			pool = nextPool
		}
		remaining = rest
	}
	return ordered
}

func channelMonitorHealthLoadSelectionWeight(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, item accountWithLoad, peers []accountWithLoad, provider, model string, minLoadRate float64, preferOAuth bool) float64 {
	snapshot := channelMonitorHealthAccountSnapshot(ctx, mode, groupID, item.account, provider, model)
	peerAccounts := make([]*Account, 0, len(peers))
	for _, peer := range peers {
		peerAccounts = append(peerAccounts, peer.account)
	}
	if channelMonitorHealthHighConfidenceUnhealthy(snapshot) && channelMonitorHealthHasNonHighConfidenceCandidate(ctx, mode, groupID, peerAccounts, provider, model) {
		return 0
	}
	loadDistance := float64(item.loadInfo.LoadRate) - minLoadRate
	loadWeight := 1 - loadDistance/channelMonitorHealthLoadBand
	if loadWeight < channelMonitorHealthMinLoadWeight {
		loadWeight = channelMonitorHealthMinLoadWeight
	}
	weight := channelMonitorHealthWeight(snapshot) * loadWeight * channelMonitorHealthAccountLRUWeight(item.account, peerAccounts)
	if preferOAuth && item.account != nil && item.account.Type == AccountTypeOAuth {
		weight *= 1.10
	}
	return weight
}

func channelMonitorHealthWeightedLoadOrder(ctx context.Context, mode ChannelMonitorHealthGateMode, groupID *int64, candidates []accountWithLoad, provider, model string, preferOAuth bool) []accountWithLoad {
	if mode == ChannelMonitorHealthGateEnabled && len(candidates) > 0 {
		monitorHealthStats.EnabledDecisions.Add(1)
	}
	remaining := append([]accountWithLoad(nil), candidates...)
	ordered := make([]accountWithLoad, 0, len(remaining))
	for len(remaining) > 0 {
		minPriority := remaining[0].account.Priority
		for _, item := range remaining[1:] {
			if item.account.Priority < minPriority {
				minPriority = item.account.Priority
			}
		}
		pool := make([]accountWithLoad, 0, len(remaining))
		rest := make([]accountWithLoad, 0, len(remaining))
		for _, item := range remaining {
			if item.account.Priority == minPriority {
				pool = append(pool, item)
			} else {
				rest = append(rest, item)
			}
		}
		for len(pool) > 0 {
			minLoadRate := 101.0
			for _, item := range pool {
				rawLoad := float64(item.loadInfo.LoadRate)
				if rawLoad < minLoadRate {
					minLoadRate = rawLoad
				}
			}
			weights := make([]float64, len(pool))
			total := 0.0
			for i, item := range pool {
				weights[i] = channelMonitorHealthLoadSelectionWeight(ctx, mode, groupID, item, pool, provider, model, minLoadRate, preferOAuth)
				total += weights[i]
			}
			if total <= 0 {
				monitorHealthStats.UnhealthyFallback.Add(1)
				for i := range weights {
					weights[i] = 1
					total++
				}
			}
			draw := channelMonitorHealthRandomFloat64(ctx) * total
			selected := len(pool) - 1
			for i, weight := range weights {
				if draw < weight {
					selected = i
					break
				}
				draw -= weight
			}
			ordered = append(ordered, pool[selected])
			pool = append(pool[:selected], pool[selected+1:]...)
		}
		remaining = rest
	}
	return ordered
}

// filterByChannelMonitorHealthLoad preserves the existing hard priority tier
// and load-aware selection. Enabled mode keeps a bounded raw-load band and
// applies health only in the weighted draw; shadow mode returns the legacy
// minimum-load set unchanged.
func filterByChannelMonitorHealthLoad(mode ChannelMonitorHealthGateMode, candidates []accountWithLoad) []accountWithLoad {
	if len(candidates) == 0 {
		return candidates
	}
	if mode == ChannelMonitorHealthGateOff {
		return filterByMinLoadRate(candidates)
	}
	minLoad := 101.0
	for _, item := range candidates {
		rawLoad := float64(item.loadInfo.LoadRate)
		if rawLoad < minLoad {
			minLoad = rawLoad
		}
	}
	if mode == ChannelMonitorHealthGateShadow {
		return filterByMinLoadRate(candidates)
	}
	out := make([]accountWithLoad, 0, len(candidates))
	for _, item := range candidates {
		// Keep a bounded soft band around the least raw-loaded candidate. Health
		// is applied exactly once by channelMonitorHealthWeight below.
		if float64(item.loadInfo.LoadRate) <= minLoad+channelMonitorHealthLoadBand {
			out = append(out, item)
		}
	}
	return out
}

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

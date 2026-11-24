package service

import (
	"context"
	"time"
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

package service

import "context"

// applyChannelMonitorHealthToOpenAICandidates adds a bounded health influence
// to the advanced OpenAI scheduler. The existing score remains authoritative
// across priority tiers; health only nudges candidates that share the same
// static Priority. Shadow mode performs the same snapshot lookups and emits
// low-cardinality decision counters without changing candidate scores.
func (s *defaultOpenAIAccountScheduler) applyChannelMonitorHealthToOpenAICandidates(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
	candidates []openAIAccountCandidateScore,
) {
	if len(candidates) == 0 || s == nil || s.service == nil {
		return
	}
	mode := channelMonitorHealthMode(ctx, s.service.settingService)
	if mode == ChannelMonitorHealthGateOff || req.GroupID == nil {
		return
	}

	tierMin := make(map[int]float64)
	tierMax := make(map[int]float64)
	for _, candidate := range candidates {
		if candidate.account == nil {
			continue
		}
		snapshot := channelMonitorHealthSnapshotForSelection(ctx, mode, req.GroupID, candidate.account.ID, req.Platform, req.RequestedModel)
		rank := channelMonitorHealthRank(channelMonitorHealthStateForSelection(ctx, mode, req.GroupID, candidate.account.ID, req.Platform, req.RequestedModel))
		if snapshot != nil && channelMonitorHealthHighConfidenceUnhealthy(snapshot) {
			rank = -1
		}
		if mode == ChannelMonitorHealthGateEnabled {
			monitorHealthStats.EnabledDecisions.Add(1)
			if rank < 0 {
				monitorHealthStats.UnhealthyFallback.Add(1)
			}
		}
		if _, ok := tierMin[candidate.priority]; !ok || candidate.score < tierMin[candidate.priority] {
			tierMin[candidate.priority] = candidate.score
		}
		if _, ok := tierMax[candidate.priority]; !ok || candidate.score > tierMax[candidate.priority] {
			tierMax[candidate.priority] = candidate.score
		}
		if mode == ChannelMonitorHealthGateShadow {
			monitorHealthStats.ShadowDecisions.Add(1)
			if rank != channelMonitorHealthRank(ChannelMonitorHealthStateUnknown) {
				monitorHealthStats.ShadowDifferent.Add(1)
			}
		}
	}
	if mode != ChannelMonitorHealthGateEnabled {
		return
	}

	for i := range candidates {
		candidate := &candidates[i]
		if candidate.account == nil {
			continue
		}
		tierRange := tierMax[candidate.priority] - tierMin[candidate.priority]
		influence := tierRange * channelMonitorHealthOpenAIScoreInfluence
		if influence < channelMonitorHealthOpenAIMinScoreInfluence {
			influence = channelMonitorHealthOpenAIMinScoreInfluence
		}
		snapshot := channelMonitorHealthSnapshotForSelection(ctx, mode, req.GroupID, candidate.account.ID, req.Platform, req.RequestedModel)
		rank := channelMonitorHealthRank(channelMonitorHealthStateForSelection(ctx, mode, req.GroupID, candidate.account.ID, req.Platform, req.RequestedModel))
		if snapshot != nil && channelMonitorHealthHighConfidenceUnhealthy(snapshot) {
			rank = -1
		}
		// Unknown is the neutral baseline. Healthy gets a bounded positive
		// nudge; degraded/unhealthy receive progressively larger penalties.
		candidate.score += float64(rank-2) / 3.0 * influence
	}
}

// SelectAccountWithSchedulerForImagePlanning selects a general account for
// the first stage of the dedicated Codex image bridge. The stage is kept
// distinct from ordinary text scheduling so capacity and routing metrics can
// distinguish planning from the later image_only execution job.
func (s *OpenAIGatewayService) SelectAccountWithSchedulerForImagePlanning(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredCapability OpenAIEndpointCapability,
	requireCompact bool,
	previousResponseCanMove bool,
	useUpstreamTokenCost bool,
	platform string,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return s.selectAccountWithScheduler(
		ctx,
		groupID,
		previousResponseID,
		sessionHash,
		requestedModel,
		excludedIDs,
		OpenAIUpstreamTransportAny,
		requiredCapability,
		"",
		OpenAIRequestStageImagePlanning,
		AccountPurposeGeneral,
		requireCompact,
		platform,
		previousResponseCanMove,
		useUpstreamTokenCost,
	)
}

// SelectAccountWithSchedulerForDedicatedImages selects only image_only accounts.
// The optional general fallback is explicit and one-way: text requests never use
// image_only accounts, while a dedicated image request may use the legacy image
// pool only when the caller has enabled that policy.
func (s *OpenAIGatewayService) SelectAccountWithSchedulerForDedicatedImages(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredCapability OpenAIImagesCapability,
	allowGeneralFallback bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	selection, decision, err := s.selectAccountWithScheduler(
		ctx,
		groupID,
		"",
		sessionHash,
		requestedModel,
		excludedIDs,
		OpenAIUpstreamTransportHTTPSSE,
		"",
		requiredCapability,
		OpenAIRequestStageImageExecution,
		AccountPurposeImageOnly,
		false,
		PlatformOpenAI,
		false,
		false,
	)
	if err == nil && selection != nil && selection.Account != nil {
		return selection, decision, nil
	}
	if !allowGeneralFallback {
		return selection, decision, err
	}
	// General fallback is not a generic OpenAI Images fallback. The worker
	// still uses the Cangyuan adapter, so select only accounts that explicitly
	// provide Cangyuan credentials and a Cangyuan model mapping.
	return s.selectAccountWithScheduler(
		ctx,
		groupID,
		"",
		sessionHash,
		requestedModel,
		excludedIDs,
		OpenAIUpstreamTransportHTTPSSE,
		"",
		OpenAIImagesCapabilityCangyuan,
		OpenAIRequestStageImageExecution,
		AccountPurposeGeneral,
		false,
		PlatformOpenAI,
		false,
		false,
	)
}

func accountSupportsOpenAIRequest(account *Account, requestedModel string, requiredCapability OpenAIEndpointCapability, requiredImageCapability OpenAIImagesCapability, requiredAccountPurpose string) bool {
	if account == nil {
		return false
	}
	if requiredAccountPurpose == AccountPurposeImageOnly {
		if !account.IsImageOnly() {
			return false
		}
	} else if account.IsImageOnly() {
		// Missing/empty requirement is deliberately treated as general. This
		// protects every legacy call path, including newly added ones that have
		// not yet learned about request stages.
		return false
	}
	if requiredAccountPurpose == AccountPurposeImageOnly || requiredImageCapability == OpenAIImagesCapabilityCangyuan {
		mappedModel, matched := account.ResolveMappedModel(requestedModel)
		if !matched {
			return false
		}
		if !IsCangyuanImageModel(mappedModel) {
			return false
		}
	}
	return account.SupportsOpenAIEndpointCapability(requiredCapability) &&
		account.SupportsOpenAIImageCapability(requiredImageCapability)
}

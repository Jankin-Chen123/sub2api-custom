package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// MembershipFactor returns the highest currently effective activity factor.
// It deliberately does not read or write user_group_rate_multipliers.
func (s *NewcomerCampaignService) MembershipFactor(ctx context.Context, userID int64) float64 {
	if !s.enabled() || userID <= 0 {
		return 1
	}
	now := s.currentTime()
	s.membershipFactorMu.Lock()
	if entry, ok := s.membershipFactorCache[userID]; ok && now.Before(entry.expiresAt) {
		s.membershipFactorMu.Unlock()
		return entry.factor
	}
	version := s.membershipFactorVersion
	s.membershipFactorMu.Unlock()

	flightKey := fmt.Sprintf("%d:%d", userID, version)
	value, _, _ := s.membershipFactorFlight.Do(flightKey, func() (any, error) {
		// Re-check after joining an in-flight lookup. The first caller may have
		// populated the cache between the initial check and singleflight join.
		lookupNow := s.currentTime()
		s.membershipFactorMu.Lock()
		if entry, ok := s.membershipFactorCache[userID]; ok && lookupNow.Before(entry.expiresAt) {
			s.membershipFactorMu.Unlock()
			return membershipFactorLookup{factor: entry.factor}, nil
		}
		lookupVersion := s.membershipFactorVersion
		s.membershipFactorMu.Unlock()

		var factor float64
		var membershipExpiresAt time.Time
		err := s.queryOne(ctx, s.db, `
SELECT effective.factor, effective.expires_at
FROM (
    SELECT g.factor, g.expires_at, g.threshold AS sort_threshold, 0 AS priority
    FROM newcomer_campaign_membership_grants g
    WHERE g.campaign_key = $1 AND g.user_id = $2 AND g.status = 'active'
      AND g.starts_at <= $3 AND g.expires_at > $3
      AND g.threshold <= (
          SELECT COUNT(*) FROM newcomer_campaign_invites i
          WHERE i.campaign_key = g.campaign_key AND i.inviter_id = g.user_id
            AND i.status = 'qualified' AND i.qualifying_amount >= $4
      )
    UNION ALL
    SELECT m.factor, m.expires_at, 2147483647 AS sort_threshold, 1 AS priority
    FROM newcomer_campaign_admin_memberships m
    WHERE m.campaign_key = $1 AND m.user_id = $2 AND m.status = 'active'
      AND m.starts_at <= $3 AND m.expires_at > $3
) effective
ORDER BY effective.priority DESC, effective.sort_threshold DESC
		LIMIT 1`, []any{NewcomerCampaignKey, userID, lookupNow, newcomerInviteThreshold}, &factor, &membershipExpiresAt)
		lookup := membershipFactorLookup{factor: factor, cache: false}
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// No active membership is a normal result and is safe to cache for
			// the regular short TTL.
			lookup.factor = 1
			lookup.cache = true
			lookup.cacheExpires = lookupNow.Add(newcomerMembershipFactorCacheTTL)
		case err != nil:
			// A transient database/context failure must not become a cached
			// five-second billing decision.
			lookup.factor = 1
		case factor <= 0 || factor > 1 || membershipExpiresAt.IsZero():
			// Invalid persisted state is fail-open for this request but is not
			// cached; operators can repair it without waiting for a TTL.
			lookup.factor = 1
		default:
			lookup.cache = true
			lookup.cacheExpires = lookupNow.Add(newcomerMembershipFactorCacheTTL)
			if membershipExpiresAt.Before(lookup.cacheExpires) {
				lookup.cacheExpires = membershipExpiresAt
			}
			if !lookup.cacheExpires.After(lookupNow) {
				lookup.cache = false
			}
		}
		s.membershipFactorMu.Lock()
		if lookup.cache && lookupVersion == s.membershipFactorVersion {
			if s.membershipFactorCache == nil {
				s.membershipFactorCache = make(map[int64]membershipFactorCacheEntry)
			}
			s.membershipFactorCache[userID] = membershipFactorCacheEntry{factor: lookup.factor, expiresAt: lookup.cacheExpires}
		}
		s.membershipFactorMu.Unlock()
		return lookup, nil
	})
	if lookup, ok := value.(membershipFactorLookup); ok {
		return lookup.factor
	}
	return 1
}

// invalidateMembershipFactor drops the short-lived billing multiplier cache
// for one user after any campaign state change. The cache is deliberately
// local to a service instance; the TTL remains the safety net for changes
// made by another process.
func (s *NewcomerCampaignService) invalidateMembershipFactor(userID int64) {
	if s == nil || userID <= 0 {
		return
	}
	s.membershipFactorMu.Lock()
	delete(s.membershipFactorCache, userID)
	oldVersion := s.membershipFactorVersion
	s.membershipFactorVersion++
	s.membershipFactorMu.Unlock()
	s.membershipFactorFlight.Forget(fmt.Sprintf("%d:%d", userID, oldVersion))
}

// invalidateAllMembershipFactors is used when the effective configuration or
// test clock changes and the affected user set is not known.
func (s *NewcomerCampaignService) invalidateAllMembershipFactors() {
	if s == nil {
		return
	}
	s.membershipFactorMu.Lock()
	s.membershipFactorCache = make(map[int64]membershipFactorCacheEntry)
	s.membershipFactorVersion++
	s.membershipFactorMu.Unlock()
}

// ApplyMembershipFactor is the final-layer multiplier operation used by all
// billing paths. The base value must already be the resolved group/user rate.
func (s *NewcomerCampaignService) ApplyMembershipFactor(ctx context.Context, userID int64, base float64) float64 {
	if base < 0 {
		base = 0
	}
	return base * s.MembershipFactor(ctx, userID)
}

func (s *NewcomerCampaignService) GetStatus(ctx context.Context, userID int64, origin string) (*NewcomerCampaignStatus, error) {
	if !s.enabled() {
		return nil, errors.New("newcomer campaign service is unavailable")
	}
	start, end := s.campaignWindow(ctx)
	now := s.currentTime()
	phase := "upcoming"
	if !now.Before(start) && now.Before(end) {
		phase = "active"
	} else if !now.Before(end) {
		phase = "ended"
	}
	var tiers []NewcomerCampaignTier
	if s.persistentConfig {
		var err error
		tiers, err = loadNewcomerCampaignTiers(ctx, s.db)
		if err != nil {
			return nil, fmt.Errorf("load newcomer campaign tier config: %w", err)
		}
	} else {
		tiers = s.configuredCampaignTiers(ctx)
	}
	status := &NewcomerCampaignStatus{
		CampaignKey: NewcomerCampaignKey,
		Name:        NewcomerCampaignName,
		Phase:       phase,
		StartsAt:    start,
		EndsAt:      end,
		Tiers:       tiers,
	}
	code, err := s.loadCampaignInviteCode(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(origin) == "" {
		origin = ""
	}
	if strings.TrimSpace(code) != "" {
		status.InviteLink = strings.TrimRight(origin, "/") + "/register?aff=" + code
	}
	if err := s.queryOne(ctx, s.db, `
SELECT COUNT(*) FROM newcomer_campaign_invites
WHERE campaign_key = $1 AND inviter_id = $2 AND status = 'qualified'
  AND qualifying_amount >= $3`, []any{NewcomerCampaignKey, userID, newcomerInviteThreshold}, &status.ValidInviteCount); err != nil {
		return nil, fmt.Errorf("load valid campaign invitations: %w", err)
	}
	status.FirstRecharge, err = s.firstRechargeStatus(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}
	membership, err := s.loadEffectiveMembership(ctx, userID, now, status.ValidInviteCount)
	if err != nil {
		return nil, err
	}
	status.CurrentMembership = membership
	status.NextTier, status.NextTierRemaining = nextNewcomerCampaignTier(status.Tiers, status.ValidInviteCount, membership)
	status.NextTierProgress = status.ValidInviteCount
	return status, nil
}

func (s *NewcomerCampaignService) ensureCampaignInviteCode(ctx context.Context, userID int64) (string, error) {
	if !s.enabled() || userID <= 0 {
		return "", nil
	}
	client := s.db
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
	// Once a code is recorded for an inviter, it is the activity's stable
	// identity. A later ordinary-affiliate code change must not change links or
	// allow a referral intent to resolve to a different inviter identity.
	var code string
	err := s.queryOne(ctx, client, `
SELECT invite_code
FROM newcomer_campaign_invite_codes
WHERE campaign_key = $1 AND inviter_id = $2
ORDER BY created_at ASC, invite_code ASC
LIMIT 1`, []any{NewcomerCampaignKey, userID}, &code)
	if err == nil {
		return strings.TrimSpace(code), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("load campaign invite code mapping: %w", err)
	}

	if s.affiliateEnsurer != nil {
		affiliateSummary, err := s.affiliateEnsurer.EnsureUserAffiliate(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("ensure campaign invite profile: %w", err)
		}
		if affiliateSummary == nil {
			return "", nil
		}
		code = strings.TrimSpace(affiliateSummary.AffCode)
	} else {
		if err := s.queryOne(ctx, s.db, `SELECT COALESCE(aff_code, '') FROM user_affiliates WHERE user_id = $1`, []any{userID}, &code); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("load campaign invite code: %w", err)
		}
	}
	code = strings.TrimSpace(code)
	if code != "" {
		if _, err := client.ExecContext(ctx, `
INSERT INTO newcomer_campaign_invite_codes (campaign_key, invite_code, inviter_id, source)
VALUES ($1, $2, $3, 'campaign')
ON CONFLICT (campaign_key, invite_code) DO NOTHING`, NewcomerCampaignKey, code, userID); err != nil {
			return "", fmt.Errorf("persist campaign invite code mapping: %w", err)
		}
		// A custom code can race with another inviter's mapping. Only return a
		// link when the activity-owned row is actually owned by this user.
		var ownerID int64
		if err := s.queryOne(ctx, client, `
SELECT inviter_id
FROM newcomer_campaign_invite_codes
WHERE campaign_key = $1 AND invite_code = $2`, []any{NewcomerCampaignKey, code}, &ownerID); err != nil {
			return "", fmt.Errorf("verify campaign invite code mapping: %w", err)
		}
		if ownerID != userID {
			return "", nil
		}
	}
	return code, nil
}

// loadCampaignInviteCode is the read-only status path.  It deliberately does
// not call EnsureUserAffiliate, because rendering the status page must not
// create or mutate an ordinary affiliate profile.
func (s *NewcomerCampaignService) loadCampaignInviteCode(ctx context.Context, userID int64) (string, error) {
	if !s.enabled() || userID <= 0 {
		return "", nil
	}
	var code string
	err := s.queryOne(ctx, s.db, `
SELECT invite_code
FROM newcomer_campaign_invite_codes
WHERE campaign_key = $1 AND inviter_id = $2`, []any{NewcomerCampaignKey, userID}, &code)
	if errors.Is(err, sql.ErrNoRows) {
		// GET is intentionally read-only and only exposes activity-owned facts.
		// Legacy profiles are imported by migration or explicit reconciliation.
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load campaign invite code mapping: %w", err)
	}
	return strings.TrimSpace(code), nil
}

func (s *NewcomerCampaignService) firstRechargeStatus(ctx context.Context, userID int64, start, end time.Time) (NewcomerFirstRechargeStatus, error) {
	status := NewcomerFirstRechargeStatus{RewardStatus: "pending", RewardAmount: newcomerRewardAmount}
	var registeredAt time.Time
	if s.persistentConfig {
		eligibility, err := s.loadCampaignEligibility(ctx, userID)
		if err != nil {
			return status, err
		}
		if eligibility == nil {
			status.RewardStatus = "ineligible"
			return status, nil
		}
		registeredAt = eligibility.RegisteredAt
		start, end = eligibility.WindowStart, eligibility.WindowEnd
	} else if err := s.queryOne(ctx, s.db, `SELECT created_at FROM users WHERE id = $1`, []any{userID}, &registeredAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return status, nil
		}
		return status, err
	}
	status.Eligible = !registeredAt.Before(start) && registeredAt.Before(end)
	var orderID int64
	var principalAmount sql.NullFloat64
	var principalCurrency sql.NullString
	var completedAt time.Time
	var refundAmount float64
	var orderStatus string
	err := s.queryOne(ctx, s.db, `
SELECT po.id, f.principal_amount, f.principal_currency, po.completed_at,
       COALESCE(po.refund_amount, 0), po.status
FROM payment_orders po
LEFT JOIN newcomer_campaign_payment_facts f ON f.order_id = po.id
WHERE po.user_id = $1 AND po.order_type = 'balance' AND po.completed_at IS NOT NULL
ORDER BY po.completed_at ASC, po.id ASC LIMIT 1`, []any{userID}, &orderID, &principalAmount, &principalCurrency, &completedAt, &refundAmount, &orderStatus)
	if errors.Is(err, sql.ErrNoRows) {
		// A pending first recharge is actionable only while the campaign is
		// active. Once the campaign closes there can be no later qualifying
		// first payment, so expose the terminal state to clients as well.
		if !status.Eligible || !s.currentTime().Before(end) {
			status.RewardStatus = "ineligible"
		}
		return status, nil
	}
	if err != nil {
		return status, err
	}
	status.FirstOrderID = &orderID
	if principalAmount.Valid {
		status.FirstAmount = &principalAmount.Float64
	}
	status.FirstCompletedAt = &completedAt
	if !principalAmount.Valid || !principalCurrency.Valid {
		s.logMissingPaymentFact(orderID, userID)
		status.RewardStatus = "ineligible"
		return status, nil
	}
	if !status.Eligible || completedAt.Before(start) || !completedAt.Before(end) || principalCurrency.String != newcomerPrincipalCurrency || principalAmount.Float64 < newcomerInviteThreshold {
		status.RewardStatus = "ineligible"
		return status, nil
	}
	if isSuccessfulPaymentRefund(orderStatus) {
		status.RewardStatus = "revoked"
		return status, nil
	}
	var granted, revoked int
	if err := s.queryOne(ctx, s.db, `
SELECT
  COUNT(*) FILTER (WHERE entry_type = 'grant'),
  COUNT(*) FILTER (WHERE entry_type = 'revoke')
FROM newcomer_campaign_reward_ledger
WHERE campaign_key = $1 AND user_id = $2 AND source_order_id = $3 AND reward_type = 'first_recharge'`, []any{NewcomerCampaignKey, userID, orderID}, &granted, &revoked); err != nil {
		return status, err
	}
	switch {
	case revoked > 0:
		status.RewardStatus = "revoked"
	case granted > 0:
		status.RewardStatus = "granted"
	default:
		status.RewardStatus = "qualified"
	}
	return status, nil
}

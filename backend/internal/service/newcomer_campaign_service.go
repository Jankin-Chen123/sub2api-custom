package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	NewcomerCampaignKey             = "newcomer_202609"
	NewcomerCampaignName            = "迎新活动"
	newcomerRewardAmount            = 2.0
	newcomerInviteThreshold         = 10.0
	newcomerPrincipalCurrency       = "CNY"
	newcomerCampaignCaptureGrace    = 14 * 24 * time.Hour
	newcomerCampaignRepairBatchSize = 500
)

var newcomerCampaignLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

// NewcomerCampaignWindow returns the campaign window in UTC. The public
// campaign definition is in Asia/Shanghai; persistence and comparisons use
// UTC instants so the result is independent of the process timezone.
func NewcomerCampaignWindow() (time.Time, time.Time) {
	start := time.Date(2026, time.September, 1, 0, 0, 0, 0, newcomerCampaignLocation)
	end := time.Date(2026, time.October, 1, 0, 0, 0, 0, newcomerCampaignLocation)
	return start.UTC(), end.UTC()
}

type NewcomerCampaignTier struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Threshold    int     `json:"threshold"`
	Factor       float64 `json:"factor"`
	DurationDays int     `json:"duration_days"`
}

var newcomerCampaignTiers = []NewcomerCampaignTier{
	{Key: "premium", Name: "高级", Threshold: 2, Factor: 0.98, DurationDays: 30},
	{Key: "gold", Name: "黄金", Threshold: 5, Factor: 0.96, DurationDays: 45},
	{Key: "diamond", Name: "钻石", Threshold: 10, Factor: 0.94, DurationDays: 60},
}

func NewcomerCampaignTiers() []NewcomerCampaignTier {
	return append([]NewcomerCampaignTier(nil), newcomerCampaignTiers...)
}

type NewcomerFirstRechargeStatus struct {
	Eligible         bool       `json:"eligible"`
	FirstOrderID     *int64     `json:"first_order_id,omitempty"`
	FirstAmount      *float64   `json:"first_amount,omitempty"`
	FirstCompletedAt *time.Time `json:"first_completed_at,omitempty"`
	RewardStatus     string     `json:"reward_status"`
	RewardAmount     float64    `json:"reward_amount"`
}

type NewcomerMembershipStatus struct {
	TierKey   string    `json:"tier_key"`
	TierName  string    `json:"tier_name"`
	Factor    float64   `json:"factor"`
	StartsAt  time.Time `json:"starts_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type NewcomerCampaignStatus struct {
	CampaignKey       string                      `json:"campaign_key"`
	Name              string                      `json:"name"`
	Phase             string                      `json:"phase"`
	StartsAt          time.Time                   `json:"starts_at"`
	EndsAt            time.Time                   `json:"ends_at"`
	FirstRecharge     NewcomerFirstRechargeStatus `json:"first_recharge"`
	InviteLink        string                      `json:"invite_link"`
	ValidInviteCount  int                         `json:"valid_invite_count"`
	NextTier          *NewcomerCampaignTier       `json:"next_tier,omitempty"`
	NextTierProgress  int                         `json:"next_tier_progress"`
	NextTierRemaining int                         `json:"next_tier_remaining"`
	CurrentMembership *NewcomerMembershipStatus   `json:"current_membership,omitempty"`
	Tiers             []NewcomerCampaignTier      `json:"tiers"`
}

// nextNewcomerCampaignTier selects the first configured tier that is both
// above the user's effective membership and not yet reached by invite count.
// The tier order and thresholds come from the persisted admin configuration;
// the frozen defaults are deliberately not consulted here.
func nextNewcomerCampaignTier(tiers []NewcomerCampaignTier, validInviteCount int, membership *NewcomerMembershipStatus) (*NewcomerCampaignTier, int) {
	startIndex := 0
	if membership != nil {
		for i := range tiers {
			if tiers[i].Key == membership.TierKey {
				startIndex = i + 1
				break
			}
		}
	}
	for i := startIndex; i < len(tiers); i++ {
		if validInviteCount < tiers[i].Threshold {
			return &tiers[i], tiers[i].Threshold - validInviteCount
		}
	}
	return nil, 0
}

type newcomerCampaignClock func() time.Time

type newcomerAffiliateEnsurer interface {
	EnsureUserAffiliate(context.Context, int64) (*AffiliateSummary, error)
}

type newcomerBalanceCacheInvalidator interface {
	InvalidateUserBalance(context.Context, int64) error
}

type NewcomerCampaignSQLClient interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type NewcomerCampaignRepository interface {
	NewcomerCampaignSQLClient
	ClientForContext(ctx context.Context) NewcomerCampaignSQLClient
	WithTx(ctx context.Context, fn func(txCtx context.Context, tx NewcomerCampaignSQLClient) error) error
}

type NewcomerCampaignService struct {
	db                      NewcomerCampaignRepository
	now                     newcomerCampaignClock
	persistentConfig        bool
	affiliateEnsurer        newcomerAffiliateEnsurer
	balanceCacheInvalidator newcomerBalanceCacheInvalidator
	authCacheInvalidator    APIKeyAuthCacheInvalidator
	membershipFactorMu      sync.Mutex
	membershipFactorCache   map[int64]membershipFactorCacheEntry
	membershipFactorFlight  singleflight.Group
	membershipFactorVersion uint64
}

const newcomerMembershipFactorCacheTTL = 5 * time.Second

type membershipFactorCacheEntry struct {
	factor    float64
	expiresAt time.Time
}

type membershipFactorLookup struct {
	factor       float64
	cache        bool
	cacheExpires time.Time
}

func NewNewcomerCampaignServiceWithRepository(repo NewcomerCampaignRepository, affiliateEnsurers ...newcomerAffiliateEnsurer) *NewcomerCampaignService {
	var affiliateEnsurer newcomerAffiliateEnsurer
	if len(affiliateEnsurers) > 0 {
		affiliateEnsurer = affiliateEnsurers[0]
	}
	return &NewcomerCampaignService{
		db:                    repo,
		now:                   func() time.Time { return time.Now().UTC() },
		affiliateEnsurer:      affiliateEnsurer,
		membershipFactorCache: make(map[int64]membershipFactorCacheEntry),
	}
}

// SetPersistentConfigEnabled enables the database-backed long-lived campaign
// window. Focused unit tests may leave it disabled to exercise the legacy
// September defaults without requiring the follow-up migration.
func (s *NewcomerCampaignService) SetPersistentConfigEnabled(enabled bool) {
	if s == nil {
		return
	}
	s.persistentConfig = enabled
	s.invalidateAllMembershipFactors()
}

// SetCacheInvalidators wires the caches that contain user balance snapshots.
// It is separate from the test-friendly constructor because the campaign
// service itself remains usable with only an Ent client in focused tests.
func (s *NewcomerCampaignService) SetCacheInvalidators(balanceCache newcomerBalanceCacheInvalidator, authCache APIKeyAuthCacheInvalidator) {
	if s == nil {
		return
	}
	s.balanceCacheInvalidator = balanceCache
	s.authCacheInvalidator = authCache
}

// SetClock is intended for deterministic service tests. Production callers
// should leave the default UTC clock in place.
func (s *NewcomerCampaignService) SetClock(clock func() time.Time) {
	if s == nil || clock == nil {
		return
	}
	s.now = clock
	s.invalidateAllMembershipFactors()
}

func (s *NewcomerCampaignService) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

// campaignWindow returns the currently configured public window. The
// database-backed setting is deliberately read at event time: changing the
// dates reopens the same campaign key without resetting invitation history.
// Eligibility snapshots (see ensureCampaignEligibility) protect users that
// registered under an earlier window.
func (s *NewcomerCampaignService) campaignWindow(ctx context.Context) (time.Time, time.Time) {
	if s == nil || !s.persistentConfig || s.db == nil {
		return NewcomerCampaignWindow()
	}
	var start, end time.Time
	if err := s.queryOne(ctx, s.db, `
SELECT starts_at, ends_at
FROM newcomer_campaign_config
WHERE campaign_key = $1`, []any{NewcomerCampaignKey}, &start, &end); err != nil {
		// Normal registration/payment paths remain fail-open during a rolling
		// migration. The admin endpoint is strict and reports missing config.
		return NewcomerCampaignWindow()
	}
	return start.UTC(), end.UTC()
}

func (s *NewcomerCampaignService) loadConfiguredWindowStrict(ctx context.Context, q campaignQueryer) (time.Time, time.Time, error) {
	var start, end time.Time
	if err := s.queryOne(ctx, q, `
SELECT starts_at, ends_at
FROM newcomer_campaign_config
WHERE campaign_key = $1`, []any{NewcomerCampaignKey}, &start, &end); err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, errors.New("newcomer campaign window must end after it starts")
	}
	return start.UTC(), end.UTC(), nil
}

func (s *NewcomerCampaignService) enabled() bool {
	return s != nil && s.db != nil
}

// OnUserRegistered independently binds the activity invitation. It never
// consults the ordinary affiliate-rebate switch and therefore remains valid
// when cash rebates are disabled.
func (s *NewcomerCampaignService) OnUserRegistered(ctx context.Context, userID int64, affiliateCode string) error {
	return s.OnUserRegisteredWithSource(ctx, userID, affiliateCode, "unknown")
}

// PersistReferralIntent is the transaction-safe part of the registration
// hook.  Registration code calls this before committing the newly-created
// user, while the post-commit hook may still retry materialization later.
func (s *NewcomerCampaignService) PersistReferralIntent(ctx context.Context, userID int64, affiliateCode, signupSource string) error {
	affiliateCode = strings.ToUpper(strings.TrimSpace(affiliateCode))
	if !s.enabled() || userID <= 0 || affiliateCode == "" {
		return nil
	}
	signupSource = strings.ToLower(strings.TrimSpace(signupSource))
	if signupSource == "" {
		signupSource = "unknown"
	}
	client := s.db.ClientForContext(ctx)
	if err := s.ensureCampaignEligibilityWithClient(ctx, client, userID); err != nil {
		return err
	}
	_, err := client.ExecContext(ctx, `
INSERT INTO newcomer_campaign_referral_intents
    (campaign_key, invitee_id, invite_code, signup_source)
VALUES ($1, $2, $3, $4)
ON CONFLICT (campaign_key, invitee_id) DO UPDATE
SET invite_code = CASE
        WHEN newcomer_campaign_referral_intents.status = 'bound'
        THEN newcomer_campaign_referral_intents.invite_code
        ELSE EXCLUDED.invite_code
    END,
    signup_source = CASE
        WHEN newcomer_campaign_referral_intents.status = 'bound'
        THEN newcomer_campaign_referral_intents.signup_source
			ELSE EXCLUDED.signup_source
		END,
		updated_at = NOW(),
		last_error = NULL`, NewcomerCampaignKey, userID, affiliateCode, signupSource)
	if err != nil {
		return fmt.Errorf("persist newcomer campaign referral intent: %w", err)
	}
	if s.persistentConfig {
		if _, err := client.ExecContext(ctx, `
UPDATE newcomer_campaign_referral_intents r
SET eligible_at_registration = TRUE,
    registration_window_start = e.window_start,
    registration_window_end = e.window_end,
    qualification_capture_deadline = e.capture_deadline,
    updated_at = NOW()
FROM newcomer_campaign_eligible_users e
WHERE r.campaign_key = $1 AND r.invitee_id = $2
  AND e.campaign_key = r.campaign_key AND e.user_id = r.invitee_id`, NewcomerCampaignKey, userID); err != nil {
			return fmt.Errorf("snapshot newcomer campaign referral eligibility: %w", err)
		}
	}
	return nil
}

// OnUserRegisteredWithSource is the source-aware registration hook used by
// email and OAuth creation paths.  The source is only diagnostic; referral
// binding remains independent from the ordinary affiliate feature switch.
func (s *NewcomerCampaignService) OnUserRegisteredWithSource(ctx context.Context, userID int64, affiliateCode, signupSource string) error {
	if !s.enabled() || userID <= 0 || strings.TrimSpace(affiliateCode) == "" {
		return nil
	}
	if err := s.PersistReferralIntent(ctx, userID, affiliateCode, signupSource); err != nil {
		return err
	}
	return s.materializeReferralIntent(ctx, userID)
}

// EnsureCampaignInviteCode makes the activity-owned code mapping durable for
// an inviter. It is intentionally independent of the ordinary affiliate
// rebate switch and is safe to call from signup bootstrap and reconciliation.
func (s *NewcomerCampaignService) EnsureCampaignInviteCode(ctx context.Context, userID int64) error {
	if !s.enabled() || userID <= 0 {
		return nil
	}
	if err := s.ensureCampaignEligibility(ctx, userID); err != nil {
		return err
	}
	if _, err := s.ensureCampaignInviteCode(ctx, userID); err != nil {
		return err
	}
	return nil
}

type newcomerCampaignEligibility struct {
	RegisteredAt    time.Time
	WindowStart     time.Time
	WindowEnd       time.Time
	CaptureDeadline time.Time
}

// ensureCampaignEligibility stores the one-time event-time decision for a
// user. It is intentionally a no-op for the legacy test/fallback service; the
// production provider enables the follow-up migration-backed configuration.
func (s *NewcomerCampaignService) ensureCampaignEligibility(ctx context.Context, userID int64) error {
	if !s.enabled() || !s.persistentConfig || userID <= 0 {
		return nil
	}
	client := s.db.ClientForContext(ctx)
	return s.ensureCampaignEligibilityWithClient(ctx, client, userID)
}

func (s *NewcomerCampaignService) ensureCampaignEligibilityWithClient(ctx context.Context, client campaignDBQuerier, userID int64) error {
	if !s.enabled() || !s.persistentConfig || userID <= 0 {
		return nil
	}
	start, end := s.campaignWindow(ctx)
	if s.persistentConfig {
		if configuredStart, configuredEnd, err := s.loadConfiguredWindowStrict(ctx, client); err == nil {
			start, end = configuredStart, configuredEnd
		}
	}
	_, err := client.ExecContext(ctx, `
INSERT INTO newcomer_campaign_eligible_users
    (campaign_key, user_id, registered_at, window_start, window_end, capture_deadline)
SELECT $1, u.id, u.created_at, $2::timestamptz, $3::timestamptz, $3::timestamptz + INTERVAL '14 days'
FROM users u
WHERE u.id = $4 AND u.created_at >= $2::timestamptz AND u.created_at < $3::timestamptz
ON CONFLICT (campaign_key, user_id) DO NOTHING`, NewcomerCampaignKey, start, end, userID)
	if err != nil {
		return fmt.Errorf("persist newcomer campaign eligibility snapshot: %w", err)
	}
	return nil
}

func (s *NewcomerCampaignService) loadCampaignEligibility(ctx context.Context, userID int64) (*newcomerCampaignEligibility, error) {
	if !s.enabled() || userID <= 0 {
		return nil, nil
	}
	var eligibility newcomerCampaignEligibility
	err := s.queryOne(ctx, s.db, `
SELECT registered_at, window_start, window_end, capture_deadline
FROM newcomer_campaign_eligible_users
WHERE campaign_key = $1 AND user_id = $2`, []any{NewcomerCampaignKey, userID},
		&eligibility.RegisteredAt, &eligibility.WindowStart, &eligibility.WindowEnd, &eligibility.CaptureDeadline)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load newcomer campaign eligibility snapshot: %w", err)
	}
	return &eligibility, nil
}

// RecordPaymentFactInTx records an online balance principal when the user has
// an eligibility snapshot and the order was created during that snapshot's
// capture period. The check is made against immutable event timestamps, so a
// later public-window edit cannot strand an old invite's 14-day payment.
func (s *NewcomerCampaignService) RecordPaymentFactInTx(ctx context.Context, tx campaignExecQuerier, orderID, userID int64, amount float64, currency string) error {
	if !s.enabled() || !s.persistentConfig || orderID <= 0 || userID <= 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO newcomer_campaign_payment_facts
    (order_id, user_id, principal_amount, principal_currency)
SELECT $1, $2, $3, $4
WHERE EXISTS (
    SELECT 1
    FROM newcomer_campaign_eligible_users e
    JOIN payment_orders po ON po.id = $1
    WHERE e.campaign_key = $5 AND e.user_id = $2
      AND po.created_at >= e.registered_at
      AND po.created_at < e.capture_deadline
)
ON CONFLICT (order_id) DO NOTHING`, orderID, userID, amount, currency, NewcomerCampaignKey)
	if err != nil {
		return fmt.Errorf("record newcomer campaign payment principal: %w", err)
	}
	return nil
}

// materializeReferralIntent resolves a persisted referral intent through the
// activity-owned code map.  A one-time legacy import is allowed only to seed
// that map; the invitation itself never joins user_affiliates directly.
func (s *NewcomerCampaignService) materializeReferralIntent(ctx context.Context, userID int64) error {
	if !s.enabled() || userID <= 0 {
		return nil
	}
	start, end := s.campaignWindow(ctx)
	client := s.db.ClientForContext(ctx)
	if _, err := client.ExecContext(ctx, `
INSERT INTO newcomer_campaign_invite_codes (campaign_key, invite_code, inviter_id, source)
SELECT $1, UPPER(TRIM(ua.aff_code)), ua.user_id, 'affiliate_recovery'
FROM user_affiliates ua
JOIN newcomer_campaign_referral_intents r
  ON r.campaign_key = $1 AND r.invitee_id = $2 AND UPPER(TRIM(ua.aff_code)) = r.invite_code
WHERE ua.aff_code <> ''
ON CONFLICT (campaign_key, invite_code) DO NOTHING`, NewcomerCampaignKey, userID); err != nil {
		return fmt.Errorf("seed newcomer campaign invite code mapping: %w", err)
	}
	materializeQuery := `
WITH resolved AS (
    SELECT r.invitee_id, r.invite_code, c.inviter_id, u.created_at
    FROM newcomer_campaign_referral_intents r
    JOIN newcomer_campaign_invite_codes c
      ON c.campaign_key = r.campaign_key AND c.invite_code = r.invite_code
    JOIN users u ON u.id = r.invitee_id
    %s
), inserted AS (
    INSERT INTO newcomer_campaign_invites
        (campaign_key, inviter_id, invitee_id, invite_code, registered_at, qualification_deadline)
    SELECT $1, resolved.inviter_id, resolved.invitee_id, resolved.invite_code,
           resolved.created_at, resolved.created_at + INTERVAL '14 days'
    FROM resolved
    WHERE resolved.inviter_id <> resolved.invitee_id
      %s
    ON CONFLICT (campaign_key, invitee_id) DO NOTHING
    RETURNING invitee_id
)
UPDATE newcomer_campaign_referral_intents r
SET inviter_id = resolved.inviter_id,
    status = CASE WHEN resolved.inviter_id = resolved.invitee_id THEN 'invalid' ELSE 'bound' END,
    bound_at = CASE WHEN resolved.inviter_id <> resolved.invitee_id THEN COALESCE(r.bound_at, NOW()) ELSE r.bound_at END,
    attempts = r.attempts + 1,
    updated_at = NOW(), last_error = NULL
FROM resolved
WHERE r.campaign_key = $1 AND r.invitee_id = $2`
	resolvedFilter := "WHERE r.campaign_key = $1 AND r.invitee_id = $2"
	registrationFilter := "AND resolved.created_at >= $3 AND resolved.created_at < $4"
	args := []any{NewcomerCampaignKey, userID, start, end}
	if s.persistentConfig {
		resolvedFilter = `WHERE r.campaign_key = $1 AND r.invitee_id = $2
      AND r.eligible_at_registration`
		registrationFilter = ""
		args = args[:2]
	}
	materializeQuery = fmt.Sprintf(materializeQuery, resolvedFilter, registrationFilter)
	_, err := client.ExecContext(ctx, materializeQuery, args...)
	if err != nil {
		return fmt.Errorf("materialize newcomer campaign referral intent: %w", err)
	}
	// Keep unresolved intents visibly pending and retryable.  In particular,
	// do not mark a missing mapping invalid: the inviter's campaign code may be
	// issued or restored after this registration attempt.
	if _, err := client.ExecContext(ctx, `
UPDATE newcomer_campaign_referral_intents r
SET attempts = attempts + 1, status = 'pending',
    last_error = 'campaign invite code mapping unavailable', updated_at = NOW()
WHERE campaign_key = $1 AND invitee_id = $2 AND status <> 'bound'
  AND NOT EXISTS (
      SELECT 1 FROM newcomer_campaign_invite_codes c
      WHERE c.campaign_key = r.campaign_key AND c.invite_code = r.invite_code
  )`, NewcomerCampaignKey, userID); err != nil {
		return fmt.Errorf("record pending newcomer campaign referral: %w", err)
	}
	return nil
}

func (s *NewcomerCampaignService) withTx(ctx context.Context, fn func(context.Context, NewcomerCampaignSQLClient) error) error {
	if !s.enabled() {
		return errors.New("newcomer campaign repository is not configured")
	}
	return s.db.WithTx(ctx, fn)
}

type campaignQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type campaignExecQuerier interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type campaignDBQuerier interface {
	campaignQueryer
	campaignExecQuerier
}

func (s *NewcomerCampaignService) queryOne(ctx context.Context, q campaignQueryer, query string, args []any, dest ...any) error {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Scan(dest...)
}

func tierName(key string) string {
	for _, tier := range newcomerCampaignTiers {
		if tier.Key == key {
			return tier.Name
		}
	}
	return key
}

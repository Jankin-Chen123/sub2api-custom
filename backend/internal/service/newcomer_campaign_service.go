package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
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

type newcomerCampaignClock func() time.Time

type newcomerAffiliateEnsurer interface {
	EnsureUserAffiliate(context.Context, int64) (*AffiliateSummary, error)
}

type newcomerBalanceCacheInvalidator interface {
	InvalidateUserBalance(context.Context, int64) error
}

type NewcomerCampaignService struct {
	db                      *dbent.Client
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

func NewNewcomerCampaignService(entClient *dbent.Client, affiliateEnsurers ...newcomerAffiliateEnsurer) *NewcomerCampaignService {
	var affiliateEnsurer newcomerAffiliateEnsurer
	if len(affiliateEnsurers) > 0 {
		affiliateEnsurer = affiliateEnsurers[0]
	}
	return &NewcomerCampaignService{
		db:                    entClient,
		now:                   func() time.Time { return time.Now().UTC() },
		affiliateEnsurer:      affiliateEnsurer,
		membershipFactorCache: make(map[int64]membershipFactorCacheEntry),
	}
}

func ProvideNewcomerCampaignService(entClient *dbent.Client, affiliateService *AffiliateService, billingCacheService *BillingCacheService, authCacheInvalidator APIKeyAuthCacheInvalidator) *NewcomerCampaignService {
	svc := NewNewcomerCampaignService(entClient, affiliateService)
	svc.persistentConfig = true
	svc.SetCacheInvalidators(billingCacheService, authCacheInvalidator)
	return svc
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
	client := s.db
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
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
	client := s.db
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
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
SELECT $1, u.id, u.created_at, $2, $3, $3 + INTERVAL '14 days'
FROM users u
WHERE u.id = $4 AND u.created_at >= $2 AND u.created_at < $3
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
	client := s.db
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
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

// OnPaymentCompleted is safe to call for every webhook delivery. The payment
// order itself is the source of truth; the ledger and unique constraints make
// reward fulfillment idempotent.
func (s *NewcomerCampaignService) OnPaymentCompleted(ctx context.Context, orderID int64) error {
	if !s.enabled() || orderID <= 0 {
		return nil
	}
	var userID int64
	var orderType string
	var completedAt *time.Time
	if err := s.queryOne(ctx, s.db, `
SELECT user_id, order_type, completed_at
FROM payment_orders
WHERE id = $1`, []any{orderID}, &userID, &orderType, &completedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load completed payment for campaign: %w", err)
	}
	if strings.TrimSpace(orderType) != "balance" || completedAt == nil {
		return nil
	}
	return s.ReconcileUser(ctx, userID)
}

// OnPaymentRefunded re-runs the same repair path after a final refund state.
// It intentionally does not assume that the original completion hook ran.
func (s *NewcomerCampaignService) OnPaymentRefunded(ctx context.Context, orderID int64) error {
	if !s.enabled() || orderID <= 0 {
		return nil
	}
	var userID int64
	var orderType string
	var orderStatus string
	if err := s.queryOne(ctx, s.db, `SELECT user_id, order_type, status FROM payment_orders WHERE id = $1`, []any{orderID}, &userID, &orderType, &orderStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load refunded payment for campaign: %w", err)
	}
	if strings.TrimSpace(orderType) != "balance" || (orderStatus != OrderStatusPartiallyRefunded && orderStatus != OrderStatusRefunded) {
		return nil
	}
	return s.ReconcileUser(ctx, userID)
}

// OnRedeemCompleted re-runs invitation qualification after a user redeems a
// positive balance code. Payment fulfilment also creates and redeems an
// internal code, but reconcileInvitee excludes codes linked to payment
// orders, so it cannot double-count the online recharge.
func (s *NewcomerCampaignService) OnRedeemCompleted(ctx context.Context, userID int64, code *RedeemCode) error {
	if !s.enabled() || userID <= 0 || code == nil || code.Type != RedeemTypeBalance || code.Value <= 0 || code.Status != StatusUsed {
		return nil
	}
	if code.UsedBy != nil && *code.UsedBy != userID {
		return nil
	}
	return s.ReconcileUser(ctx, userID)
}

// ReconcileUser is the repeatable repair entry used by payment hooks and the
// user-facing repair endpoint. It repairs first-recharge state, the invitee's
// effective qualification, and the inviter's membership state.
func (s *NewcomerCampaignService) ReconcileUser(ctx context.Context, userID int64) error {
	if !s.enabled() || userID <= 0 {
		return nil
	}
	if err := s.EnsureCampaignInviteCode(ctx, userID); err != nil {
		return err
	}
	if err := s.materializeReferralIntent(ctx, userID); err != nil {
		return err
	}
	if err := s.reconcileFirstRecharge(ctx, userID); err != nil {
		return err
	}
	inviterID, err := s.reconcileInvitee(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.reconcileMembership(ctx, userID); err != nil {
		return err
	}
	if inviterID > 0 && inviterID != userID {
		if err := s.reconcileMembership(ctx, inviterID); err != nil {
			return err
		}
	}
	return nil
}

func (s *NewcomerCampaignService) reconcileFirstRecharge(ctx context.Context, userID int64) error {
	start, end := NewcomerCampaignWindow()
	var registeredAt time.Time
	if s.persistentConfig {
		eligibility, err := s.loadCampaignEligibility(ctx, userID)
		if err != nil {
			return err
		}
		if eligibility == nil {
			// A reward ledger entry is still authoritative for refunds, even if
			// the eligibility snapshot was missing and needs operator repair.
			registeredAt = time.Time{}
		} else {
			registeredAt = eligibility.RegisteredAt
			start, end = eligibility.WindowStart, eligibility.WindowEnd
		}
	} else if err := s.queryOne(ctx, s.db, `SELECT created_at FROM users WHERE id = $1`, []any{userID}, &registeredAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load campaign user registration: %w", err)
	}
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
WHERE po.user_id = $1
  AND po.order_type = 'balance'
  AND po.completed_at IS NOT NULL
ORDER BY po.completed_at ASC, po.id ASC
LIMIT 1`, []any{userID}, &orderID, &principalAmount, &principalCurrency, &completedAt, &refundAmount, &orderStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load first campaign recharge: %w", err)
	}
	if isSuccessfulPaymentRefund(orderStatus) {
		return s.revokeFirstRechargeReward(ctx, userID, orderID, "source payment refunded")
	}
	if !principalAmount.Valid || !principalCurrency.Valid {
		s.logMissingPaymentFact(orderID, userID)
		return nil
	}
	if registeredAt.IsZero() || registeredAt.Before(start) || !registeredAt.Before(end) || completedAt.Before(start) || !completedAt.Before(end) || principalCurrency.String != newcomerPrincipalCurrency || principalAmount.Float64 < newcomerInviteThreshold {
		return nil
	}
	return s.grantFirstRechargeReward(ctx, userID, orderID, principalAmount.Float64)
}

func isSuccessfulPaymentRefund(status string) bool {
	switch strings.TrimSpace(status) {
	case OrderStatusPartiallyRefunded, OrderStatusRefunded:
		return true
	default:
		return false
	}
}

// newcomerCampaignNetPrincipal mirrors the SQL qualification expression. A
// payment order stores refund_amount in credited-balance units, while the
// campaign fact stores the original recharge principal. Therefore a partial
// refund is converted by the credited amount ratio and clamped to [0, 1].
func newcomerCampaignNetPrincipal(principalAmount, creditedAmount, refundAmount float64, status string) float64 {
	if principalAmount <= 0 || math.IsNaN(principalAmount) || math.IsInf(principalAmount, 0) {
		return 0
	}
	switch strings.TrimSpace(status) {
	case OrderStatusRefunded:
		return 0
	case OrderStatusPartiallyRefunded:
		if creditedAmount <= 0 || math.IsNaN(creditedAmount) || math.IsInf(creditedAmount, 0) {
			return 0
		}
		ratio := refundAmount / creditedAmount
		if math.IsNaN(ratio) || ratio <= 0 {
			return principalAmount
		}
		if math.IsInf(ratio, 0) || ratio >= 1 {
			return 0
		}
		return principalAmount * (1 - ratio)
	default:
		return principalAmount
	}
}

func (s *NewcomerCampaignService) logMissingPaymentFact(orderID, userID int64) {
	slog.Warn("newcomer campaign payment fact missing; order is not eligible until reconciliation", "order_id", orderID, "user_id", userID)
}

func (s *NewcomerCampaignService) grantFirstRechargeReward(ctx context.Context, userID, orderID int64, rechargeAmount float64) error {
	metadata, _ := json.Marshal(map[string]any{"recharge_amount": rechargeAmount, "reason": "first recharge"})
	balanceChanged := false
	err := s.withTx(ctx, func(txCtx context.Context, tx *dbent.Client) error {
		// A campaign reward is intentionally exempt from the generic balance
		// clawback trigger.  The reward must not repay the debt created when the
		// very same reward is later revoked after a refund.
		if _, err := tx.ExecContext(txCtx, `SELECT set_config('sub2api.balance_credit_kind', 'campaign_reward', true)`); err != nil {
			return err
		}
		// Serialize the grant with refund finalization. A completion callback can
		// race a refund callback; only the still-completed order may grant the
		// reward, and a refund that wins the lock will be observed here.
		var orderStatus string
		if err := s.queryOne(txCtx, tx, `
SELECT status
FROM payment_orders
WHERE id = $1 AND user_id = $2 AND order_type = 'balance'
FOR UPDATE`, []any{orderID, userID}, &orderStatus); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		if strings.TrimSpace(orderStatus) != OrderStatusCompleted {
			return nil
		}
		result, err := tx.ExecContext(txCtx, `
INSERT INTO newcomer_campaign_reward_ledger
    (campaign_key, user_id, source_order_id, reward_type, entry_type, amount, idempotency_key, metadata)
VALUES ($1, $2, $3, 'first_recharge', 'grant', $4, $5, $6)
ON CONFLICT (idempotency_key) DO NOTHING`,
			NewcomerCampaignKey, userID, orderID, newcomerRewardAmount,
			fmt.Sprintf("%s:first-recharge:grant:%d", NewcomerCampaignKey, orderID), string(metadata))
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil
		}
		_, err = tx.ExecContext(txCtx, `UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2`, newcomerRewardAmount, userID)
		if err == nil {
			balanceChanged = true
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("grant first recharge reward: %w", err)
	}
	if balanceChanged {
		s.invalidateBalanceCaches(ctx, userID)
	}
	return nil
}

func (s *NewcomerCampaignService) revokeFirstRechargeReward(ctx context.Context, userID, orderID int64, reason string) error {
	balanceChanged := false
	err := s.withTx(ctx, func(txCtx context.Context, tx *dbent.Client) error {
		var grantedAmount float64
		if err := s.queryOne(txCtx, tx, `
	SELECT COALESCE(SUM(amount), 0)
	FROM newcomer_campaign_reward_ledger
	WHERE campaign_key = $1 AND user_id = $2 AND source_order_id = $3
	  AND reward_type = 'first_recharge' AND entry_type = 'grant'`,
			[]any{NewcomerCampaignKey, userID, orderID}, &grantedAmount); err != nil {
			return fmt.Errorf("load first recharge reward for reversal: %w", err)
		}
		if grantedAmount <= 0 {
			return nil
		}

		// Insert the legacy reward reversal first.  Its idempotency key keeps
		// repeated refund callbacks from deducting the balance or creating debt
		// twice.  The new debt/allocation facts below are committed atomically
		// with this existing ledger entry.
		result, err := tx.ExecContext(txCtx, `
INSERT INTO newcomer_campaign_reward_ledger
    (campaign_key, user_id, source_order_id, reward_type, entry_type, amount, idempotency_key, metadata)
VALUES ($1, $2, $3, 'first_recharge', 'revoke', $4, $5, $6)
ON CONFLICT (idempotency_key) DO NOTHING`,
			NewcomerCampaignKey, userID, orderID, grantedAmount,
			fmt.Sprintf("%s:first-recharge:revoke:%d", NewcomerCampaignKey, orderID),
			fmt.Sprintf(`{"reason":%q}`, reason))
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil
		}

		// Persist the full amount owed before recovering anything.  The unique
		// key is tied to the refunded source order, making this safe against
		// duplicate and concurrent refund callbacks.
		var debtID int64
		if err := s.queryOne(txCtx, tx, `
INSERT INTO newcomer_campaign_clawback_debts
    (campaign_key, user_id, source_order_id, reward_type, due_amount, idempotency_key, metadata)
VALUES ($1, $2, $3, 'first_recharge', $4, $5, $6)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING id`,
			[]any{NewcomerCampaignKey, userID, orderID, grantedAmount,
				fmt.Sprintf("%s:first-recharge:clawback:%d", NewcomerCampaignKey, orderID),
				fmt.Sprintf(`{"reason":%q,"due_amount":%.8f}`, reason, grantedAmount)}, &debtID); err != nil {
			return fmt.Errorf("create first recharge clawback debt: %w", err)
		}

		var availableBalance float64
		if err := s.queryOne(txCtx, tx, `
SELECT balance
FROM users
WHERE id = $1
FOR UPDATE`, []any{userID}, &availableBalance); err != nil {
			return fmt.Errorf("lock user balance for first recharge clawback: %w", err)
		}
		recoverable := math.Min(math.Max(availableBalance, 0), grantedAmount)
		if recoverable <= 0 {
			return nil
		}
		if _, err := tx.ExecContext(txCtx, `
UPDATE users
SET balance = balance - $1, updated_at = NOW()
WHERE id = $2`, recoverable, userID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(txCtx, `
INSERT INTO newcomer_campaign_clawback_allocations
    (debt_id, user_id, amount, source_type, source_id, idempotency_key)
VALUES ($1, $2, $3, 'first_recharge_refund', $4, $5)
ON CONFLICT (idempotency_key) DO NOTHING`,
			debtID, userID, recoverable, fmt.Sprintf("%d", orderID),
			fmt.Sprintf("newcomer-clawback:%d:first_recharge_refund:%d", debtID, orderID)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(txCtx, `
UPDATE newcomer_campaign_clawback_debts
SET recovered_amount = recovered_amount + $1,
    status = CASE WHEN recovered_amount + $1 >= due_amount THEN 'settled' ELSE 'pending' END,
    settled_at = CASE WHEN recovered_amount + $1 >= due_amount THEN NOW() ELSE settled_at END,
    updated_at = NOW()
WHERE id = $2`, recoverable, debtID); err != nil {
			return err
		}
		balanceChanged = true
		return err
	})
	if err != nil {
		return fmt.Errorf("revoke first recharge reward: %w", err)
	}
	if balanceChanged {
		s.invalidateBalanceCaches(ctx, userID)
	}
	return nil
}

func (s *NewcomerCampaignService) invalidateBalanceCaches(ctx context.Context, userID int64) {
	if s == nil || userID <= 0 {
		return
	}
	if s.balanceCacheInvalidator != nil {
		if err := s.balanceCacheInvalidator.InvalidateUserBalance(ctx, userID); err != nil {
			slog.Warn("newcomer campaign balance cache invalidation failed", "user_id", userID, "error", err)
		}
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
}

func (s *NewcomerCampaignService) reconcileInvitee(ctx context.Context, userID int64) (int64, error) {
	now := s.currentTime()
	var inviterID int64
	err := s.queryOne(ctx, s.db, `
SELECT inviter_id FROM newcomer_campaign_invites
WHERE campaign_key = $1 AND invitee_id = $2`, []any{NewcomerCampaignKey, userID}, &inviterID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load campaign inviter: %w", err)
	}
	s.logMissingInvitePaymentFacts(ctx, userID)
	_, err = s.db.ExecContext(ctx, `
WITH consumption AS (
	SELECT i.id AS invite_id,
	       po.id AS source_order_id,
	       NULL::BIGINT AS source_redeem_code_id,
	       po.completed_at AS occurred_at,
	       CASE WHEN f.principal_currency = $5 THEN
				f.principal_amount * CASE
					WHEN po.status = 'REFUNDED' THEN 0
					WHEN po.status = 'PARTIALLY_REFUNDED' THEN
						-- refund_amount and amount are credited-balance units;
						-- convert their clamped ratio back to original principal.
						CASE
							WHEN po.amount > 0 THEN
								1 - LEAST(GREATEST(COALESCE(po.refund_amount, 0) / po.amount, 0), 1)
							ELSE 0
						END
					ELSE 1
				END
			ELSE 0 END AS amount
	FROM newcomer_campaign_invites i
	JOIN payment_orders po
	  ON po.user_id = i.invitee_id
	  AND po.order_type = 'balance'
	  AND po.completed_at IS NOT NULL
	  AND po.completed_at >= i.registered_at
	  AND po.completed_at < i.qualification_deadline
	JOIN newcomer_campaign_payment_facts f ON f.order_id = po.id
	WHERE i.campaign_key = $1 AND i.invitee_id = $2

	UNION ALL

	SELECT i.id AS invite_id,
	       NULL::BIGINT AS source_order_id,
	       rc.id AS source_redeem_code_id,
	       rc.used_at AS occurred_at,
	       rc.value AS amount
	FROM newcomer_campaign_invites i
	JOIN redeem_codes rc
	  ON rc.used_by = i.invitee_id
	  AND rc.type = 'balance'
	  AND rc.status = 'used'
	  AND rc.value > 0
	  -- An explicitly excluded/free code is a gift and never campaign spend.
	  -- Pending/not_applicable codes remain valid user redemptions; the
	  -- affiliate review switch must not disable campaign qualification.
	  AND COALESCE(rc.affiliate_rebate_status, 'not_applicable') <> 'excluded'
	  AND rc.used_at IS NOT NULL
	  AND rc.used_at >= i.registered_at
	  AND rc.used_at < i.qualification_deadline
	WHERE i.campaign_key = $1 AND i.invitee_id = $2
	  -- Payment fulfilment uses a redeem code internally; the payment fact
	  -- above is the single source for that online recharge.
	  AND NOT EXISTS (
		SELECT 1 FROM payment_orders internal_po
		WHERE internal_po.order_type = 'balance'
		  AND internal_po.recharge_code = rc.code
	  )
), ordered_consumption AS (
	SELECT c.*,
	       SUM(c.amount) OVER (
			PARTITION BY c.invite_id
			ORDER BY c.occurred_at ASC, c.source_order_id ASC NULLS LAST, c.source_redeem_code_id ASC NULLS LAST
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		) AS cumulative_amount
	FROM consumption c
), totals AS (
	SELECT i.id,
	       COALESCE(SUM(c.amount), 0) AS qualifying_amount,
       -- Keep the source that actually crossed the threshold, rather than
       -- whichever payment/code happened to be latest. The same ordering as
       -- the running total gives deterministic results for equal timestamps.
       (ARRAY_AGG(c.source_order_id ORDER BY c.occurred_at ASC, c.source_order_id ASC NULLS LAST, c.source_redeem_code_id ASC NULLS LAST)
		FILTER (WHERE c.cumulative_amount >= $3 AND c.source_order_id IS NOT NULL))[1] AS qualifying_order_id,
       (ARRAY_AGG(c.source_redeem_code_id ORDER BY c.occurred_at ASC, c.source_order_id ASC NULLS LAST, c.source_redeem_code_id ASC NULLS LAST)
		FILTER (WHERE c.cumulative_amount >= $3 AND c.source_redeem_code_id IS NOT NULL))[1] AS qualifying_redeem_code_id,
	       MIN(c.occurred_at) FILTER (WHERE c.cumulative_amount >= $3) AS qualified_at
	FROM newcomer_campaign_invites i
	LEFT JOIN ordered_consumption c ON c.invite_id = i.id
	WHERE i.campaign_key = $1 AND i.invitee_id = $2
	GROUP BY i.id
)
UPDATE newcomer_campaign_invites i
SET qualifying_amount = totals.qualifying_amount,
    qualifying_order_id = totals.qualifying_order_id,
    qualifying_redeem_code_id = totals.qualifying_redeem_code_id,
    status = CASE
        WHEN totals.qualifying_amount >= $3 THEN 'qualified'
        WHEN i.status = 'qualified' THEN 'revoked'
        WHEN i.qualification_deadline <= $4 THEN 'expired'
        ELSE 'pending'
    END,
    -- qualified_at is the real event instant that first crossed ¥10. It must
    -- be recomputed after a revoke/re-qualification and must never use the
    -- reconciliation clock.
    qualified_at = CASE
	    WHEN totals.qualifying_amount >= $3 THEN totals.qualified_at
	    ELSE NULL
	END,
    revoked_at = CASE
        WHEN totals.qualifying_amount < $3 AND i.status = 'qualified' THEN $4
        ELSE i.revoked_at
    END,
    updated_at = $4
FROM totals
WHERE i.id = totals.id`, NewcomerCampaignKey, userID, newcomerInviteThreshold, now, newcomerPrincipalCurrency)
	if err != nil {
		return 0, fmt.Errorf("reconcile campaign invitation: %w", err)
	}
	return inviterID, nil
}

// logMissingInvitePaymentFacts makes a missing immutable principal visible to
// operators. The qualification UPDATE below intentionally excludes such rows;
// it never guesses payment_orders.amount. Redeem-code consumption is already
// self-contained in redeem_codes and does not require a payment fact.
func (s *NewcomerCampaignService) logMissingInvitePaymentFacts(ctx context.Context, userID int64) {
	var missing int
	err := s.queryOne(ctx, s.db, `
SELECT COUNT(*)
FROM newcomer_campaign_invites i
JOIN payment_orders po
  ON po.user_id = i.invitee_id
  AND po.order_type = 'balance'
  AND po.completed_at IS NOT NULL
  AND po.completed_at >= i.registered_at
  AND po.completed_at < i.qualification_deadline
LEFT JOIN newcomer_campaign_payment_facts f ON f.order_id = po.id
WHERE i.campaign_key = $1 AND i.invitee_id = $2 AND f.order_id IS NULL`,
		[]any{NewcomerCampaignKey, userID}, &missing)
	if err != nil {
		slog.Warn("newcomer campaign invite payment fact check failed", "user_id", userID, "error", err)
		return
	}
	if missing > 0 {
		slog.Warn("newcomer campaign invite payment fact missing; orders excluded until reconciliation", "user_id", userID, "missing_orders", missing)
	}
}

func (s *NewcomerCampaignService) reconcileMembership(ctx context.Context, inviterID int64) error {
	if inviterID <= 0 {
		return nil
	}
	defer s.invalidateMembershipFactor(inviterID)
	now := s.currentTime()
	err := s.withTx(ctx, func(txCtx context.Context, tx *dbent.Client) error {
		tiers := s.configuredCampaignTiers(txCtx)
		var grantFloor time.Time
		if s.persistentConfig {
			var configStart, configUpdatedAt time.Time
			if err := s.queryOne(txCtx, tx, `
SELECT starts_at, updated_at
FROM newcomer_campaign_config
WHERE campaign_key = $1`, []any{NewcomerCampaignKey}, &configStart, &configUpdatedAt); err != nil {
				return err
			}
			grantFloor = configStart
			if configUpdatedAt.After(grantFloor) {
				grantFloor = configUpdatedAt
			}
		}
		// Serialize only campaign membership changes for this inviter. This does
		// not touch gateway concurrency or any user concurrency field.
		if _, err := tx.ExecContext(txCtx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("%s:membership:%d", NewcomerCampaignKey, inviterID)); err != nil {
			return err
		}
		var validCount int
		if err := s.queryOne(txCtx, tx, `
SELECT COUNT(*)
FROM newcomer_campaign_invites
WHERE campaign_key = $1 AND inviter_id = $2 AND status = 'qualified'
  AND qualifying_amount >= $3`, []any{NewcomerCampaignKey, inviterID, newcomerInviteThreshold}, &validCount); err != nil {
			return err
		}
		if _, err := tx.ExecContext(txCtx, `
UPDATE newcomer_campaign_membership_grants
SET status = 'expired', updated_at = $3
WHERE campaign_key = $1 AND user_id = $2 AND status = 'active' AND expires_at <= $3`, NewcomerCampaignKey, inviterID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(txCtx, `
UPDATE newcomer_campaign_membership_grants
SET status = 'revoked', revoked_at = $3, revoke_reason = 'valid invitation count decreased', updated_at = $3
WHERE campaign_key = $1 AND user_id = $2 AND status = 'active' AND threshold > $4`, NewcomerCampaignKey, inviterID, now, validCount); err != nil {
			return err
		}
		for _, tier := range tiers {
			if validCount < tier.Threshold {
				continue
			}
			var startsAt time.Time
			err := s.queryOne(txCtx, tx, `
SELECT qualified_at
FROM newcomer_campaign_invites
WHERE campaign_key = $1 AND inviter_id = $2 AND status = 'qualified'
  AND qualifying_amount >= $3 AND qualified_at IS NOT NULL
ORDER BY qualified_at ASC, id ASC
OFFSET $4 LIMIT 1`, []any{NewcomerCampaignKey, inviterID, newcomerInviteThreshold, tier.Threshold - 1}, &startsAt)
			if errors.Is(err, sql.ErrNoRows) {
				// A legacy qualified row without an event timestamp cannot be
				// used to invent a membership start time.
				continue
			}
			if err != nil {
				return err
			}
			expiresAt := startsAt.Add(time.Duration(tier.DurationDays) * 24 * time.Hour)
			if grantFloor.After(startsAt) {
				startsAt = grantFloor
				expiresAt = startsAt.Add(time.Duration(tier.DurationDays) * 24 * time.Hour)
			}
			if !expiresAt.After(now) {
				// Reconciliation must not resurrect a tier whose real event-based
				// expiry has already passed.
				continue
			}
			if _, err := tx.ExecContext(txCtx, `
INSERT INTO newcomer_campaign_membership_grants
    (campaign_key, user_id, tier_key, threshold, factor, duration_days, granted_at, starts_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (campaign_key, user_id, tier_key) DO NOTHING`,
				NewcomerCampaignKey, inviterID, tier.Key, tier.Threshold, tier.Factor, tier.DurationDays, now, startsAt, expiresAt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("reconcile campaign membership: %w", err)
	}
	return nil
}

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
	tiers := s.configuredCampaignTiers(ctx)
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
	for i := range status.Tiers {
		if status.ValidInviteCount < status.Tiers[i].Threshold {
			status.NextTier = &status.Tiers[i]
			status.NextTierProgress = status.ValidInviteCount
			status.NextTierRemaining = status.Tiers[i].Threshold - status.ValidInviteCount
			break
		}
	}
	membership, err := s.loadEffectiveMembership(ctx, userID, now, status.ValidInviteCount)
	if err != nil {
		return nil, err
	}
	status.CurrentMembership = membership
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

func (s *NewcomerCampaignService) withTx(ctx context.Context, fn func(context.Context, *dbent.Client) error) error {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return err
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
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
	defer rows.Close()
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

// BackfillPaymentFactsForUser is an explicit, repeatable repair operation. It
// only trusts the immutable ORDER_CREATED audit payload for the original
// principal; payment_orders.amount is the credited balance and is never used
// as a substitute.
func (s *NewcomerCampaignService) BackfillPaymentFactsForUser(ctx context.Context, userID int64) (int, error) {
	if !s.enabled() || userID <= 0 {
		return 0, nil
	}
	return s.backfillPaymentFacts(ctx, &userID)
}

// BackfillPaymentFacts is the global operator repair entry. It is safe to run
// repeatedly because facts are keyed by payment order ID.
func (s *NewcomerCampaignService) BackfillPaymentFacts(ctx context.Context) (int, error) {
	if !s.enabled() {
		return 0, nil
	}
	return s.backfillPaymentFacts(ctx, nil)
}

func (s *NewcomerCampaignService) backfillPaymentFacts(ctx context.Context, userID *int64) (int, error) {
	start, end := NewcomerCampaignWindow()
	captureEnd := end.Add(newcomerCampaignCaptureGrace)
	backfilled := 0
	var lastOrderID int64
	for {
		// Keyset pagination bounds both the amount held in memory and the
		// transaction time for a large repair. Campaign users are identified by
		// their registration window; orders after the global +14-day capture
		// cutoff are deliberately not reconstructed.
		query := `
SELECT DISTINCT ON (po.id) po.id, po.user_id, pal.detail
FROM payment_orders po
JOIN users u
  ON u.id = po.user_id
 AND u.created_at >= $1 AND u.created_at < $2
JOIN payment_audit_logs pal
  ON pal.order_id = po.id::text AND pal.action = 'ORDER_CREATED'
LEFT JOIN newcomer_campaign_payment_facts f ON f.order_id = po.id
WHERE po.order_type = 'balance'
  AND po.created_at >= $1 AND po.created_at < $3
  AND po.id > $4
  AND f.order_id IS NULL`
		args := []any{start, end, captureEnd, lastOrderID}
		if s.persistentConfig {
			query = `
SELECT DISTINCT ON (po.id) po.id, po.user_id, pal.detail
FROM payment_orders po
JOIN newcomer_campaign_eligible_users e
  ON e.campaign_key = $1 AND e.user_id = po.user_id
JOIN payment_audit_logs pal
  ON pal.order_id = po.id::text AND pal.action = 'ORDER_CREATED'
LEFT JOIN newcomer_campaign_payment_facts f ON f.order_id = po.id
WHERE po.order_type = 'balance'
  AND po.created_at >= e.registered_at
  AND po.created_at < e.capture_deadline
  AND po.id > $2
  AND f.order_id IS NULL`
			args = []any{NewcomerCampaignKey, lastOrderID}
		}
		if userID != nil {
			query += " AND po.user_id = $" + strconv.Itoa(len(args)+1)
			args = append(args, *userID)
		}
		query += " ORDER BY po.id, pal.created_at DESC, pal.id DESC LIMIT $" + strconv.Itoa(len(args)+1)
		args = append(args, newcomerCampaignRepairBatchSize)

		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return backfilled, fmt.Errorf("load newcomer payment fact backfill candidates: %w", err)
		}
		type candidate struct {
			orderID     int64
			auditUserID int64
			detail      string
		}
		candidates := make([]candidate, 0, newcomerCampaignRepairBatchSize)
		for rows.Next() {
			var item candidate
			if err := rows.Scan(&item.orderID, &item.auditUserID, &item.detail); err != nil {
				_ = rows.Close()
				return backfilled, fmt.Errorf("scan newcomer payment fact backfill candidate: %w", err)
			}
			candidates = append(candidates, item)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return backfilled, fmt.Errorf("iterate newcomer payment fact backfill candidates: %w", err)
		}
		if err := rows.Close(); err != nil {
			return backfilled, fmt.Errorf("close newcomer payment fact backfill candidates: %w", err)
		}
		if len(candidates) == 0 {
			break
		}
		for _, item := range candidates {
			orderID, auditUserID, detail := item.orderID, item.auditUserID, item.detail
			var audit struct {
				PaymentAmount     *float64 `json:"paymentAmount"`
				PrincipalCurrency string   `json:"principalCurrency"`
			}
			if err := json.Unmarshal([]byte(detail), &audit); err != nil {
				slog.Warn("newcomer campaign payment fact backfill skipped invalid order audit", "order_id", orderID, "user_id", auditUserID, "error", err)
				continue
			}
			if audit.PaymentAmount == nil || math.IsNaN(*audit.PaymentAmount) || math.IsInf(*audit.PaymentAmount, 0) || *audit.PaymentAmount <= 0 {
				slog.Warn("newcomer campaign payment fact backfill skipped missing payment principal", "order_id", orderID, "user_id", auditUserID)
				continue
			}
			// paymentAmount is the platform's selected recharge principal. It is
			// always expressed in the campaign/base currency; provider currency is
			// deliberately not copied into this fact.
			currency := newcomerPrincipalCurrency
			if audit.PrincipalCurrency != "" && strings.ToUpper(strings.TrimSpace(audit.PrincipalCurrency)) != currency {
				slog.Warn("newcomer campaign payment fact backfill ignored non-base audit currency", "order_id", orderID, "user_id", auditUserID, "currency", audit.PrincipalCurrency, "base_currency", currency)
			}
			result, err := s.db.ExecContext(ctx, `
INSERT INTO newcomer_campaign_payment_facts
    (order_id, user_id, principal_amount, principal_currency)
VALUES ($1, $2, $3, $4)
ON CONFLICT (order_id) DO NOTHING`, orderID, auditUserID, *audit.PaymentAmount, currency)
			if err != nil {
				return backfilled, fmt.Errorf("backfill newcomer payment fact for order %d: %w", orderID, err)
			}
			if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected > 0 {
				backfilled++
			}
		}
		lastOrderID = candidates[len(candidates)-1].orderID
		if len(candidates) < newcomerCampaignRepairBatchSize {
			break
		}
	}
	return backfilled, nil
}

func isPaymentPrincipalCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for i := 0; i < len(currency); i++ {
		if currency[i] < 'A' || currency[i] > 'Z' {
			return false
		}
	}
	return true
}

// ReconcileAll is intentionally explicit so operators can run a repeatable
// repair job without changing normal startup or touching production here.
func (s *NewcomerCampaignService) ReconcileAll(ctx context.Context) (int, error) {
	if !s.enabled() {
		return 0, nil
	}
	if _, err := s.BackfillPaymentFacts(ctx); err != nil {
		return 0, err
	}
	start, end := s.campaignWindow(ctx)
	var lastUserID int64
	var repaired int
	for {
		// Reconcile only activity participants and their inviters. Keyset
		// pagination avoids scanning every account and allows each batch's rows
		// to be closed before the per-user repair queries run.
		candidateQuery := `
WITH candidate_ids AS (
    SELECT u.id
    FROM users u
    WHERE u.created_at >= $2 AND u.created_at < $3
      AND u.role <> 'admin' AND u.deleted_at IS NULL
    UNION
    SELECT inviter_id AS id
    FROM newcomer_campaign_invites
    WHERE campaign_key = $1 AND registered_at >= $2 AND registered_at < $3
    UNION
    SELECT invitee_id AS id
    FROM newcomer_campaign_invites
    WHERE campaign_key = $1 AND registered_at >= $2 AND registered_at < $3
    UNION
    SELECT r.invitee_id AS id
    FROM newcomer_campaign_referral_intents r
    JOIN users ru ON ru.id = r.invitee_id
    WHERE r.campaign_key = $1 AND ru.created_at >= $2 AND ru.created_at < $3
    UNION
    SELECT pf.user_id AS id
    FROM newcomer_campaign_payment_facts pf
    JOIN payment_orders pfo ON pfo.id = pf.order_id
    JOIN users pfu ON pfu.id = pf.user_id
    WHERE pfu.created_at >= $2 AND pfu.created_at < $3
      AND pfo.created_at < $3 + INTERVAL '14 days'
)
SELECT u.id
FROM users u
JOIN candidate_ids c ON c.id = u.id
WHERE u.role <> 'admin' AND u.deleted_at IS NULL AND u.id > $4
ORDER BY u.id
	LIMIT $5`
		args := []any{NewcomerCampaignKey, start, end, lastUserID, newcomerCampaignRepairBatchSize}
		if s.persistentConfig {
			candidateQuery = `
WITH candidate_ids AS (
    SELECT eu.user_id AS id
    FROM newcomer_campaign_eligible_users eu
    WHERE eu.campaign_key = $1
    UNION
    SELECT inviter_id AS id
    FROM newcomer_campaign_invites
    WHERE campaign_key = $1
    UNION
    SELECT invitee_id AS id
    FROM newcomer_campaign_invites
    WHERE campaign_key = $1
    UNION
    SELECT r.invitee_id AS id
    FROM newcomer_campaign_referral_intents r
    WHERE r.campaign_key = $1
    UNION
    SELECT pf.user_id AS id
    FROM newcomer_campaign_payment_facts pf
    WHERE pf.user_id > 0
)
SELECT u.id
FROM users u
JOIN candidate_ids c ON c.id = u.id
WHERE u.role <> 'admin' AND u.deleted_at IS NULL AND u.id > $2
ORDER BY u.id
LIMIT $3`
			args = []any{NewcomerCampaignKey, lastUserID, newcomerCampaignRepairBatchSize}
		}
		rows, err := s.db.QueryContext(ctx, candidateQuery, args...)
		if err != nil {
			return repaired, err
		}
		userIDs := make([]int64, 0, newcomerCampaignRepairBatchSize)
		for rows.Next() {
			var userID int64
			if err := rows.Scan(&userID); err != nil {
				_ = rows.Close()
				return repaired, err
			}
			userIDs = append(userIDs, userID)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return repaired, err
		}
		if err := rows.Close(); err != nil {
			return repaired, err
		}
		if len(userIDs) == 0 {
			break
		}
		for _, userID := range userIDs {
			if err := s.ReconcileUser(ctx, userID); err != nil {
				slog.Warn("newcomer campaign reconciliation failed", "user_id", userID, "error", err)
				continue
			}
			repaired++
		}
		lastUserID = userIDs[len(userIDs)-1]
		if len(userIDs) < newcomerCampaignRepairBatchSize {
			break
		}
	}
	return repaired, nil
}

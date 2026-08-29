package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

const (
	NewcomerCampaignKey       = "newcomer_202609"
	NewcomerCampaignName      = "2026 年 9 月迎新活动"
	newcomerRewardAmount      = 2.0
	newcomerInviteThreshold   = 10.0
	newcomerPrincipalCurrency = "CNY"
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
	affiliateEnsurer        newcomerAffiliateEnsurer
	balanceCacheInvalidator newcomerBalanceCacheInvalidator
	authCacheInvalidator    APIKeyAuthCacheInvalidator
}

func NewNewcomerCampaignService(entClient *dbent.Client, affiliateEnsurers ...newcomerAffiliateEnsurer) *NewcomerCampaignService {
	var affiliateEnsurer newcomerAffiliateEnsurer
	if len(affiliateEnsurers) > 0 {
		affiliateEnsurer = affiliateEnsurers[0]
	}
	return &NewcomerCampaignService{
		db:               entClient,
		now:              func() time.Time { return time.Now().UTC() },
		affiliateEnsurer: affiliateEnsurer,
	}
}

func ProvideNewcomerCampaignService(entClient *dbent.Client, affiliateService *AffiliateService, billingCacheService *BillingCacheService, authCacheInvalidator APIKeyAuthCacheInvalidator) *NewcomerCampaignService {
	svc := NewNewcomerCampaignService(entClient, affiliateService)
	svc.SetCacheInvalidators(billingCacheService, authCacheInvalidator)
	return svc
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
}

func (s *NewcomerCampaignService) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func (s *NewcomerCampaignService) enabled() bool {
	return s != nil && s.db != nil
}

// OnUserRegistered independently binds the activity invitation. It never
// consults the ordinary affiliate-rebate switch and therefore remains valid
// when cash rebates are disabled.
func (s *NewcomerCampaignService) OnUserRegistered(ctx context.Context, userID int64, affiliateCode string) error {
	affiliateCode = strings.ToUpper(strings.TrimSpace(affiliateCode))
	if !s.enabled() || userID <= 0 || affiliateCode == "" {
		return nil
	}
	start, end := NewcomerCampaignWindow()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO newcomer_campaign_invites
    (campaign_key, inviter_id, invitee_id, invite_code, registered_at, qualification_deadline)
SELECT $1, ua.user_id, u.id, ua.aff_code, u.created_at, u.created_at + INTERVAL '14 days'
FROM user_affiliates ua
JOIN users u ON u.id = $2
WHERE ua.aff_code = $3
  AND ua.user_id <> u.id
  AND u.created_at >= $4
  AND u.created_at < $5
ON CONFLICT (campaign_key, invitee_id) DO NOTHING`,
		NewcomerCampaignKey, userID, affiliateCode, start, end)
	if err != nil {
		return fmt.Errorf("bind newcomer campaign invitation: %w", err)
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
	if err := s.queryOne(ctx, s.db, `SELECT created_at FROM users WHERE id = $1`, []any{userID}, &registeredAt); err != nil {
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
	if !principalAmount.Valid || !principalCurrency.Valid {
		s.logMissingPaymentFact(orderID, userID)
		return nil
	}
	if registeredAt.Before(start) || !registeredAt.Before(end) || completedAt.Before(start) || !completedAt.Before(end) || principalCurrency.String != newcomerPrincipalCurrency || principalAmount.Float64 < newcomerInviteThreshold {
		return nil
	}
	if isSuccessfulPaymentRefund(orderStatus) {
		return s.revokeFirstRechargeReward(ctx, userID, orderID, "source payment refunded")
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
	var grantedAmount float64
	err := s.queryOne(ctx, s.db, `
SELECT COALESCE(SUM(amount), 0)
FROM newcomer_campaign_reward_ledger
WHERE campaign_key = $1 AND user_id = $2 AND source_order_id = $3
  AND reward_type = 'first_recharge' AND entry_type = 'grant'`,
		[]any{NewcomerCampaignKey, userID, orderID}, &grantedAmount)
	if err != nil {
		return fmt.Errorf("load first recharge reward for reversal: %w", err)
	}
	if grantedAmount <= 0 {
		return nil
	}
	balanceChanged := false
	err = s.withTx(ctx, func(txCtx context.Context, tx *dbent.Client) error {
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
		_, err = tx.ExecContext(txCtx, `
UPDATE users
SET balance = GREATEST(balance - $1, 0), updated_at = NOW()
WHERE id = $2`, grantedAmount, userID)
		if err == nil {
			balanceChanged = true
		}
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
), totals AS (
	SELECT i.id,
	       COALESCE(SUM(c.amount), 0) AS qualifying_amount,
	       (ARRAY_AGG(c.source_order_id ORDER BY c.occurred_at DESC, c.source_order_id DESC)
		FILTER (WHERE c.source_order_id IS NOT NULL))[1] AS qualifying_order_id,
	       (ARRAY_AGG(c.source_redeem_code_id ORDER BY c.occurred_at DESC, c.source_redeem_code_id DESC)
		FILTER (WHERE c.source_redeem_code_id IS NOT NULL))[1] AS qualifying_redeem_code_id
	FROM newcomer_campaign_invites i
	LEFT JOIN consumption c ON c.invite_id = i.id
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
    qualified_at = CASE
        WHEN totals.qualifying_amount >= $3 THEN COALESCE(i.qualified_at, $4)
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
	now := s.currentTime()
	err := s.withTx(ctx, func(txCtx context.Context, tx *dbent.Client) error {
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
		for _, tier := range newcomerCampaignTiers {
			if validCount < tier.Threshold {
				continue
			}
			startsAt := now
			expiresAt := now.Add(time.Duration(tier.DurationDays) * 24 * time.Hour)
			if _, err := tx.ExecContext(txCtx, `
INSERT INTO newcomer_campaign_membership_grants
    (campaign_key, user_id, tier_key, threshold, factor, duration_days, granted_at, starts_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8)
ON CONFLICT (campaign_key, user_id, tier_key) DO NOTHING`,
				NewcomerCampaignKey, inviterID, tier.Key, tier.Threshold, tier.Factor, tier.DurationDays, startsAt, expiresAt); err != nil {
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
	var factor float64
	err := s.queryOne(ctx, s.db, `
SELECT g.factor
FROM newcomer_campaign_membership_grants g
WHERE g.campaign_key = $1 AND g.user_id = $2 AND g.status = 'active'
  AND g.starts_at <= $3 AND g.expires_at > $3
  AND g.threshold <= (
      SELECT COUNT(*) FROM newcomer_campaign_invites i
      WHERE i.campaign_key = g.campaign_key AND i.inviter_id = g.user_id
        AND i.status = 'qualified' AND i.qualifying_amount >= $4
  )
ORDER BY g.threshold DESC
LIMIT 1`, []any{NewcomerCampaignKey, userID, now, newcomerInviteThreshold}, &factor)
	if err != nil || factor <= 0 || factor > 1 {
		return 1
	}
	return factor
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
	if err := s.ReconcileUser(ctx, userID); err != nil {
		return nil, err
	}
	start, end := NewcomerCampaignWindow()
	now := s.currentTime()
	phase := "upcoming"
	if !now.Before(start) && now.Before(end) {
		phase = "active"
	} else if !now.Before(end) {
		phase = "ended"
	}
	status := &NewcomerCampaignStatus{
		CampaignKey: NewcomerCampaignKey,
		Name:        NewcomerCampaignName,
		Phase:       phase,
		StartsAt:    start,
		EndsAt:      end,
		Tiers:       NewcomerCampaignTiers(),
	}
	code, err := s.ensureCampaignInviteCode(ctx, userID)
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
	var membership NewcomerMembershipStatus
	err = s.queryOne(ctx, s.db, `
SELECT g.tier_key, g.factor, g.starts_at, g.expires_at
FROM newcomer_campaign_membership_grants g
WHERE g.campaign_key = $1 AND g.user_id = $2 AND g.status = 'active'
  AND g.starts_at <= $3 AND g.expires_at > $3 AND g.threshold <= $4
ORDER BY g.threshold DESC
LIMIT 1`, []any{NewcomerCampaignKey, userID, now, status.ValidInviteCount}, &membership.TierKey, &membership.Factor, &membership.StartsAt, &membership.ExpiresAt)
	if err == nil {
		membership.TierName = tierName(membership.TierKey)
		status.CurrentMembership = &membership
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load current campaign membership: %w", err)
	}
	return status, nil
}

func (s *NewcomerCampaignService) ensureCampaignInviteCode(ctx context.Context, userID int64) (string, error) {
	if s.affiliateEnsurer != nil {
		affiliateSummary, err := s.affiliateEnsurer.EnsureUserAffiliate(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("ensure campaign invite profile: %w", err)
		}
		if affiliateSummary == nil {
			return "", nil
		}
		return strings.TrimSpace(affiliateSummary.AffCode), nil
	}

	var code string
	if err := s.queryOne(ctx, s.db, `SELECT COALESCE(aff_code, '') FROM user_affiliates WHERE user_id = $1`, []any{userID}, &code); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("load campaign invite code: %w", err)
	}
	return strings.TrimSpace(code), nil
}

func (s *NewcomerCampaignService) firstRechargeStatus(ctx context.Context, userID int64, start, end time.Time) (NewcomerFirstRechargeStatus, error) {
	status := NewcomerFirstRechargeStatus{RewardStatus: "pending", RewardAmount: newcomerRewardAmount}
	var registeredAt time.Time
	if err := s.queryOne(ctx, s.db, `SELECT created_at FROM users WHERE id = $1`, []any{userID}, &registeredAt); err != nil {
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
	query := `
SELECT DISTINCT ON (po.id) po.id, po.user_id, pal.detail
FROM payment_orders po
JOIN payment_audit_logs pal
  ON pal.order_id = po.id::text AND pal.action = 'ORDER_CREATED'
LEFT JOIN newcomer_campaign_payment_facts f ON f.order_id = po.id
WHERE po.order_type = 'balance' AND f.order_id IS NULL`
	args := []any{}
	if userID != nil {
		query += " AND po.user_id = $1"
		args = append(args, *userID)
	}
	query += " ORDER BY po.id, pal.created_at DESC, pal.id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("load newcomer payment fact backfill candidates: %w", err)
	}
	defer rows.Close()
	backfilled := 0
	for rows.Next() {
		var orderID, auditUserID int64
		var detail string
		if err := rows.Scan(&orderID, &auditUserID, &detail); err != nil {
			return backfilled, fmt.Errorf("scan newcomer payment fact backfill candidate: %w", err)
		}
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
	if err := rows.Err(); err != nil {
		return backfilled, fmt.Errorf("iterate newcomer payment fact backfill candidates: %w", err)
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
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM users WHERE role <> 'admin'`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var repaired int
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return repaired, err
		}
		if err := s.ReconcileUser(ctx, userID); err != nil {
			slog.Warn("newcomer campaign reconciliation failed", "user_id", userID, "error", err)
			continue
		}
		repaired++
	}
	return repaired, rows.Err()
}

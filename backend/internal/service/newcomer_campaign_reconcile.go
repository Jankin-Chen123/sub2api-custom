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
)

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
	err := s.withTx(ctx, func(txCtx context.Context, tx NewcomerCampaignSQLClient) error {
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
	err := s.withTx(ctx, func(txCtx context.Context, tx NewcomerCampaignSQLClient) error {
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
	err := s.withTx(ctx, func(txCtx context.Context, tx NewcomerCampaignSQLClient) error {
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

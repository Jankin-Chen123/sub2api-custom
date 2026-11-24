package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
)

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
    WHERE u.created_at >= $2::timestamptz AND u.created_at < $3::timestamptz
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
    WHERE pfu.created_at >= $2::timestamptz AND pfu.created_at < $3::timestamptz
      AND pfo.created_at < $3::timestamptz + INTERVAL '14 days'
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

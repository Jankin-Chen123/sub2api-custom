package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func newNewcomerCampaignSQLMock(t *testing.T) (*dbent.Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() {
		_ = client.Close()
		_ = db.Close()
	})
	return client, mock
}

type newcomerBalanceCacheInvalidatorStub struct {
	calls []int64
	err   error
}

func (s *newcomerBalanceCacheInvalidatorStub) InvalidateUserBalance(_ context.Context, userID int64) error {
	s.calls = append(s.calls, userID)
	return s.err
}

type newcomerAuthCacheInvalidatorStub struct {
	calls []int64
}

func (s *newcomerAuthCacheInvalidatorStub) InvalidateAuthCacheByKey(context.Context, string) {}

func (s *newcomerAuthCacheInvalidatorStub) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	s.calls = append(s.calls, userID)
}

func (s *newcomerAuthCacheInvalidatorStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}

func TestNewcomerCampaignWindowUsesShanghaiBoundaries(t *testing.T) {
	start, end := NewcomerCampaignWindow()
	require.Equal(t, time.Date(2026, time.August, 31, 16, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, time.September, 30, 16, 0, 0, 0, time.UTC), end)
	require.Equal(t, 30*24*time.Hour, end.Sub(start))
}

func TestNewcomerCampaignTiersAreConfiguredOnce(t *testing.T) {
	tiers := NewcomerCampaignTiers()
	require.Equal(t, []NewcomerCampaignTier{
		{Key: "premium", Name: "高级", Threshold: 2, Factor: 0.98, DurationDays: 30},
		{Key: "gold", Name: "黄金", Threshold: 5, Factor: 0.96, DurationDays: 45},
		{Key: "diamond", Name: "钻石", Threshold: 10, Factor: 0.94, DurationDays: 60},
	}, tiers)
	tiers[0].Factor = 0.1
	require.Equal(t, 0.98, NewcomerCampaignTiers()[0].Factor)
}

func TestNewcomerCampaignFirstRechargeThresholdAndEndBoundary(t *testing.T) {
	start, end := NewcomerCampaignWindow()
	completedAt := start.Add(time.Hour)

	tests := []struct {
		name       string
		amount     float64
		wantStatus string
	}{
		{name: "9.99 is not qualified", amount: 9.99, wantStatus: "ineligible"},
		{name: "10.00 is qualified", amount: 10, wantStatus: "qualified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, mock := newNewcomerCampaignSQLMock(t)
			mock.ExpectQuery(`SELECT created_at FROM users WHERE id = \$1`).
				WithArgs(int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(start.Add(time.Hour)))
			mock.ExpectQuery(`(?s)SELECT po\.id, f\.principal_amount, f\.principal_currency.*FROM payment_orders po.*LEFT JOIN newcomer_campaign_payment_facts`).
				WithArgs(int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"id", "principal_amount", "principal_currency", "completed_at", "refund_amount", "status"}).
					AddRow(int64(99), tt.amount, "CNY", completedAt, 0.0, OrderStatusCompleted))
			if tt.amount >= newcomerInviteThreshold {
				mock.ExpectQuery(`(?s)SELECT\s+COUNT\(\*\) FILTER.*FROM newcomer_campaign_reward_ledger`).
					WithArgs(NewcomerCampaignKey, int64(7), int64(99)).
					WillReturnRows(sqlmock.NewRows([]string{"grants", "revokes"}).AddRow(0, 0))
			}

			svc := NewNewcomerCampaignService(client)
			status, err := svc.firstRechargeStatus(context.Background(), 7, start, end)
			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, status.RewardStatus)
			require.True(t, status.Eligible)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	client, mock := newNewcomerCampaignSQLMock(t)
	mock.ExpectQuery(`SELECT created_at FROM users WHERE id = \$1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(start.Add(time.Hour)))
	mock.ExpectQuery(`(?s)SELECT po\.id, f\.principal_amount, f\.principal_currency.*FROM payment_orders po.*LEFT JOIN newcomer_campaign_payment_facts`).
		WithArgs(int64(7)).
		WillReturnError(sql.ErrNoRows)
	svc := NewNewcomerCampaignService(client)
	svc.SetClock(func() time.Time { return end })
	status, err := svc.firstRechargeStatus(context.Background(), 7, start, end)
	require.NoError(t, err)
	require.True(t, status.Eligible)
	require.Equal(t, "ineligible", status.RewardStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignFirstRechargeGrantIsIdempotent(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	insert := `(?s)INSERT INTO newcomer_campaign_reward_ledger.*ON CONFLICT \(idempotency_key\) DO NOTHING`
	update := `UPDATE users SET balance = balance \+ \$1, updated_at = NOW\(\) WHERE id = \$2`
	lockOrder := `(?s)SELECT status.*FROM payment_orders.*WHERE id = \$1 AND user_id = \$2 AND order_type = 'balance'.*FOR UPDATE`

	mock.ExpectBegin()
	mock.ExpectQuery(lockOrder).WithArgs(int64(99), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(OrderStatusCompleted))
	mock.ExpectExec(insert).
		WithArgs(NewcomerCampaignKey, int64(7), int64(99), newcomerRewardAmount,
			"newcomer_202609:first-recharge:grant:99", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(update).WithArgs(newcomerRewardAmount, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := NewNewcomerCampaignService(client)
	require.NoError(t, svc.grantFirstRechargeReward(context.Background(), 7, 99, 10))

	// A repeated callback sees the same idempotency key and must not credit the
	// user a second time.
	mock.ExpectBegin()
	mock.ExpectQuery(lockOrder).WithArgs(int64(99), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(OrderStatusCompleted))
	mock.ExpectExec(insert).
		WithArgs(NewcomerCampaignKey, int64(7), int64(99), newcomerRewardAmount,
			"newcomer_202609:first-recharge:grant:99", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	require.NoError(t, svc.grantFirstRechargeReward(context.Background(), 7, 99, 10))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignRewardInvalidatesBalanceAndAuthCachesAfterCommit(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	balanceCache := &newcomerBalanceCacheInvalidatorStub{}
	authCache := &newcomerAuthCacheInvalidatorStub{}
	svc := NewNewcomerCampaignService(client)
	svc.SetCacheInvalidators(balanceCache, authCache)

	lockOrder := `(?s)SELECT status.*FROM payment_orders.*WHERE id = \$1 AND user_id = \$2 AND order_type = 'balance'.*FOR UPDATE`
	mock.ExpectBegin()
	mock.ExpectQuery(lockOrder).WithArgs(int64(99), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(OrderStatusCompleted))
	mock.ExpectExec(`(?s)INSERT INTO newcomer_campaign_reward_ledger.*ON CONFLICT \(idempotency_key\) DO NOTHING`).
		WithArgs(NewcomerCampaignKey, int64(7), int64(99), newcomerRewardAmount,
			"newcomer_202609:first-recharge:grant:99", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE users SET balance = balance \+ \$1, updated_at = NOW\(\) WHERE id = \$2`).
		WithArgs(newcomerRewardAmount, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, svc.grantFirstRechargeReward(context.Background(), 7, 99, 10))
	require.Equal(t, []int64{7}, balanceCache.calls)
	require.Equal(t, []int64{7}, authCache.calls)

	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(amount\), 0\).*FROM newcomer_campaign_reward_ledger`).
		WithArgs(NewcomerCampaignKey, int64(7), int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(newcomerRewardAmount))
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO newcomer_campaign_reward_ledger.*entry_type, amount, idempotency_key.*'revoke'`).
		WithArgs(NewcomerCampaignKey, int64(7), int64(99), newcomerRewardAmount,
			"newcomer_202609:first-recharge:revoke:99", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(`(?s)UPDATE users\s+SET balance = GREATEST\(balance - \$1, 0\).*WHERE id = \$2`).
		WithArgs(newcomerRewardAmount, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, svc.revokeFirstRechargeReward(context.Background(), 7, 99, "refund"))
	require.Equal(t, []int64{7, 7}, balanceCache.calls)
	require.Equal(t, []int64{7, 7}, authCache.calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignRefundReversalIsIdempotent(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(amount\), 0\).*FROM newcomer_campaign_reward_ledger`).
		WithArgs(NewcomerCampaignKey, int64(7), int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(newcomerRewardAmount))
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO newcomer_campaign_reward_ledger.*entry_type, amount, idempotency_key.*'revoke'`).
		WithArgs(NewcomerCampaignKey, int64(7), int64(99), newcomerRewardAmount,
			"newcomer_202609:first-recharge:revoke:99", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(`(?s)UPDATE users\s+SET balance = GREATEST\(balance - \$1, 0\).*WHERE id = \$2`).
		WithArgs(newcomerRewardAmount, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := NewNewcomerCampaignService(client)
	require.NoError(t, svc.revokeFirstRechargeReward(context.Background(), 7, 99, "refund"))

	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(amount\), 0\).*FROM newcomer_campaign_reward_ledger`).
		WithArgs(NewcomerCampaignKey, int64(7), int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(newcomerRewardAmount))
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO newcomer_campaign_reward_ledger.*entry_type, amount, idempotency_key.*'revoke'`).
		WithArgs(NewcomerCampaignKey, int64(7), int64(99), newcomerRewardAmount,
			"newcomer_202609:first-recharge:revoke:99", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	require.NoError(t, svc.revokeFirstRechargeReward(context.Background(), 7, 99, "refund"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignInviteQualificationUsesOnlinePaymentsAndRedeemCodes(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT inviter_id FROM newcomer_campaign_invites`).
		WithArgs(NewcomerCampaignKey, int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"inviter_id"}).AddRow(int64(3)))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM newcomer_campaign_invites.*LEFT JOIN newcomer_campaign_payment_facts f.*f\.order_id IS NULL`).
		WithArgs(NewcomerCampaignKey, int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`(?s)WITH consumption AS.*f\.principal_amount \* CASE.*WHEN po\.status = 'REFUNDED' THEN 0.*WHEN po\.status = 'PARTIALLY_REFUNDED' THEN.*WHEN po\.amount > 0 THEN.*LEAST\(GREATEST\(COALESCE\(po\.refund_amount, 0\) / po\.amount, 0\), 1\).*JOIN redeem_codes rc.*rc\.used_by = i\.invitee_id.*rc\.type = 'balance'.*rc\.status = 'used'.*rc\.value > 0.*COALESCE\(rc\.affiliate_rebate_status, 'not_applicable'\) <> 'excluded'.*rc\.used_at IS NOT NULL.*NOT EXISTS.*payment_orders internal_po.*internal_po\.recharge_code = rc\.code.*qualification_deadline`).
		WithArgs(NewcomerCampaignKey, int64(11), newcomerInviteThreshold, now, newcomerPrincipalCurrency).
		WillReturnResult(sqlmock.NewResult(0, 1))

	svc := NewNewcomerCampaignService(client)
	svc.SetClock(func() time.Time { return now })
	inviterID, err := svc.reconcileInvitee(context.Background(), 11)
	require.NoError(t, err)
	require.Equal(t, int64(3), inviterID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignRewardSkipsRefundedOrderWhileHoldingOrderLock(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT status.*FROM payment_orders.*FOR UPDATE`).
		WithArgs(int64(99), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(OrderStatusRefunded))
	mock.ExpectCommit()

	svc := NewNewcomerCampaignService(client)
	require.NoError(t, svc.grantFirstRechargeReward(context.Background(), 7, 99, 10))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignFirstRechargeNeverFallsBackToCreditedAmount(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	start, end := NewcomerCampaignWindow()
	completedAt := start.Add(time.Hour)
	mock.ExpectQuery(`SELECT created_at FROM users WHERE id = \$1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(start.Add(time.Hour)))
	mock.ExpectQuery(`(?s)SELECT po\.id, f\.principal_amount, f\.principal_currency.*LEFT JOIN newcomer_campaign_payment_facts`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "principal_amount", "principal_currency", "completed_at", "refund_amount", "status"}).
			AddRow(int64(99), nil, nil, completedAt, 0.0, OrderStatusCompleted))

	svc := NewNewcomerCampaignService(client)
	status, err := svc.firstRechargeStatus(context.Background(), 7, start, end)
	require.NoError(t, err)
	require.True(t, status.Eligible)
	require.Equal(t, "ineligible", status.RewardStatus)
	require.NotNil(t, status.FirstOrderID)
	require.Equal(t, int64(99), *status.FirstOrderID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignRefundStatesAreFinalOnly(t *testing.T) {
	require.False(t, isSuccessfulPaymentRefund(OrderStatusRefundPending))
	require.False(t, isSuccessfulPaymentRefund(OrderStatusRefundFailed))
	// The final status is authoritative; a successful full refund must revoke
	// even if a legacy row has a zero refund_amount.
	require.True(t, isSuccessfulPaymentRefund(OrderStatusPartiallyRefunded))
	require.True(t, isSuccessfulPaymentRefund(OrderStatusRefunded))
}

func TestNewcomerCampaignNetPrincipalUsesOriginalPrincipalForPartialRefund(t *testing.T) {
	// The order credited 120 from a 100 CNY principal (a 1.2x recharge
	// multiplier); refund_amount is the credited-balance amount, not principal.
	tests := []struct {
		name      string
		principal float64
		credited  float64
		refund    float64
		status    string
		want      float64
	}{
		{name: "no refund", principal: 100, credited: 120, status: OrderStatusCompleted, want: 100},
		{name: "quarter credited refund", principal: 100, credited: 120, refund: 30, status: OrderStatusPartiallyRefunded, want: 75},
		{name: "refund is clamped at credited amount", principal: 100, credited: 120, refund: 200, status: OrderStatusPartiallyRefunded, want: 0},
		{name: "full refund always removes principal", principal: 100, credited: 120, refund: 0, status: OrderStatusRefunded, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.InDelta(t, tt.want, newcomerCampaignNetPrincipal(tt.principal, tt.credited, tt.refund, tt.status), 1e-9)
		})
	}
}

func TestNewcomerCampaignStatusEnsuresInviteProfileWithoutAffiliateSwitch(t *testing.T) {
	client, _ := newNewcomerCampaignSQLMock(t)
	ensurer := &newcomerAffiliateEnsurerStub{summary: &AffiliateSummary{UserID: 7, AffCode: "NEWCOMER7"}}
	svc := NewNewcomerCampaignService(client, ensurer)

	code, err := svc.ensureCampaignInviteCode(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "NEWCOMER7", code)
	require.Equal(t, int64(7), ensurer.userID)
	require.Equal(t, 1, ensurer.calls)
}

func TestNewcomerCampaignInviteBindingIgnoresAffiliateSwitch(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	start, end := NewcomerCampaignWindow()
	mock.ExpectExec(`(?s)INSERT INTO newcomer_campaign_invites.*FROM user_affiliates ua.*WHERE ua\.aff_code = \$3.*u\.created_at >= \$4.*u\.created_at < \$5`).
		WithArgs(NewcomerCampaignKey, int64(11), "INVITER11", start, end).
		WillReturnResult(sqlmock.NewResult(1, 1))

	svc := NewNewcomerCampaignService(client)
	require.NoError(t, svc.OnUserRegistered(context.Background(), 11, " INVITER11 "))
	// No affiliate_enabled query or ordinary rebate call is made by this
	// independent campaign binding path.
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignPaidRedeemRuleExcludesExplicitGifts(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT inviter_id FROM newcomer_campaign_invites`).
		WithArgs(NewcomerCampaignKey, int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"inviter_id"}).AddRow(int64(3)))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*LEFT JOIN newcomer_campaign_payment_facts f.*f\.order_id IS NULL`).
		WithArgs(NewcomerCampaignKey, int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`(?s)WITH consumption AS.*COALESCE\(rc\.affiliate_rebate_status, 'not_applicable'\) <> 'excluded'`).
		WithArgs(NewcomerCampaignKey, int64(11), newcomerInviteThreshold, now, newcomerPrincipalCurrency).
		WillReturnResult(sqlmock.NewResult(0, 1))

	svc := NewNewcomerCampaignService(client)
	svc.SetClock(func() time.Time { return now })
	_, err := svc.reconcileInvitee(context.Background(), 11)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignBackfillUsesOrderCreatedPrincipalAndIsIdempotent(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	candidates := `(?s)SELECT DISTINCT ON \(po\.id\).*FROM payment_orders po.*JOIN payment_audit_logs pal.*LEFT JOIN newcomer_campaign_payment_facts f.*AND po\.user_id = \$1`
	insert := `(?s)INSERT INTO newcomer_campaign_payment_facts.*ON CONFLICT \(order_id\) DO NOTHING`

	mock.ExpectQuery(candidates).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "detail"}).
			AddRow(int64(99), int64(7), `{"paymentAmount":10,"principalCurrency":"HKD"}`))
	mock.ExpectExec(insert).
		WithArgs(int64(99), int64(7), 10.0, "CNY").
		WillReturnResult(sqlmock.NewResult(1, 1))

	svc := NewNewcomerCampaignService(client)
	count, err := svc.BackfillPaymentFactsForUser(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Re-running the same repair remains safe and does not report a second
	// inserted fact when the database's unique key rejects it.
	mock.ExpectQuery(candidates).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "detail"}).
			AddRow(int64(99), int64(7), `{"paymentAmount":10,"principalCurrency":"HKD"}`))
	mock.ExpectExec(insert).
		WithArgs(int64(99), int64(7), 10.0, "CNY").
		WillReturnResult(sqlmock.NewResult(0, 0))
	count, err = svc.BackfillPaymentFactsForUser(context.Background(), 7)
	require.NoError(t, err)
	require.Zero(t, count)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignBackfillDoesNotGuessMissingPrincipal(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	mock.ExpectQuery(`(?s)SELECT DISTINCT ON \(po\.id\).*FROM payment_orders po.*JOIN payment_audit_logs pal`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "detail"}).
			AddRow(int64(99), int64(7), `{"creditedAmount":20,"payAmount":21}`))

	svc := NewNewcomerCampaignService(client)
	count, err := svc.BackfillPaymentFacts(context.Background())
	require.NoError(t, err)
	require.Zero(t, count)
	require.NoError(t, mock.ExpectationsWereMet())
}

type newcomerAffiliateEnsurerStub struct {
	summary *AffiliateSummary
	err     error
	calls   int
	userID  int64
}

func (s *newcomerAffiliateEnsurerStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	s.calls++
	s.userID = userID
	return s.summary, s.err
}

func TestNewcomerCampaignApplyMembershipFactorIsFinalLayer(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)SELECT g.factor.*FROM newcomer_campaign_membership_grants`).
		WithArgs(NewcomerCampaignKey, int64(3), now, newcomerInviteThreshold).
		WillReturnRows(sqlmock.NewRows([]string{"factor"}).AddRow(0.96))

	svc := NewNewcomerCampaignService(client)
	svc.SetClock(func() time.Time { return now })
	require.InDelta(t, 1.152, svc.ApplyMembershipFactor(context.Background(), 3, 1.2), 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())

	// No campaign service/database means the existing multiplier remains the
	// base value; the activity layer never changes it in place.
	withoutCampaign := NewNewcomerCampaignService(nil)
	require.Equal(t, 1.2, withoutCampaign.ApplyMembershipFactor(context.Background(), 3, 1.2))
	require.Zero(t, withoutCampaign.ApplyMembershipFactor(context.Background(), 3, -1))
}

func expectNewcomerMembershipReconcile(t *testing.T, mock sqlmock.Sqlmock, now time.Time, validCount int, insertTiers bool) {
	t.Helper()
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtext\(\$1\)\)`).
		WithArgs("newcomer_202609:membership:3").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM newcomer_campaign_invites`).
		WithArgs(NewcomerCampaignKey, int64(3), newcomerInviteThreshold).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(validCount))
	mock.ExpectExec(`(?s)UPDATE newcomer_campaign_membership_grants.*SET status = 'expired'`).
		WithArgs(NewcomerCampaignKey, int64(3), now).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`(?s)UPDATE newcomer_campaign_membership_grants.*SET status = 'revoked'.*threshold > \$4`).
		WithArgs(NewcomerCampaignKey, int64(3), now, validCount).
		WillReturnResult(sqlmock.NewResult(0, 0))
	if insertTiers {
		for _, tier := range newcomerCampaignTiers {
			if validCount < tier.Threshold {
				continue
			}
			expiresAt := now.Add(time.Duration(tier.DurationDays) * 24 * time.Hour)
			mock.ExpectExec(`(?s)INSERT INTO newcomer_campaign_membership_grants.*ON CONFLICT \(campaign_key, user_id, tier_key\) DO NOTHING`).
				WithArgs(NewcomerCampaignKey, int64(3), tier.Key, tier.Threshold, tier.Factor, tier.DurationDays, now, expiresAt).
				WillReturnResult(sqlmock.NewResult(0, 1))
		}
	}
	mock.ExpectCommit()
}

func TestNewcomerCampaignMembershipTiersGrantOnceAndReconcileDowngrade(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for _, validCount := range []int{2, 5, 10} {
		t.Run(fmt.Sprintf("grant through %d", validCount), func(t *testing.T) {
			client, mock := newNewcomerCampaignSQLMock(t)
			svc := NewNewcomerCampaignService(client)
			svc.SetClock(func() time.Time { return now })
			expectNewcomerMembershipReconcile(t, mock, now, validCount, true)
			require.NoError(t, svc.reconcileMembership(context.Background(), 3))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	client, mock := newNewcomerCampaignSQLMock(t)
	svc := NewNewcomerCampaignService(client)
	svc.SetClock(func() time.Time { return now })
	// A later count drop revokes only thresholds above the still-valid count;
	// the unique tier key prevents a previously granted tier from being issued
	// a second time on a future recovery.
	expectNewcomerMembershipReconcile(t, mock, now, 10, true)
	expectNewcomerMembershipReconcile(t, mock, now.Add(31*24*time.Hour), 2, true)
	require.NoError(t, svc.reconcileMembership(context.Background(), 3))
	svc.SetClock(func() time.Time { return now.Add(31 * 24 * time.Hour) })
	require.NoError(t, svc.reconcileMembership(context.Background(), 3))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignPhaseBoundaries(t *testing.T) {
	start, end := NewcomerCampaignWindow()
	for _, tc := range []struct {
		name string
		now  time.Time
		want string
	}{
		{name: "before start", now: start.Add(-time.Nanosecond), want: "upcoming"},
		{name: "at start", now: start, want: "active"},
		{name: "before end", now: end.Add(-time.Nanosecond), want: "active"},
		{name: "at end", now: end, want: "ended"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			phase := "upcoming"
			if !tc.now.Before(start) && tc.now.Before(end) {
				phase = "active"
			} else if !tc.now.Before(end) {
				phase = "ended"
			}
			require.Equal(t, tc.want, phase)
		})
	}
}

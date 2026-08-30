package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestNewcomerCampaignAdminTierValidationAllowsThresholdEdits(t *testing.T) {
	tiers := NewcomerCampaignTiers()
	tiers[0].Factor = 0.97
	tiers[0].DurationDays = 31
	require.NoError(t, validateNewcomerCampaignTiers(tiers))

	tiers[0].Threshold = 3
	require.NoError(t, validateNewcomerCampaignTiers(tiers))

	tiers[1].Threshold = 2
	require.ErrorContains(t, validateNewcomerCampaignTiers(tiers), "thresholds must be ascending")
}

func TestNewcomerCampaignAdminTierValidationKeepsFactorAndDurationMonotonic(t *testing.T) {
	tiers := NewcomerCampaignTiers()
	tiers[1].Factor = 0.99
	require.ErrorContains(t, validateNewcomerCampaignTiers(tiers), "factor cannot increase")

	tiers = NewcomerCampaignTiers()
	tiers[1].DurationDays = 29
	require.ErrorContains(t, validateNewcomerCampaignTiers(tiers), "duration_days cannot decrease")
}

func TestNewcomerCampaignAdminUpdateConfigPersistsFutureGrantSettings(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	tiers := NewcomerCampaignTiers()
	tiers[0].Factor = 0.97
	tiers[0].DurationDays = 31

	mock.ExpectBegin()
	for _, tier := range tiers {
		mock.ExpectExec(`(?s)UPDATE newcomer_campaign_tier_configs.*SET factor = \$3, duration_days = \$4, updated_by = \$5`).
			WithArgs(NewcomerCampaignKey, tier.Key, tier.Factor, tier.DurationDays, int64(42)).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()
	mock.ExpectQuery(`(?s)SELECT tier_key, tier_name, threshold, factor, duration_days.*FROM newcomer_campaign_tier_configs`).
		WithArgs(NewcomerCampaignKey).
		WillReturnRows(sqlmock.NewRows([]string{"tier_key", "tier_name", "threshold", "factor", "duration_days"}).
			AddRow("premium", "高级", 2, 0.97, 31).
			AddRow("gold", "黄金", 5, 0.96, 45).
			AddRow("diamond", "钻石", 10, 0.94, 60))

	svc := NewNewcomerCampaignService(client)
	config, err := svc.AdminUpdateConfig(context.Background(), 42, tiers)
	require.NoError(t, err)
	require.Equal(t, 0.97, config.Tiers[0].Factor)
	require.Equal(t, 31, config.Tiers[0].DurationDays)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignAdminUpdateConfigEditsThresholdsAndWindow(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	svc := NewNewcomerCampaignService(client)
	svc.SetPersistentConfigEnabled(true)
	now := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, newcomerCampaignLocation).UTC()
	end := time.Date(2026, 11, 8, 0, 0, 0, 0, newcomerCampaignLocation).UTC()
	tiers := NewcomerCampaignTiers()
	tiers[0].Threshold, tiers[1].Threshold, tiers[2].Threshold = 3, 6, 12

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE newcomer_campaign_config.*SET starts_at = \$2, ends_at = \$3`).
		WithArgs(NewcomerCampaignKey, start, end, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE newcomer_campaign_tier_configs.*SET threshold = threshold \+ 1000000000`).
		WithArgs(NewcomerCampaignKey).
		WillReturnResult(sqlmock.NewResult(0, 3))
	for _, tier := range tiers {
		mock.ExpectExec(`(?s)UPDATE newcomer_campaign_tier_configs.*SET threshold = \$3, factor = \$4, duration_days = \$5.*WHERE campaign_key = \$1 AND tier_key = \$2`).
			WithArgs(NewcomerCampaignKey, tier.Key, tier.Threshold, tier.Factor, tier.DurationDays, int64(42)).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()
	mock.ExpectQuery(`(?s)SELECT tier_key, tier_name, threshold, factor, duration_days.*FROM newcomer_campaign_tier_configs`).
		WithArgs(NewcomerCampaignKey).
		WillReturnRows(sqlmock.NewRows([]string{"tier_key", "tier_name", "threshold", "factor", "duration_days"}).
			AddRow("premium", "高级", 3, 0.98, 30).
			AddRow("gold", "黄金", 6, 0.96, 45).
			AddRow("diamond", "钻石", 12, 0.94, 60))
	mock.ExpectQuery(`(?s)SELECT starts_at, ends_at.*FROM newcomer_campaign_config`).
		WithArgs(NewcomerCampaignKey).
		WillReturnRows(sqlmock.NewRows([]string{"starts_at", "ends_at"}).AddRow(start, end))
	mock.ExpectQuery(`(?s)SELECT name FROM newcomer_campaign_config`).
		WithArgs(NewcomerCampaignKey).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow(NewcomerCampaignName))

	config, err := svc.AdminUpdateConfigWithWindow(context.Background(), 42, tiers, &start, &end)
	require.NoError(t, err)
	require.Equal(t, []int{3, 6, 12}, []int{config.Tiers[0].Threshold, config.Tiers[1].Threshold, config.Tiers[2].Threshold})
	require.Equal(t, start, config.StartsAt)
	require.Equal(t, end, config.EndsAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignAdminSetMembershipUsesConfigAndWinsEffectiveFactor(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	now := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	cache := &newcomerAuthCacheInvalidatorStub{}
	svc := NewNewcomerCampaignService(client)
	svc.SetClock(func() time.Time { return now })
	svc.SetCacheInvalidators(nil, cache)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT tier_key, tier_name, threshold, factor, duration_days.*FROM newcomer_campaign_tier_configs`).
		WithArgs(NewcomerCampaignKey).
		WillReturnRows(sqlmock.NewRows([]string{"tier_key", "tier_name", "threshold", "factor", "duration_days"}).
			AddRow("premium", "高级", 2, 0.98, 30).
			AddRow("gold", "黄金", 5, 0.96, 45).
			AddRow("diamond", "钻石", 10, 0.94, 60))
	mock.ExpectQuery(`SELECT role FROM users WHERE id = \$1.*FOR UPDATE`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(RoleUser))
	mock.ExpectExec(`(?s)UPDATE newcomer_campaign_admin_memberships.*SET status = 'revoked'`).
		WithArgs(NewcomerCampaignKey, int64(7), now, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`(?s)INSERT INTO newcomer_campaign_admin_memberships.*VALUES \(\$1, \$2, \$3, \$4, \$5, \$6, 'active', \$7, \$8\)`).
		WithArgs(NewcomerCampaignKey, int64(7), "gold", 0.955, now, now.Add(40*24*time.Hour), int64(42), "support case").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// AdminGetUserMembership response after the write.
	mock.ExpectQuery(`SELECT email, COALESCE\(username, ''\).*FROM users`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"email", "username"}).AddRow("user@example.com", "demo"))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM newcomer_campaign_invites`).
		WithArgs(NewcomerCampaignKey, int64(7), newcomerInviteThreshold).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`(?s)UPDATE newcomer_campaign_admin_memberships.*SET status = 'expired'`).
		WithArgs(NewcomerCampaignKey, int64(7), now).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)SELECT tier_key, factor, starts_at, expires_at.*FROM newcomer_campaign_admin_memberships`).
		WithArgs(NewcomerCampaignKey, int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"tier_key", "factor", "starts_at", "expires_at"}).
			AddRow("gold", 0.955, now, now.Add(40*24*time.Hour)))
	mock.ExpectQuery(`(?s)SELECT tier_key, factor, starts_at, expires_at.*FROM \(.*sort_threshold.*ORDER BY priority DESC, sort_threshold DESC`).
		WithArgs(NewcomerCampaignKey, int64(7), now, 0).
		WillReturnRows(sqlmock.NewRows([]string{"tier_key", "factor", "starts_at", "expires_at"}).
			AddRow("gold", 0.955, now, now.Add(40*24*time.Hour)))

	result, err := svc.AdminSetUserMembership(context.Background(), 42, 7, NewcomerCampaignAdminMembershipInput{
		TierKey:      "gold",
		Factor:       func() *float64 { value := 0.955; return &value }(),
		DurationDays: func() *int { value := 40; return &value }(),
		Reason:       "support case",
	})
	require.NoError(t, err)
	require.Equal(t, "gold", result.CurrentMembership.TierKey)
	require.Equal(t, []int64{7}, cache.calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

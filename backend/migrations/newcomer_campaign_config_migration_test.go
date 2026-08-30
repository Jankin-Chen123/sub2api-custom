package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewcomerCampaignConfigMigrationPreservesSingleKeyHistoryAndEligibilitySnapshots(t *testing.T) {
	sql, err := FS.ReadFile("custom_20260831_newcomer_campaign_config.sql")
	require.NoError(t, err)
	text := string(sql)
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS newcomer_campaign_config",
		"newcomer_202609",
		"starts_at",
		"ends_at",
		"CREATE TABLE IF NOT EXISTS newcomer_campaign_eligible_users",
		"capture_deadline",
		"eligible_at_registration",
		"registration_window_start",
		"registration_window_end",
		"qualification_capture_deadline",
		"ON CONFLICT (campaign_key, user_id) DO NOTHING",
	} {
		require.Contains(t, text, fragment)
	}
	require.True(t, strings.Contains(text, "TIMESTAMPTZ '2026-09-01 00:00:00+08'"))
	require.True(t, strings.Contains(text, "TIMESTAMPTZ '2026-10-15 00:00:00+08'"))
}

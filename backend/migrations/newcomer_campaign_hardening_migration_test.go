package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewcomerCampaignHardeningMigrationContainsAuditableClawbackTrigger(t *testing.T) {
	sql, err := FS.ReadFile("custom_20260830_newcomer_campaign_hardening.sql")
	require.NoError(t, err)
	text := string(sql)
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS newcomer_campaign_clawback_debts",
		"CREATE TABLE IF NOT EXISTS newcomer_campaign_clawback_allocations",
		"due_amount",
		"recovered_amount",
		"source_type",
		"source_id",
		"CREATE OR REPLACE FUNCTION newcomer_campaign_consume_clawback_on_balance_credit",
		"BEFORE UPDATE OF balance ON users",
		"total_recharged",
		"campaign_reward",
		"FOR UPDATE",
	} {
		require.Contains(t, text, fragment)
	}
	require.True(t, strings.Contains(text, "ON CONFLICT (idempotency_key) DO NOTHING"))
}

package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyCheckinMigrationHasAtomicClaimAndDefaultWheel(t *testing.T) {
	content, err := FS.ReadFile("custom_20260828_daily_checkin.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS daily_checkin_prizes")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS daily_checkins")
	require.Contains(t, sql, "UNIQUE (user_id, checkin_date)")
	require.Contains(t, sql, "REFERENCES users(id) ON DELETE CASCADE")
	require.Equal(t, 8, strings.Count(sql, "('幸运 $"))
}

func TestDailyCheckinStreakMigrationAddsBonusSnapshotAndDefault(t *testing.T) {
	content, err := FS.ReadFile("custom_20260829_daily_checkin_streak.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS bonus_amount")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS streak_days")
	require.Contains(t, sql, "daily_checkin_streak_bonus_amount")
	require.Contains(t, sql, "5.00000000")
}

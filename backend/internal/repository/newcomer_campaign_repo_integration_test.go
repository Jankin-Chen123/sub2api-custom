//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNewcomerCampaignRepository_ReferralIntentSharesRegistrationTransaction(t *testing.T) {
	client := testEntClient(t)
	campaign := service.NewNewcomerCampaignServiceWithRepository(NewNewcomerCampaignRepository(client))
	campaign.SetPersistentConfigEnabled(true)
	var startsAt, endsAt time.Time
	err := integrationDB.QueryRowContext(context.Background(), "SELECT starts_at, ends_at FROM newcomer_campaign_config WHERE campaign_key = $1", service.NewcomerCampaignKey).Scan(&startsAt, &endsAt)
	require.NoError(t, err)
	registeredAt := startsAt.Add(endsAt.Sub(startsAt) / 2)
	for _, commit := range []bool{false, true} {
		name := "rollback"
		if commit {
			name = "commit"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			tx, err := client.Tx(ctx)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			user := mustCreateUser(t, tx.Client(), &service.User{CreatedAt: registeredAt})
			t.Cleanup(func() {
				_, cleanupErr := integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
				require.NoError(t, cleanupErr)
			})

			require.NoError(t, campaign.PersistReferralIntent(ent.NewTxContext(ctx, tx), user.ID, " abcd1234 ", "email"))
			if commit {
				require.NoError(t, tx.Commit())
			} else {
				require.NoError(t, tx.Rollback())
			}

			var count int
			err = integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM newcomer_campaign_referral_intents WHERE campaign_key = $1 AND invitee_id = $2", service.NewcomerCampaignKey, user.ID).Scan(&count)
			require.NoError(t, err)
			expected := 0
			if commit {
				expected = 1
			}
			require.Equal(t, expected, count)
			if commit {
				var inviteCode string
				var eligible bool
				err = integrationDB.QueryRowContext(ctx, "SELECT invite_code, eligible_at_registration FROM newcomer_campaign_referral_intents WHERE campaign_key = $1 AND invitee_id = $2", service.NewcomerCampaignKey, user.ID).Scan(&inviteCode, &eligible)
				require.NoError(t, err)
				require.Equal(t, "ABCD1234", inviteCode)
				require.True(t, eligible)
			}
		})
	}
}

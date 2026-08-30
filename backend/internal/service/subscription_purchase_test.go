package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIsExpiredSubscriptionRecord(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		status    string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "explicitly expired",
			status:    SubscriptionStatusExpired,
			expiresAt: now.Add(24 * time.Hour),
			want:      true,
		},
		{
			name:      "active but past expiration",
			status:    SubscriptionStatusActive,
			expiresAt: now.Add(-time.Minute),
			want:      true,
		},
		{
			name:      "active and valid",
			status:    SubscriptionStatusActive,
			expiresAt: now.Add(time.Minute),
			want:      false,
		},
		{
			name:      "pending purchase card",
			status:    SubscriptionStatusPending,
			expiresAt: now,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isExpiredSubscriptionRecord(tt.status, tt.expiresAt, now))
		})
	}
}

func TestExcludeExpiredSubscriptions(t *testing.T) {
	subs := []UserSubscription{
		{ID: 1, Status: SubscriptionStatusExpired},
		{ID: 2, Status: SubscriptionStatusPending},
		{ID: 3, Status: SubscriptionStatusActive},
	}

	visible := excludeExpiredSubscriptions(subs)
	require.Len(t, visible, 2)
	require.Equal(t, int64(2), visible[0].ID)
	require.Equal(t, int64(3), visible[1].ID)
}

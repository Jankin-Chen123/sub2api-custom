//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type adminFulfillmentRedeemRepo struct {
	*paymentFulfillmentRedeemRepo
}

func (r *adminFulfillmentRedeemRepo) GetByIDForUpdate(ctx context.Context, id int64) (*RedeemCode, error) {
	return r.GetByID(ctx, id)
}

func (r *adminFulfillmentRedeemRepo) UpdateAffiliateReview(_ context.Context, id int64, status string, amount *float64, reviewedAt time.Time) error {
	for _, code := range r.codesByCode {
		if code.ID != id {
			continue
		}
		code.AffiliateRebateStatus = status
		code.AffiliateRebateAmount = amount
		code.AffiliateRebateReviewedAt = &reviewedAt
		return nil
	}
	return ErrRedeemCodeNotFound
}

type adminFulfillmentAffiliateRepo struct {
	paymentFulfillmentAffiliateRepoStub
	redeemCodeIDs []int64
}

func (r *adminFulfillmentAffiliateRepo) AccrueQuotaForRedeemCode(_ context.Context, inviterID, inviteeUserID int64, amount float64, freezeHours int, redeemCodeID int64) (bool, error) {
	r.accrueCalls = append(r.accrueCalls, paymentFulfillmentAffiliateAccrueCall{
		inviterID:     inviterID,
		inviteeUserID: inviteeUserID,
		amount:        amount,
		freezeHours:   freezeHours,
	})
	r.redeemCodeIDs = append(r.redeemCodeIDs, redeemCodeID)
	return true, nil
}

func TestAdminFulfillmentBypassesLimitAndUsesRedeemAffiliateReview(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	userID := int64(42)
	inviterID := int64(9001)
	code := &RedeemCode{
		ID: 102, Code: "ADMIN-SUCCESS", Type: RedeemTypeBalance, Value: 20, Status: StatusUnused,
	}
	redeemRepo := &adminFulfillmentRedeemRepo{
		paymentFulfillmentRedeemRepo: &paymentFulfillmentRedeemRepo{
			paymentOrderLifecycleRedeemRepo: paymentOrderLifecycleRedeemRepo{
				codesByCode: map[string]*RedeemCode{code.Code: code},
			},
		},
	}
	userRepo := &mockUserRepo{getByIDUser: &User{ID: userID}}
	userRepo.updateBalanceFn = func(context.Context, int64, float64) error { return nil }
	cache := &paymentFulfillmentRedeemCacheStub{count: redeemMaxFailedAttempts}
	affiliateRepo := &adminFulfillmentAffiliateRepo{
		paymentFulfillmentAffiliateRepoStub: paymentFulfillmentAffiliateRepoStub{
			inviteeSummary: &AffiliateSummary{
				UserID: userID, AffCode: "INVITEE", InviterID: &inviterID, CreatedAt: time.Now().Add(-time.Hour),
			},
			inviterSummary: &AffiliateSummary{
				UserID: inviterID, AffCode: "INVITER", CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		},
	}
	settingSvc := NewSettingService(&paymentFulfillmentSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:                "true",
		SettingKeyAffiliateRebateRate:             "10",
		SettingKeyAffiliateRebateFreezeHours:      "0",
		SettingKeyAffiliateRedeemAutoValidAmounts: `[20]`,
	}}, nil)
	affiliateSvc := NewAffiliateService(affiliateRepo, settingSvc, nil, nil)
	svc := NewRedeemService(redeemRepo, userRepo, nil, cache, nil, client, nil, affiliateSvc)

	result, err := svc.RedeemForAdminFulfillment(ctx, userID, code.Code)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Zero(t, cache.getCalls)
	require.Zero(t, cache.incrementCalls)
	require.Equal(t, 1, cache.acquireCalls)
	require.Equal(t, 1, cache.releaseCalls)
	require.Len(t, redeemRepo.useCalls, 1)
	require.Len(t, affiliateRepo.accrueCalls, 1)
	require.Equal(t, inviterID, affiliateRepo.accrueCalls[0].inviterID)
	require.Equal(t, userID, affiliateRepo.accrueCalls[0].inviteeUserID)
	require.InDelta(t, 2, affiliateRepo.accrueCalls[0].amount, 1e-8)
	require.Nil(t, affiliateRepo.accrueCalls[0].sourceOrderID)
	require.Equal(t, []int64{code.ID}, affiliateRepo.redeemCodeIDs)
	require.Equal(t, AffiliateRebateStatusApproved, code.AffiliateRebateStatus)
}

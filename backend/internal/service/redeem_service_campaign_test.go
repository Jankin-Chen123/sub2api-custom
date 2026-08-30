package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type redeemCampaignReconcileStub struct {
	onRedeemUsers  []int64
	reconcileUsers []int64
}

func (s *redeemCampaignReconcileStub) OnRedeemCompleted(_ context.Context, userID int64, _ *RedeemCode) error {
	s.onRedeemUsers = append(s.onRedeemUsers, userID)
	return nil
}

func (s *redeemCampaignReconcileStub) ReconcileUser(_ context.Context, userID int64) error {
	s.reconcileUsers = append(s.reconcileUsers, userID)
	return nil
}

type redeemCampaignReviewRepo struct {
	*redeemRejectRepo
	codes map[int64]RedeemCode
}

func (r *redeemCampaignReviewRepo) GetByID(_ context.Context, id int64) (*RedeemCode, error) {
	code, ok := r.codes[id]
	if !ok {
		return nil, ErrRedeemCodeNotFound
	}
	clone := code
	return &clone, nil
}

func (r *redeemCampaignReviewRepo) GetByIDForUpdate(ctx context.Context, id int64) (*RedeemCode, error) {
	return r.GetByID(ctx, id)
}

func (r *redeemCampaignReviewRepo) UpdateAffiliateReview(_ context.Context, id int64, status string, amount *float64, _ time.Time) error {
	code, ok := r.codes[id]
	if !ok {
		return ErrRedeemCodeNotFound
	}
	code.AffiliateRebateStatus = status
	code.AffiliateRebateAmount = amount
	r.codes[id] = code
	return nil
}

func campaignReviewCode(id, userID int64) RedeemCode {
	return RedeemCode{
		ID:                    id,
		Type:                  RedeemTypeBalance,
		Value:                 10,
		Status:                StatusUsed,
		AffiliateRebateStatus: AffiliateRebateStatusPending,
		UsedBy:                &userID,
	}
}

func TestReviewAffiliateRedeemFreeReconcilesNewcomerCampaign(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	code := campaignReviewCode(41, 7)
	repo := &redeemCampaignReviewRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes:            map[int64]RedeemCode{code.ID: code},
	}
	campaign := &redeemCampaignReconcileStub{}
	svc := NewRedeemService(repo, nil, nil, nil, nil, client, nil, nil)
	svc.newcomerCampaign = campaign

	mock.ExpectBegin()
	mock.ExpectCommit()
	updated, err := svc.ReviewAffiliateRedeem(context.Background(), code.ID, "free")

	require.NoError(t, err)
	require.Equal(t, AffiliateRebateStatusExcluded, updated.AffiliateRebateStatus)
	require.Equal(t, []int64{7}, campaign.onRedeemUsers)

	// Repeating the same administrative decision remains safe and still runs
	// the idempotent campaign reconciliation against the now-excluded code.
	mock.ExpectBegin()
	mock.ExpectCommit()
	updated, err = svc.ReviewAffiliateRedeem(context.Background(), code.ID, "free")
	require.NoError(t, err)
	require.Equal(t, AffiliateRebateStatusExcluded, updated.AffiliateRebateStatus)
	require.Equal(t, []int64{7, 7}, campaign.onRedeemUsers)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReviewAffiliateRedeemsFreeReconcilesEachAffectedUserOnce(t *testing.T) {
	client, mock := newNewcomerCampaignSQLMock(t)
	codeA := campaignReviewCode(41, 7)
	codeB := campaignReviewCode(42, 8)
	codeDuplicateUser := campaignReviewCode(43, 7)
	repo := &redeemCampaignReviewRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes: map[int64]RedeemCode{
			codeA.ID:             codeA,
			codeB.ID:             codeB,
			codeDuplicateUser.ID: codeDuplicateUser,
		},
	}
	campaign := &redeemCampaignReconcileStub{}
	svc := NewRedeemService(repo, nil, nil, nil, nil, client, nil, nil)
	svc.newcomerCampaign = campaign

	mock.ExpectBegin()
	mock.ExpectCommit()
	result, err := svc.ReviewAffiliateRedeems(context.Background(), []int64{43, 41, 42, 41}, "free")

	require.NoError(t, err)
	require.Equal(t, 3, result.Processed)
	require.ElementsMatch(t, []int64{7, 8}, campaign.reconcileUsers)
	require.NoError(t, mock.ExpectationsWereMet())
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type imageBillingRepo struct {
	UsageBillingRepository
	reserved   []*BatchImageBalanceHoldCommand
	captured   []*BatchImageBalanceHoldCommand
	released   []*BatchImageBalanceHoldCommand
	applied    []*UsageBillingCommand
	releaseErr error
}

func (r *imageBillingRepo) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	copyValue := *cmd
	r.applied = append(r.applied, &copyValue)
	return &UsageBillingApplyResult{Applied: true}, nil
}

type imageBillingAPIKeys struct {
	apiKey *APIKey
}

func (s *imageBillingAPIKeys) GetByID(context.Context, int64) (*APIKey, error)    { return s.apiKey, nil }
func (s *imageBillingAPIKeys) InvalidateAuthCacheByKey(context.Context, string)   {}
func (s *imageBillingAPIKeys) InvalidateAuthCacheByUserID(context.Context, int64) {}

type imageBillingAccounts struct {
	account *Account
}

func (s *imageBillingAccounts) GetByID(context.Context, int64) (*Account, error) {
	return s.account, nil
}

type imageBillingUsageLogs struct {
	UsageLogRepository
	logs []*UsageLog
}

// idempotentImageBillingRepo models the request ledger used by the real
// repository. It also allows a test to fail after the upstream side effect
// has committed, which is the crash window the worker must safely replay.
type idempotentImageBillingRepo struct {
	imageBillingRepo
	seen            map[string]struct{}
	captureCalls    int
	captureApplied  int
	applyCalls      int
	applyApplied    int
	captureFailures int
	applyFailures   int
}

func (r *idempotentImageBillingRepo) claim(requestID string, failures *int) (bool, error) {
	if failures != nil && *failures > 0 {
		*failures = *failures - 1
		return false, errors.New("injected billing repository failure")
	}
	if r.seen == nil {
		r.seen = make(map[string]struct{})
	}
	if _, ok := r.seen[requestID]; ok {
		return false, nil
	}
	r.seen[requestID] = struct{}{}
	return true, nil
}

func (r *idempotentImageBillingRepo) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	r.applyCalls++
	applied, err := r.claim(cmd.RequestID, &r.applyFailures)
	if err != nil {
		return nil, err
	}
	if applied {
		r.applyApplied++
	}
	return &UsageBillingApplyResult{Applied: applied}, nil
}

func (r *idempotentImageBillingRepo) ReserveBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	applied, err := r.claim(cmd.RequestID, nil)
	if err != nil {
		return nil, err
	}
	return &BatchImageBalanceHoldResult{Applied: applied}, nil
}

func (r *idempotentImageBillingRepo) CaptureBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	r.captureCalls++
	applied, err := r.claim(cmd.RequestID, &r.captureFailures)
	if err != nil {
		return nil, err
	}
	if applied {
		r.captureApplied++
	}
	return &BatchImageBalanceHoldResult{Applied: applied}, nil
}

func (r *idempotentImageBillingRepo) ReleaseBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	applied, err := r.claim(cmd.RequestID, nil)
	if err != nil {
		return nil, err
	}
	return &BatchImageBalanceHoldResult{Applied: applied}, nil
}

func (s *imageBillingUsageLogs) Create(_ context.Context, log *UsageLog) (bool, error) {
	s.logs = append(s.logs, log)
	return true, nil
}

func (r *imageBillingRepo) ReserveBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	copyValue := *cmd
	r.reserved = append(r.reserved, &copyValue)
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}
func (r *imageBillingRepo) CaptureBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	copyValue := *cmd
	r.captured = append(r.captured, &copyValue)
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}
func (r *imageBillingRepo) ReleaseBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	copyValue := *cmd
	r.released = append(r.released, &copyValue)
	return &BatchImageBalanceHoldResult{Applied: true}, r.releaseErr
}

func TestUsageImageGenerationBillingUsesStableIdempotentLedgerCommands(t *testing.T) {
	userID, apiKeyID, accountID := int64(11), int64(22), int64(33)
	requestHash := "request-hash"
	job := &ImageGenerationJob{
		JobID: "imgjob_billing", UserID: &userID, APIKeyID: &apiKeyID, AccountID: &accountID,
		RequestHash: &requestHash, PublicModel: CangyuanImageModel2K,
		BaseCost: 0.1, RateMultiplier: 1.5, EstimatedCost: 0.15, HeldCost: 0.2,
	}
	repo := &imageBillingRepo{}
	usageLogs := &imageBillingUsageLogs{}
	billing := &UsageImageGenerationBilling{
		Repo:      repo,
		APIKeys:   &imageBillingAPIKeys{apiKey: &APIKey{ID: apiKeyID, UserID: userID}},
		Accounts:  &imageBillingAccounts{account: &Account{ID: accountID, Type: AccountTypeAPIKey}},
		UsageLogs: usageLogs,
	}

	require.NoError(t, billing.Hold(context.Background(), job))
	cost, err := billing.Settle(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, 0.15, cost)
	require.NoError(t, billing.Release(context.Background(), job))

	require.Equal(t, "image_generation_hold:imgjob_billing", repo.reserved[0].RequestID)
	require.Equal(t, "image_generation_capture:imgjob_billing", repo.captured[0].RequestID)
	require.Equal(t, "image_generation_release:imgjob_billing", repo.released[0].RequestID)
	require.Equal(t, "image_generation_usage:imgjob_billing", repo.applied[0].RequestID)
	require.Equal(t, 0.2, repo.reserved[0].HoldAmount)
	require.Equal(t, 0.15, repo.captured[0].ActualAmount)
	require.NotEmpty(t, repo.reserved[0].RequestFingerprint)
	require.Equal(t, requestHash, repo.reserved[0].RequestPayloadHash)
	require.Len(t, usageLogs.logs, 1)
	require.Equal(t, 0.1, usageLogs.logs[0].TotalCost)
	require.Equal(t, 0.15, usageLogs.logs[0].ActualCost)
	require.Equal(t, ImageBillingSize2K, *usageLogs.logs[0].ImageSize)
	require.Equal(t, ImageSizeSourceInput, *usageLogs.logs[0].ImageSizeSource)
	require.Equal(t, "2K", *usageLogs.logs[0].ImageInputSize)
	require.Equal(t, map[string]int{ImageBillingSize2K: 1}, usageLogs.logs[0].ImageSizeBreakdown)
}

func TestUsageImageGenerationBillingStoresTierAndDisplaySizeSeparately(t *testing.T) {
	userID, apiKeyID, accountID := int64(11), int64(22), int64(33)
	requestedSize := "3840x2160"
	actualSize := "3840x2160"
	job := &ImageGenerationJob{
		JobID: "imgjob_billing_4k", UserID: &userID, APIKeyID: &apiKeyID, AccountID: &accountID,
		PublicModel: CangyuanImageModel4K, RequestedSize: &requestedSize, ActualSize: &actualSize,
		BaseCost: 0.4, EstimatedCost: 0.4, HeldCost: 0.4,
	}
	usageLogs := &imageBillingUsageLogs{}
	billing := &UsageImageGenerationBilling{
		Repo:      &imageBillingRepo{},
		APIKeys:   &imageBillingAPIKeys{apiKey: &APIKey{ID: apiKeyID, UserID: userID}},
		Accounts:  &imageBillingAccounts{account: &Account{ID: accountID, Type: AccountTypeAPIKey}},
		UsageLogs: usageLogs,
	}

	_, err := billing.Settle(context.Background(), job)
	require.NoError(t, err)
	require.Len(t, usageLogs.logs, 1)
	log := usageLogs.logs[0]
	require.Equal(t, ImageBillingSize4K, *log.ImageSize)
	require.Equal(t, "3840x2160", *log.ImageInputSize)
	require.Equal(t, "3840x2160", *log.ImageOutputSize)
	require.Equal(t, ImageSizeSourceInput, *log.ImageSizeSource)
	require.Equal(t, map[string]int{ImageBillingSize4K: 1}, log.ImageSizeBreakdown)
}

func TestUsageImageGenerationBillingSubscriptionSkipsBalanceHoldAndSettlesUsage(t *testing.T) {
	userID, apiKeyID, accountID, subscriptionID := int64(11), int64(22), int64(33), int64(44)
	job := &ImageGenerationJob{
		JobID: "imgjob_subscription", UserID: &userID, APIKeyID: &apiKeyID, AccountID: &accountID,
		SubscriptionID: &subscriptionID, BillingType: BillingTypeSubscription,
		PublicModel: CangyuanImageModel4K, BaseCost: 0.2, RateMultiplier: 2, EstimatedCost: 0.4,
	}
	repo := &imageBillingRepo{}
	billing := &UsageImageGenerationBilling{
		Repo:      repo,
		APIKeys:   &imageBillingAPIKeys{apiKey: &APIKey{ID: apiKeyID, UserID: userID}},
		Accounts:  &imageBillingAccounts{account: &Account{ID: accountID, Type: AccountTypeAPIKey}},
		UsageLogs: &imageBillingUsageLogs{},
	}

	require.NoError(t, billing.Hold(context.Background(), job))
	cost, err := billing.Settle(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, 0.4, cost)
	require.NoError(t, billing.Release(context.Background(), job))

	require.Empty(t, repo.reserved)
	require.Empty(t, repo.captured)
	require.Empty(t, repo.released)
	require.Len(t, repo.applied, 1)
	require.Equal(t, &subscriptionID, repo.applied[0].SubscriptionID)
	require.Equal(t, 0.4, repo.applied[0].SubscriptionCost)
}

func TestUsageImageGenerationBillingRetriesAfterApplyFailureWithoutDoubleCharging(t *testing.T) {
	userID, apiKeyID, accountID := int64(11), int64(22), int64(33)
	job := &ImageGenerationJob{
		JobID: "imgjob_billing_retry", UserID: &userID, APIKeyID: &apiKeyID, AccountID: &accountID,
		PublicModel: CangyuanImageModel1K, BaseCost: 0.1, EstimatedCost: 0.1, HeldCost: 0.1,
	}
	repo := &idempotentImageBillingRepo{applyFailures: 1}
	billing := &UsageImageGenerationBilling{
		Repo:      repo,
		APIKeys:   &imageBillingAPIKeys{apiKey: &APIKey{ID: apiKeyID, UserID: userID}},
		Accounts:  &imageBillingAccounts{account: &Account{ID: accountID, Type: AccountTypeAPIKey}},
		UsageLogs: &imageBillingUsageLogs{},
	}

	require.NoError(t, billing.Hold(context.Background(), job))
	_, err := billing.Settle(context.Background(), job)
	require.Error(t, err)

	cost, err := billing.Settle(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, 0.1, cost)
	require.Equal(t, 2, repo.captureCalls, "the retry replays the idempotent capture lookup")
	require.Equal(t, 1, repo.captureApplied, "the balance capture is applied once")
	require.Equal(t, 2, repo.applyCalls, "the retry replays the idempotent usage lookup")
	require.Equal(t, 1, repo.applyApplied, "usage accounting is applied once")
}

func TestUsageImageGenerationBillingRepeatedLifecycleOperationsAreIdempotent(t *testing.T) {
	userID, apiKeyID, accountID := int64(11), int64(22), int64(33)
	job := &ImageGenerationJob{
		JobID: "imgjob_billing_idempotent", UserID: &userID, APIKeyID: &apiKeyID, AccountID: &accountID,
		PublicModel: CangyuanImageModel2K, BaseCost: 0.2, EstimatedCost: 0.2, HeldCost: 0.2,
	}
	repo := &idempotentImageBillingRepo{}
	billing := &UsageImageGenerationBilling{
		Repo:      repo,
		APIKeys:   &imageBillingAPIKeys{apiKey: &APIKey{ID: apiKeyID, UserID: userID}},
		Accounts:  &imageBillingAccounts{account: &Account{ID: accountID, Type: AccountTypeAPIKey}},
		UsageLogs: &imageBillingUsageLogs{},
	}

	require.NoError(t, billing.Hold(context.Background(), job))
	require.NoError(t, billing.Hold(context.Background(), job))
	_, err := billing.Settle(context.Background(), job)
	require.NoError(t, err)
	_, err = billing.Settle(context.Background(), job)
	require.NoError(t, err)
	require.NoError(t, billing.Release(context.Background(), job))
	require.NoError(t, billing.Release(context.Background(), job))

	require.Equal(t, 1, repo.captureApplied)
	require.Equal(t, 1, repo.applyApplied)
}

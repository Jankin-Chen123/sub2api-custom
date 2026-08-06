package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	imageGenerationHoldRequestPrefix    = "image_generation_hold:"
	imageGenerationCaptureRequestPrefix = "image_generation_capture:"
	imageGenerationReleaseRequestPrefix = "image_generation_release:"
	imageGenerationUsageRequestPrefix   = "image_generation_usage:"
)

type ImageGenerationBillingAPIKeys interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
	InvalidateAuthCacheByKey(ctx context.Context, key string)
	InvalidateAuthCacheByUserID(ctx context.Context, userID int64)
}

type ImageGenerationBillingAccounts interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}

type ImageGenerationBillingCache interface {
	QueueDeductBalance(userID int64, amount float64)
	InvalidateUserBalance(ctx context.Context, userID int64) error
	QueueUpdateAPIKeyRateLimitUsage(apiKeyID int64, cost float64)
}

type ImageGenerationBilling interface {
	Hold(ctx context.Context, job *ImageGenerationJob) error
	Release(ctx context.Context, job *ImageGenerationJob) error
	Settle(ctx context.Context, job *ImageGenerationJob) (float64, error)
}

// UsageImageGenerationBilling reuses the repository's transactional frozen
// balance ledger. Stable request IDs make every operation safe to retry after
// a process crash or a lost database response.
type UsageImageGenerationBilling struct {
	Repo      UsageBillingRepository
	APIKeys   ImageGenerationBillingAPIKeys
	Accounts  ImageGenerationBillingAccounts
	UsageLogs UsageLogRepository
	Cache     ImageGenerationBillingCache
}

func (b *UsageImageGenerationBilling) Hold(ctx context.Context, job *ImageGenerationJob) error {
	if job != nil && job.BillingType == BillingTypeSubscription {
		return nil
	}
	cmd, err := buildImageGenerationBillingCommand(job, imageGenerationHoldRequestPrefix+jobIDOrEmpty(job), 0)
	if err != nil {
		return err
	}
	if cmd.HoldAmount <= 0 {
		return nil
	}
	if b == nil || b.Repo == nil {
		return errors.New("image generation billing repository is not configured")
	}
	result, err := b.Repo.ReserveBatchImageBalance(ctx, cmd)
	if err == nil && result != nil && result.Applied {
		if b.Cache != nil {
			b.Cache.QueueDeductBalance(cmd.UserID, cmd.HoldAmount)
		}
		if b.APIKeys != nil {
			b.APIKeys.InvalidateAuthCacheByUserID(ctx, cmd.UserID)
		}
	}
	return err
}

func (b *UsageImageGenerationBilling) Release(ctx context.Context, job *ImageGenerationJob) error {
	if job != nil && job.BillingType == BillingTypeSubscription {
		return nil
	}
	cmd, err := buildImageGenerationBillingCommand(job, imageGenerationReleaseRequestPrefix+jobIDOrEmpty(job), 0)
	if err != nil {
		return err
	}
	if cmd.HoldAmount <= 0 {
		return nil
	}
	if b == nil || b.Repo == nil {
		return errors.New("image generation billing repository is not configured")
	}
	result, err := b.Repo.ReleaseBatchImageBalance(ctx, cmd)
	if errors.Is(err, ErrUsageBillingRequestConflict) {
		// A prior release with the same stable request ID already returned the
		// funds. Treat a legacy fingerprint mismatch as released.
		return nil
	}
	if err == nil && result != nil && result.Applied {
		b.invalidateBalanceCaches(ctx, cmd.UserID)
	}
	return err
}

func (b *UsageImageGenerationBilling) Settle(ctx context.Context, job *ImageGenerationJob) (float64, error) {
	actual := 0.0
	if job != nil {
		actual = job.EstimatedCost
		if actual < 0 {
			actual = 0
		}
	}
	cmd, err := buildImageGenerationBillingCommand(job, imageGenerationCaptureRequestPrefix+jobIDOrEmpty(job), actual)
	if err != nil {
		return 0, err
	}
	if b == nil || b.Repo == nil {
		return 0, errors.New("image generation billing repository is not configured")
	}
	if job.BillingType == BillingTypeSubscription {
		if job.SubscriptionID == nil || *job.SubscriptionID <= 0 {
			return 0, errors.New("image generation subscription billing identity is incomplete")
		}
	} else {
		result, captureErr := b.Repo.CaptureBatchImageBalance(ctx, cmd)
		if captureErr != nil {
			return 0, captureErr
		}
		if result != nil && result.Applied {
			b.invalidateBalanceCaches(ctx, cmd.UserID)
		}
	}
	if err := b.applyUsageAccounting(ctx, job, actual); err != nil {
		return 0, err
	}
	return actual, nil
}

func (b *UsageImageGenerationBilling) applyUsageAccounting(ctx context.Context, job *ImageGenerationJob, actual float64) error {
	if actual < 0 {
		actual = 0
	}
	if b == nil || b.Repo == nil || b.APIKeys == nil || b.Accounts == nil || b.UsageLogs == nil {
		return errors.New("image generation usage accounting is not configured")
	}
	if job == nil || job.APIKeyID == nil || job.UserID == nil || job.AccountID == nil {
		return errors.New("image generation usage accounting identity is incomplete")
	}
	apiKey, err := b.APIKeys.GetByID(ctx, *job.APIKeyID)
	if err != nil || apiKey == nil {
		return errors.New("image generation api key accounting snapshot is unavailable")
	}
	account, err := b.Accounts.GetByID(ctx, *job.AccountID)
	if err != nil || account == nil {
		return errors.New("image generation account accounting snapshot is unavailable")
	}
	baseCost := job.BaseCost
	if baseCost < 0 {
		baseCost = 0
	}
	accountRate := account.BillingRateMultiplier()
	accountStatsCost := baseCost * accountRate
	requestID := imageGenerationUsageRequestPrefix + job.JobID
	cmd := &UsageBillingCommand{
		RequestID: requestID, APIKeyID: *job.APIKeyID, UserID: *job.UserID,
		AccountID: account.ID, AccountType: account.Type, Model: job.PublicModel,
		BillingType: job.BillingType, ImageCount: 1, MediaType: "image",
		Platform: PlatformOpenAI, PlatformQuotaCost: actual,
		RequestPayloadHash: imageGenerationStringValue(job.RequestHash),
	}
	if job.BillingType == BillingTypeSubscription {
		cmd.SubscriptionID = job.SubscriptionID
		cmd.SubscriptionCost = actual
	}
	if apiKey.Quota > 0 {
		cmd.APIKeyQuotaCost = actual
	}
	if apiKey.HasRateLimits() {
		cmd.APIKeyRateLimitCost = actual
	}
	if account.IsAPIKeyOrBedrock() && account.HasAnyQuotaLimit() {
		cmd.AccountQuotaCost = accountStatsCost
	}
	cmd.Normalize()
	result, err := b.Repo.Apply(ctx, cmd)
	if err != nil {
		return err
	}
	if result != nil && result.Applied {
		if result.APIKeyQuotaExhausted && strings.TrimSpace(apiKey.Key) != "" {
			b.APIKeys.InvalidateAuthCacheByKey(ctx, apiKey.Key)
		}
		if cmd.APIKeyRateLimitCost > 0 && b.Cache != nil {
			b.Cache.QueueUpdateAPIKeyRateLimitUsage(apiKey.ID, actual)
		}
	}
	b.writeUsageLog(ctx, job, account, actual, baseCost, accountRate, accountStatsCost, requestID)
	return nil
}

func (b *UsageImageGenerationBilling) writeUsageLog(ctx context.Context, job *ImageGenerationJob, account *Account, actual, baseCost, accountRate, accountStatsCost float64, requestID string) {
	if b == nil || b.UsageLogs == nil || job == nil || job.UserID == nil || job.APIKeyID == nil || account == nil {
		return
	}
	inboundEndpoint := "/v1/images/generations"
	upstreamEndpoint := "/v1/images/generations"
	if job.Operation == ImageGenerationJobOperationEdit {
		inboundEndpoint = "/v1/images/edits"
		upstreamEndpoint = "/v1/images/edits"
	}
	if job.Source == ImageGenerationJobSourceWorkbench {
		inboundEndpoint = "/api/v1/user/image-workbench/jobs"
	}
	mediaType := "image"
	requestedSize := imageGenerationStringValue(job.RequestedSize)
	billingSize := imageGenerationJobBillingTier(job)
	if requestedSize == "" {
		requestedSize = billingSize
	}
	actualSize := imageGenerationStringValue(job.ActualSize)
	imageSizeSource := ImageSizeSourceInput
	imageSizeBreakdown := map[string]int{billingSize: 1}
	usageLog := &UsageLog{
		UserID: *job.UserID, APIKeyID: *job.APIKeyID, AccountID: account.ID,
		RequestID: requestID, Model: job.PublicModel, RequestedModel: job.PublicModel,
		UpstreamModel: job.UpstreamModel, GroupID: job.GroupID, SubscriptionID: job.SubscriptionID,
		InboundEndpoint: &inboundEndpoint, UpstreamEndpoint: &upstreamEndpoint,
		ImageCount: 1, ImageOutputCost: baseCost, TotalCost: baseCost, ActualCost: actual,
		RateMultiplier: job.RateMultiplier, AccountRateMultiplier: &accountRate, AccountStatsCost: &accountStatsCost,
		BillingType: job.BillingType, RequestType: RequestTypeSync, BillingMode: stringPointer(string(BillingModeImage)),
		ImageSize: &billingSize, ImageInputSize: &requestedSize,
		ImageOutputSize: stringPointer(actualSize), ImageSizeSource: &imageSizeSource,
		ImageSizeBreakdown: imageSizeBreakdown, MediaType: &mediaType, CreatedAt: time.Now(),
	}
	writeUsageLogBestEffort(ctx, b.UsageLogs, usageLog, "service.image_generation_billing")
}

func imageGenerationJobBillingTier(job *ImageGenerationJob) string {
	if job == nil {
		return ImageBillingSize2K
	}
	for _, model := range []string{imageGenerationStringValue(job.UpstreamModel), job.PublicModel} {
		if normalizeDedicatedImageModel(model) != "" {
			return dedicatedImageTierForModel(model)
		}
	}
	if tier, ok := ClassifyImageBillingTier(imageGenerationStringValue(job.RequestedSize)); ok {
		return tier
	}
	return ImageBillingSize2K
}

func (b *UsageImageGenerationBilling) invalidateBalanceCaches(ctx context.Context, userID int64) {
	if b == nil || userID <= 0 {
		return
	}
	if b.Cache != nil {
		_ = b.Cache.InvalidateUserBalance(ctx, userID)
	}
	if b.APIKeys != nil {
		b.APIKeys.InvalidateAuthCacheByUserID(ctx, userID)
	}
}

func buildImageGenerationBillingCommand(job *ImageGenerationJob, requestID string, actual float64) (*BatchImageBalanceHoldCommand, error) {
	if job == nil || job.UserID == nil || job.APIKeyID == nil || *job.UserID <= 0 || *job.APIKeyID <= 0 || strings.TrimSpace(job.JobID) == "" {
		return nil, errors.New("image generation billing identity is incomplete")
	}
	hold := job.HeldCost
	if hold <= 0 {
		hold = job.EstimatedCost
	}
	if hold < 0 {
		hold = 0
	}
	cmd := &BatchImageBalanceHoldCommand{
		RequestID:          strings.TrimSpace(requestID),
		HoldRequestID:      imageGenerationHoldRequestPrefix + job.JobID,
		APIKeyID:           *job.APIKeyID,
		UserID:             *job.UserID,
		BatchID:            job.JobID,
		HoldAmount:         hold,
		ActualAmount:       actual,
		RequestPayloadHash: imageGenerationStringValue(job.RequestHash),
	}
	cmd.Normalize()
	return cmd, nil
}

func jobIDOrEmpty(job *ImageGenerationJob) string {
	if job == nil {
		return ""
	}
	return strings.TrimSpace(job.JobID)
}

func imageGenerationStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ErrImageGenerationWorkerIdle = errors.New("no image generation job is ready")

type CangyuanImageClient interface {
	SubmitGeneration(ctx context.Context, request CangyuanImageRequest) (*CangyuanImageResult, error)
	SubmitEdit(ctx context.Context, request CangyuanImageRequest) (*CangyuanImageResult, error)
	PollGeneration(ctx context.Context, upstreamTaskID string) (*CangyuanImageResult, error)
	PollEdit(ctx context.Context, upstreamTaskID string) (*CangyuanImageResult, error)
}

type ImageGenerationProviderFactory interface {
	ForAccount(account *Account, requireImageOnly bool) (CangyuanImageClient, error)
}

type DefaultImageGenerationProviderFactory struct {
	HTTPClient *http.Client
}

func (f *DefaultImageGenerationProviderFactory) ForAccount(account *Account, requireImageOnly bool) (CangyuanImageClient, error) {
	if account == nil {
		return nil, errors.New("image provider account is required")
	}
	if requireImageOnly {
		return NewCangyuanImageAdapterFromAccount(account, f.HTTPClient)
	}
	// Already-submitted jobs stay pinned even if an administrator later changes
	// the account purpose or disables scheduling. Polling still uses the exact
	// stored account credentials and never resubmits elsewhere.
	apiKey := account.GetCredential("api_key")
	if apiKey == "" && account.IsOpenAIOAuth() {
		apiKey = account.GetOpenAIAccessToken()
	}
	return NewCangyuanImageAdapter(account.GetOpenAIBaseURL(), apiKey, f.HTTPClient)
}

type ImageGenerationAccountLease struct {
	Account   *Account
	ImageOnly bool
	Release   func()
}

type ImageGenerationAccountSelector interface {
	Select(ctx context.Context, job *ImageGenerationJob) (*ImageGenerationAccountLease, error)
	GetBoundAccount(ctx context.Context, accountID int64) (*Account, error)
}

type imageGenerationExecutionLeaser interface {
	AcquireExecution(ctx context.Context, job *ImageGenerationJob, accountID int64) (func(), bool, error)
}

type DedicatedImageAccountSelector struct {
	Gateway              *OpenAIGatewayService
	Accounts             AccountRepository
	ImageAdmission       *ImageGenerationAdmission
	AllowGeneralFallback bool
}

func (s *DedicatedImageAccountSelector) Select(ctx context.Context, job *ImageGenerationJob) (*ImageGenerationAccountLease, error) {
	if s == nil || s.Gateway == nil || job == nil {
		return nil, errors.New("dedicated image account selector is not configured")
	}
	selection, _, err := s.Gateway.SelectAccountWithSchedulerForDedicatedImages(
		ctx,
		job.GroupID,
		job.JobID,
		job.PublicModel,
		nil,
		OpenAIImagesCapabilityNative,
		s.AllowGeneralFallback,
	)
	if err != nil {
		return nil, err
	}
	if selection == nil || selection.Account == nil || (!selection.Account.IsImageOnly() && !s.AllowGeneralFallback) {
		if selection != nil && selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		return nil, errors.New("no dedicated image account is available")
	}
	selectedAccount := selection.Account
	// Scheduler snapshots intentionally omit provider endpoint secrets and may
	// also carry only metadata. Hydrate the selected account before submission so
	// the worker never tries to initialize Cangyuan with a redacted base URL or
	// stale credentials. The persisted job still binds the account by ID.
	if s.Accounts != nil {
		fresh, resolveErr := s.Accounts.GetByID(ctx, selectedAccount.ID)
		if resolveErr != nil || fresh == nil {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			if resolveErr != nil {
				return nil, fmt.Errorf("resolve selected image account: %w", resolveErr)
			}
			return nil, errors.New("selected image account is unavailable")
		}
		selectedAccount = fresh
	}
	recordImageGenerationAccountSelection(selectedAccount.IsImageOnly())
	imageRelease, acquired, err := s.AcquireExecution(ctx, job, accountIDOrZero(selectedAccount))
	if err != nil || !acquired {
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		if err != nil {
			return nil, err
		}
		return nil, errors.New("image concurrency limit exceeded")
	}
	var once sync.Once
	return &ImageGenerationAccountLease{
		Account:   selectedAccount,
		ImageOnly: selectedAccount.IsImageOnly(),
		Release: func() {
			once.Do(func() {
				if imageRelease != nil {
					imageRelease()
				}
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
			})
		},
	}, nil
}

func accountIDOrZero(account *Account) int64 {
	if account == nil {
		return 0
	}
	return account.ID
}

func (s *DedicatedImageAccountSelector) AcquireExecution(ctx context.Context, job *ImageGenerationJob, accountID int64) (func(), bool, error) {
	if s == nil || s.ImageAdmission == nil {
		return nil, true, nil
	}
	if job == nil || accountID <= 0 {
		return nil, false, errors.New("image execution identity is incomplete")
	}
	var userID, apiKeyID, groupID int64
	if job.UserID != nil {
		userID = *job.UserID
	}
	if job.APIKeyID != nil {
		apiKeyID = *job.APIKeyID
	}
	if job.GroupID != nil {
		groupID = *job.GroupID
	}
	tier := ""
	if strings.Contains(strings.ToLower(strings.TrimSpace(job.PublicModel)), "4k") {
		tier = ImageConcurrencyTier4K
	}
	return s.ImageAdmission.Acquire(ctx, ImageGenerationAdmissionRequest{
		UserID: userID, APIKeyID: apiKeyID, GroupID: groupID, AccountID: accountID, Tier: tier,
	})
}

func (s *DedicatedImageAccountSelector) GetBoundAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.Accounts == nil || accountID <= 0 {
		return nil, errors.New("bound image account resolver is not configured")
	}
	return s.Accounts.GetByID(ctx, accountID)
}

type ImageGenerationWorkerOptions struct {
	LeaseDuration     time.Duration
	PollInterval      time.Duration
	RetryDelay        time.Duration
	IdleDelay         time.Duration
	RecoveryInterval  time.Duration
	PayloadTTL        time.Duration
	MaxSubmitAttempts int
	RecoveryLimit     int
	Now               func() time.Time
}

type ImageGenerationWorker struct {
	repo      ImageGenerationJobRepository
	payloads  ImageGenerationPayloadStore
	results   ImageGenerationResultStore
	billing   ImageGenerationBilling
	accounts  ImageGenerationAccountSelector
	providers ImageGenerationProviderFactory
	queue     *ImageGenerationQueueController
	opts      ImageGenerationWorkerOptions
}

func NewImageGenerationWorker(
	repo ImageGenerationJobRepository,
	payloads ImageGenerationPayloadStore,
	results ImageGenerationResultStore,
	billing ImageGenerationBilling,
	accounts ImageGenerationAccountSelector,
	providers ImageGenerationProviderFactory,
	opts ImageGenerationWorkerOptions,
) *ImageGenerationWorker {
	return &ImageGenerationWorker{
		repo: repo, payloads: payloads, results: results, billing: billing,
		accounts: accounts, providers: providers, opts: normalizeImageGenerationWorkerOptions(opts),
	}
}

// SetQueueController keeps direct worker construction backwards compatible
// while allowing the application wiring to inject the server-wide guard.
func (w *ImageGenerationWorker) SetQueueController(queue *ImageGenerationQueueController) {
	if w != nil {
		w.queue = queue
	}
}

func normalizeImageGenerationWorkerOptions(opts ImageGenerationWorkerOptions) ImageGenerationWorkerOptions {
	if opts.LeaseDuration <= 0 {
		opts.LeaseDuration = time.Minute
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = 10 * time.Second
	}
	if opts.IdleDelay <= 0 {
		opts.IdleDelay = 500 * time.Millisecond
	}
	if opts.RecoveryInterval <= 0 {
		opts.RecoveryInterval = time.Minute
	}
	if opts.PayloadTTL <= 0 {
		opts.PayloadTTL = 6 * time.Hour
	}
	if opts.MaxSubmitAttempts <= 0 {
		opts.MaxSubmitAttempts = 3
	}
	if opts.RecoveryLimit <= 0 {
		opts.RecoveryLimit = 100
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return opts
}

func (w *ImageGenerationWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.repo == nil || w.payloads == nil || w.results == nil || w.billing == nil || w.accounts == nil || w.providers == nil {
		return errors.New("image generation worker is not fully configured")
	}
	now := w.opts.Now()
	job, err := w.repo.ClaimNextImageGenerationJob(ctx, now, w.opts.LeaseDuration)
	if errors.Is(err, ErrImageGenerationJobNotFound) {
		return ErrImageGenerationWorkerIdle
	}
	if err != nil {
		return err
	}
	if job == nil {
		return ErrImageGenerationWorkerIdle
	}
	recordImageGenerationClaimed()
	defer recordImageGenerationClaimFinished()

	processCtx, cancel := context.WithCancel(ctx)
	stopLease := make(chan struct{})
	leaseDone := make(chan error, 1)
	go w.renewLease(processCtx, cancel, job, stopLease, leaseDone)
	processErr := w.processClaim(processCtx, job)
	close(stopLease)
	leaseErr := <-leaseDone
	cancel()
	if processErr != nil {
		return processErr
	}
	return leaseErr
}

func (w *ImageGenerationWorker) renewLease(ctx context.Context, cancel context.CancelFunc, job *ImageGenerationJob, stop <-chan struct{}, done chan<- error) {
	interval := w.opts.LeaseDuration / 3
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			done <- nil
			return
		case <-ctx.Done():
			done <- nil
			return
		case <-ticker.C:
			if err := w.repo.RenewImageGenerationJobLease(ctx, job.JobID, job.ClaimVersion, w.opts.Now().Add(w.opts.LeaseDuration)); err != nil {
				recordImageGenerationClaimLeaseRenewalFailure()
				cancel()
				done <- err
				return
			}
		}
	}
}

func (w *ImageGenerationWorker) processClaim(ctx context.Context, job *ImageGenerationJob) error {
	switch job.Status {
	case ImageGenerationJobStatusCreated:
		return w.prepare(ctx, job)
	case ImageGenerationJobStatusSubmitting:
		return w.submit(ctx, job)
	case ImageGenerationJobStatusPolling, ImageGenerationJobStatusSubmitted:
		return w.poll(ctx, job)
	case ImageGenerationJobStatusStoring:
		return w.storeAndSettle(ctx, job)
	case ImageGenerationJobStatusSettling:
		return w.settle(ctx, job)
	default:
		return fmt.Errorf("unsupported claimed image generation status %q", job.Status)
	}
}

func (w *ImageGenerationWorker) prepare(ctx context.Context, job *ImageGenerationJob) error {
	if _, err := w.loadPayload(ctx, job); err != nil {
		return w.fail(ctx, job, "image_payload_unavailable", "image request payload is unavailable")
	}
	if err := w.billing.Hold(ctx, job); err != nil {
		return w.retry(ctx, job, ImageGenerationJobStatusCreated, "image_hold_failed", "image balance hold is temporarily unavailable")
	}
	heldCost := job.HeldCost
	if job.BillingType == BillingTypeSubscription {
		heldCost = 0
	} else if heldCost <= 0 {
		heldCost = job.EstimatedCost
	}
	return w.repo.QueueImageGenerationJob(ctx, job.JobID, job.ClaimVersion, heldCost, w.opts.Now())
}

func (w *ImageGenerationWorker) submit(ctx context.Context, job *ImageGenerationJob) error {
	payload, err := w.loadPayload(ctx, job)
	if err != nil {
		return w.fail(ctx, job, "image_payload_unavailable", err.Error())
	}
	if w.queue != nil {
		acquired, acquireErr := w.queue.Acquire(ctx, job.JobID)
		if acquireErr != nil {
			return w.retry(ctx, job, ImageGenerationJobStatusQueued, "image_queue_unavailable", "image generation capacity is temporarily unavailable")
		}
		if !acquired {
			return w.retry(ctx, job, ImageGenerationJobStatusQueued, "image_queue_full", "another image generation task is still running")
		}
	}
	lease, err := w.accounts.Select(ctx, job)
	if err != nil {
		recordImageGenerationProviderUnavailable()
		return w.retry(ctx, job, ImageGenerationJobStatusQueued, "image_provider_unavailable", "no dedicated image provider is currently available")
	}
	if lease == nil || lease.Account == nil {
		recordImageGenerationProviderUnavailable()
		return w.retry(ctx, job, ImageGenerationJobStatusQueued, "image_provider_unavailable", "no dedicated image provider is currently available")
	}
	if lease.Release != nil {
		defer lease.Release()
	}
	account := lease.Account
	upstreamModel, err := resolveCangyuanImageModel(account, job.PublicModel, lease.ImageOnly)
	if err != nil {
		return w.fail(ctx, job, "image_model_not_allowed", "the selected image account does not map the requested Cangyuan model")
	}
	request := payload.Request
	request.Model = upstreamModel
	client, err := w.providers.ForAccount(account, lease.ImageOnly)
	if err != nil {
		return w.fail(ctx, job, "image_provider_config_invalid", "the selected image provider configuration is invalid")
	}
	var result *CangyuanImageResult
	recordImageGenerationSubmission()
	upstreamStartedAt := time.Now()
	if job.Operation == ImageGenerationJobOperationEdit {
		result, err = client.SubmitEdit(ctx, request)
	} else {
		result, err = client.SubmitGeneration(ctx, request)
	}
	recordImageGenerationUpstreamLatency(upstreamStartedAt)
	if err != nil {
		return w.handleSubmitError(ctx, job, err)
	}
	if result == nil {
		return w.handleSubmitError(ctx, job, errors.New("image upstream returned an empty response"))
	}
	if result.Failed {
		recordImageGenerationUpstreamError()
		return w.fail(ctx, job, "image_upstream_rejected", "image upstream reported a failed task")
	}
	if result.Completed && len(result.Data) > 0 {
		payload.PendingResult = result
		if err := w.payloads.Save(ctx, *job.PayloadObjectRef, payload, w.opts.PayloadTTL); err != nil {
			// The upstream may already have generated and charged for the image.
			// Never resubmit when the completed output could not be persisted.
			return w.markSubmissionUnknown(ctx, job, "image_result_payload_failed", "completed image output could not be durably staged")
		}
		if err := w.repo.MarkImageGenerationJobStoringFromSubmission(ctx, job.JobID, job.ClaimVersion, account.ID, upstreamModel, "", w.opts.Now()); err != nil {
			return err
		}
		job.Status = ImageGenerationJobStatusStoring
		job.AccountID = int64Pointer(account.ID)
		job.UpstreamModel = stringPointer(upstreamModel)
		return w.storeAndSettle(ctx, job)
	}
	if strings.TrimSpace(result.UpstreamTaskID) == "" {
		return w.markSubmissionUnknown(ctx, job, "image_submission_unknown", "image upstream acceptance could not be determined")
	}
	return w.repo.MarkImageGenerationJobSubmitted(ctx, job.JobID, job.ClaimVersion, account.ID, upstreamModel, result.UpstreamTaskID, w.opts.Now())
}

func resolveCangyuanImageModel(account *Account, requestedModel string, imageOnly bool) (string, error) {
	if account == nil || strings.TrimSpace(requestedModel) == "" {
		return "", errors.New("image account and requested model are required")
	}
	upstreamModel, matched := account.ResolveMappedModel(requestedModel)
	// Both image_only accounts and the explicit general fallback must use an
	// account-level mapping. This prevents a normal OpenAI account with no
	// mapping from accidentally receiving Cangyuan-only model names.
	if !matched || strings.TrimSpace(upstreamModel) == "" {
		return "", errors.New("requested image model is not mapped")
	}
	if !imageOnly && !account.SupportsCangyuanImageFallback() {
		return "", errors.New("general fallback account is not Cangyuan-compatible")
	}
	if _, supported := cangyuanImageModels[strings.TrimSpace(upstreamModel)]; !supported {
		return "", errors.New("mapped model is not a supported Cangyuan image tier")
	}
	return strings.TrimSpace(upstreamModel), nil
}

func (w *ImageGenerationWorker) handleSubmitError(ctx context.Context, job *ImageGenerationJob, err error) error {
	recordImageGenerationUpstreamError()
	var adapterErr *CangyuanAdapterError
	if errors.As(err, &adapterErr) && adapterErr != nil {
		if adapterErr.SubmissionUnknown {
			return w.markSubmissionUnknown(ctx, job, adapterErr.Code, "image submission outcome is unknown")
		}
		if adapterErr.Retryable && job.AttemptCount < w.opts.MaxSubmitAttempts {
			return w.retry(ctx, job, ImageGenerationJobStatusQueued, adapterErr.Code, "image provider temporarily rejected the submission")
		}
		return w.fail(ctx, job, adapterErr.Code, "image provider rejected the submission")
	}
	return w.markSubmissionUnknown(ctx, job, "image_submission_unknown", "image submission outcome is unknown")
}

func (w *ImageGenerationWorker) poll(ctx context.Context, job *ImageGenerationJob) error {
	if job.AccountID == nil || job.UpstreamTaskID == nil {
		return w.markSubmissionUnknown(ctx, job, "image_task_binding_missing", "submitted image task lost its upstream binding")
	}
	if w.queue != nil {
		renewed, renewErr := w.queue.Renew(ctx, job.JobID)
		if renewErr != nil {
			return w.retry(ctx, job, ImageGenerationJobStatusPolling, "image_queue_unavailable", "image generation capacity is temporarily unavailable")
		}
		if !renewed {
			return w.retry(ctx, job, ImageGenerationJobStatusPolling, "image_queue_full", "image generation capacity is temporarily full")
		}
	}
	account, err := w.accounts.GetBoundAccount(ctx, *job.AccountID)
	if err != nil {
		return w.retry(ctx, job, ImageGenerationJobStatusPolling, "image_bound_account_unavailable", "the bound image account is temporarily unavailable")
	}
	if leaser, ok := w.accounts.(imageGenerationExecutionLeaser); ok {
		release, acquired, leaseErr := leaser.AcquireExecution(ctx, job, *job.AccountID)
		if leaseErr != nil {
			return w.retry(ctx, job, ImageGenerationJobStatusPolling, "image_provider_unavailable", "image concurrency service is temporarily unavailable")
		}
		if !acquired {
			return w.retry(ctx, job, ImageGenerationJobStatusPolling, "image_provider_busy", "the bound image provider is at its concurrency limit")
		}
		if release != nil {
			defer release()
		}
	}
	client, err := w.providers.ForAccount(account, false)
	if err != nil {
		recordImageGenerationProviderUnavailable()
		return w.retry(ctx, job, ImageGenerationJobStatusPolling, "image_bound_account_unavailable", "the bound image account could not be initialized")
	}
	var result *CangyuanImageResult
	recordImageGenerationPoll()
	upstreamStartedAt := time.Now()
	if job.Operation == ImageGenerationJobOperationEdit {
		result, err = client.PollEdit(ctx, *job.UpstreamTaskID)
	} else {
		result, err = client.PollGeneration(ctx, *job.UpstreamTaskID)
	}
	recordImageGenerationUpstreamLatency(upstreamStartedAt)
	if err != nil {
		recordImageGenerationUpstreamError()
		var adapterErr *CangyuanAdapterError
		if errors.As(err, &adapterErr) && adapterErr != nil && adapterErr.Retryable {
			return w.retry(ctx, job, ImageGenerationJobStatusPolling, adapterErr.Code, "image provider polling is temporarily unavailable")
		}
		code := "image_upstream_rejected"
		if adapterErr != nil && strings.TrimSpace(adapterErr.Code) != "" {
			code = adapterErr.Code
		}
		return w.fail(ctx, job, code, "image provider polling failed")
	}
	if result == nil {
		recordImageGenerationUpstreamError()
		return w.retry(ctx, job, ImageGenerationJobStatusPolling, "image_upstream_invalid_response", "image provider returned an empty polling response")
	}
	if result.Failed {
		recordImageGenerationUpstreamError()
		return w.fail(ctx, job, "image_upstream_rejected", "image upstream reported a failed task")
	}
	if !result.Completed || len(result.Data) == 0 {
		return w.repo.ScheduleImageGenerationJobPoll(ctx, job.JobID, job.ClaimVersion, w.opts.Now().Add(w.opts.PollInterval))
	}
	payload, err := w.loadPayload(ctx, job)
	if err != nil {
		// Once an upstream task ID exists the original prompt is no longer
		// required for polling. Redis loss can therefore be repaired by staging
		// the completed result into a fresh encrypted payload.
		payload = &ImageGenerationPayload{}
	}
	payload.PendingResult = result
	if err := w.payloads.Save(ctx, *job.PayloadObjectRef, payload, w.opts.PayloadTTL); err != nil {
		return w.retry(ctx, job, ImageGenerationJobStatusPolling, "image_result_payload_failed", "completed image output could not be durably staged")
	}
	if err := w.repo.MarkImageGenerationJobStoring(ctx, job.JobID, job.ClaimVersion, ""); err != nil {
		return err
	}
	job.Status = ImageGenerationJobStatusStoring
	return w.storeAndSettle(ctx, job)
}

func (w *ImageGenerationWorker) storeAndSettle(ctx context.Context, job *ImageGenerationJob) error {
	payload, err := w.loadPayload(ctx, job)
	if err != nil || payload.PendingResult == nil || len(payload.PendingResult.Data) == 0 {
		if job.AccountID != nil && job.UpstreamTaskID != nil {
			// Querying the same bound task is safe; it does not generate a second
			// image. This repairs a Redis loss between polling and object storage.
			return w.retry(ctx, job, ImageGenerationJobStatusPolling, "image_result_payload_missing", "completed image output will be queried again from the bound task")
		}
		// A synchronous response has no task ID to query again. Its submission
		// may already have been billed upstream, so never resubmit automatically.
		return w.markSubmissionUnknown(ctx, job, "image_result_payload_missing", "synchronous image output was lost before object storage")
	}
	refs, actualSize, err := w.results.Store(ctx, job.JobID, payload.PendingResult.Data)
	if err != nil {
		recordImageGenerationStorageFailure()
		return w.retry(ctx, job, ImageGenerationJobStatusStoring, "image_storage_failed", "completed image output could not be stored")
	}
	if err := w.repo.MarkImageGenerationJobSettling(ctx, job.JobID, job.ClaimVersion, refs, actualSize, w.opts.Now()); err != nil {
		return err
	}
	job.Status = ImageGenerationJobStatusSettling
	job.ResultObjectRefs = append([]string(nil), refs...)
	job.ActualSize = stringPointer(actualSize)
	return w.settle(ctx, job)
}

func (w *ImageGenerationWorker) settle(ctx context.Context, job *ImageGenerationJob) error {
	actualCost, err := w.billing.Settle(ctx, job)
	if err != nil {
		recordImageGenerationSettlementFailure()
		return w.retry(ctx, job, ImageGenerationJobStatusSettling, "image_settlement_failed", "image billing settlement is temporarily unavailable")
	}
	if err := w.repo.MarkImageGenerationJobCompleted(ctx, job.JobID, job.ClaimVersion, actualCost, w.opts.Now()); err != nil {
		return err
	}
	recordImageGenerationTerminal(ImageGenerationJobStatusCompleted)
	w.releaseQueueSlot(job)
	w.deletePayloadBestEffort(ctx, job)
	return nil
}

func (w *ImageGenerationWorker) fail(ctx context.Context, job *ImageGenerationJob, code, message string) error {
	if err := w.billing.Release(ctx, job); err != nil {
		return w.retry(ctx, job, retryStatusForImageGenerationJob(job.Status), "image_hold_release_failed", "image balance hold release is temporarily unavailable")
	}
	if err := w.repo.MarkImageGenerationJobFailed(ctx, job.JobID, job.ClaimVersion, code, message, w.opts.Now()); err != nil {
		return err
	}
	recordImageGenerationTerminal(ImageGenerationJobStatusFailed)
	w.releaseQueueSlot(job)
	w.deletePayloadBestEffort(ctx, job)
	return nil
}

func (w *ImageGenerationWorker) markSubmissionUnknown(ctx context.Context, job *ImageGenerationJob, code, message string) error {
	err := w.repo.MarkImageGenerationJobSubmissionUnknown(ctx, job.JobID, job.ClaimVersion, code, message, w.opts.Now())
	if err == nil {
		recordImageGenerationTerminal(ImageGenerationJobStatusSubmissionUnknown)
		// submission_unknown is terminal: the upstream outcome must be
		// reconciled manually and this job will never be polled by the worker
		// again. Keeping its server-wide execution lease until Redis TTL expiry
		// blocks otherwise unrelated image jobs, which is especially visible
		// when the configured maximum active count is one.
		w.releaseQueueSlot(job)
	}
	return err
}

func (w *ImageGenerationWorker) retry(ctx context.Context, job *ImageGenerationJob, status, code, message string) error {
	recordImageGenerationRetry()
	err := w.repo.ReleaseImageGenerationJobForRetry(ctx, job.JobID, job.ClaimVersion, status, code, message, w.opts.Now().Add(w.opts.RetryDelay))
	if err == nil && (status == ImageGenerationJobStatusCreated || status == ImageGenerationJobStatusQueued) {
		w.releaseQueueSlot(job)
	}
	return err
}

func (w *ImageGenerationWorker) releaseQueueSlot(job *ImageGenerationJob) {
	if w == nil || w.queue == nil || job == nil || strings.TrimSpace(job.JobID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = w.queue.Release(ctx, job.JobID)
}

func (w *ImageGenerationWorker) loadPayload(ctx context.Context, job *ImageGenerationJob) (*ImageGenerationPayload, error) {
	if job == nil || job.PayloadObjectRef == nil || strings.TrimSpace(*job.PayloadObjectRef) == "" {
		return nil, ErrImageGenerationPayloadNotFound
	}
	return w.payloads.Get(ctx, *job.PayloadObjectRef)
}

func (w *ImageGenerationWorker) deletePayloadBestEffort(ctx context.Context, job *ImageGenerationJob) {
	if job != nil && job.PayloadObjectRef != nil {
		_ = w.payloads.Delete(ctx, *job.PayloadObjectRef)
	}
}

func retryStatusForImageGenerationJob(status string) string {
	switch status {
	case ImageGenerationJobStatusCreated:
		return ImageGenerationJobStatusCreated
	case ImageGenerationJobStatusSubmitting:
		return ImageGenerationJobStatusQueued
	case ImageGenerationJobStatusSubmitted, ImageGenerationJobStatusPolling:
		return ImageGenerationJobStatusPolling
	case ImageGenerationJobStatusStoring:
		return ImageGenerationJobStatusStoring
	case ImageGenerationJobStatusSettling:
		return ImageGenerationJobStatusSettling
	default:
		return ImageGenerationJobStatusQueued
	}
}

func int64Pointer(value int64) *int64    { return &value }
func stringPointer(value string) *string { return &value }

type ImageGenerationWorkerRuntime struct {
	worker *ImageGenerationWorker
	wakeup ImageGenerationWakeup
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	wake   chan struct{}
}

func NewImageGenerationWorkerRuntime(worker *ImageGenerationWorker, wakeups ...ImageGenerationWakeup) *ImageGenerationWorkerRuntime {
	var wakeup ImageGenerationWakeup
	if len(wakeups) > 0 {
		wakeup = wakeups[0]
	}
	return &ImageGenerationWorkerRuntime{worker: worker, wakeup: wakeup}
}

func (r *ImageGenerationWorkerRuntime) Start() {
	if r == nil || r.worker == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	// Runtime may be constructed around a zero-value worker by an embedding
	// caller or a test. Normalize here as well as in the worker constructor so
	// the recovery ticker can never receive a non-positive interval.
	r.worker.opts = normalizeImageGenerationWorkerOptions(r.worker.opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	wake := make(chan struct{}, 1)
	r.cancel = cancel
	r.done = done
	r.wake = wake
	var wg sync.WaitGroup
	workerWakeup := r.wakeup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			if ctx.Err() != nil {
				return
			}
			err := r.worker.RunOnce(ctx)
			if err == nil {
				continue
			}
			delay := r.worker.opts.RetryDelay
			if errors.Is(err, ErrImageGenerationWorkerIdle) {
				delay = r.worker.opts.IdleDelay
			}
			sleepOrWake(ctx, delay, wake)
		}
	}()
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(r.worker.opts.RecoveryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				recovered, err := r.worker.repo.RecoverExpiredImageGenerationJobLeases(ctx, r.worker.opts.Now(), r.worker.opts.RecoveryLimit)
				if err != nil {
					continue
				}
				for _, item := range recovered {
					if item.Status != ImageGenerationJobStatusSubmissionUnknown {
						continue
					}
					recordImageGenerationTerminal(ImageGenerationJobStatusSubmissionUnknown)
					r.worker.releaseQueueSlot(&ImageGenerationJob{JobID: item.JobID})
				}
			}
		}
	}()
	if workerWakeup != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			backoff := 100 * time.Millisecond
			for ctx.Err() == nil {
				err := workerWakeup.SubscribeImageGenerationWakeups(ctx, func(string) {
					select {
					case wake <- struct{}{}:
					default:
					}
				})
				if ctx.Err() != nil {
					return
				}
				// A subscriber is expected to block until cancellation. Treat an
				// unexpected return (including nil) as a disconnected listener and
				// reconnect with bounded backoff.
				_ = err
				sleepOrDone(ctx, backoff)
				if backoff < 5*time.Second {
					backoff *= 2
					if backoff > 5*time.Second {
						backoff = 5 * time.Second
					}
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(done)
	}()
}

func (r *ImageGenerationWorkerRuntime) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.cancel, r.done, r.wake = nil, nil, nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// sleepOrWake keeps the database polling fallback but lets a durable job
// notification interrupt an idle/backoff delay immediately.
func sleepOrWake(ctx context.Context, d time.Duration, wake <-chan struct{}) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	case <-wake:
	}
}

func (r *ImageGenerationWorkerRuntime) Running() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel != nil
}

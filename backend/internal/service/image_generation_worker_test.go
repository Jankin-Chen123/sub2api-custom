package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageWorkerRepo struct {
	ImageGenerationJobRepository
	mu  sync.Mutex
	job *ImageGenerationJob
}

func (r *imageWorkerRepo) ClaimNextImageGenerationJob(_ context.Context, now time.Time, lease time.Duration) (*ImageGenerationJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job == nil || r.job.Status == ImageGenerationJobStatusCompleted || r.job.Status == ImageGenerationJobStatusFailed || r.job.Status == ImageGenerationJobStatusSubmissionUnknown {
		return nil, ErrImageGenerationJobNotFound
	}
	if r.job.LeaseExpiresAt != nil && r.job.LeaseExpiresAt.After(now) {
		return nil, ErrImageGenerationJobNotFound
	}
	switch r.job.Status {
	case ImageGenerationJobStatusQueued:
		r.job.Status = ImageGenerationJobStatusSubmitting
		r.job.AttemptCount++
	case ImageGenerationJobStatusSubmitted:
		r.job.Status = ImageGenerationJobStatusPolling
	}
	r.job.ClaimVersion++
	expires := now.Add(lease)
	r.job.LeaseExpiresAt = &expires
	return r.job, nil
}

func (r *imageWorkerRepo) RenewImageGenerationJobLease(_ context.Context, _ string, claimVersion int64, leaseUntil time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion {
		return ErrImageGenerationClaimLost
	}
	r.job.LeaseExpiresAt = &leaseUntil
	return nil
}

func (r *imageWorkerRepo) QueueImageGenerationJob(_ context.Context, _ string, claimVersion int64, heldCost float64, queuedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion || r.job.Status != ImageGenerationJobStatusCreated {
		return ErrImageGenerationClaimLost
	}
	r.job.Status = ImageGenerationJobStatusQueued
	r.job.HeldCost = heldCost
	r.job.NextAttemptAt = &queuedAt
	r.job.LeaseExpiresAt = nil
	return nil
}

func (r *imageWorkerRepo) MarkImageGenerationJobSubmitted(_ context.Context, _ string, claimVersion, accountID int64, upstreamModel, taskID string, submittedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion || r.job.Status != ImageGenerationJobStatusSubmitting {
		return ErrImageGenerationClaimLost
	}
	r.job.Status = ImageGenerationJobStatusSubmitted
	r.job.AccountID = int64Pointer(accountID)
	r.job.UpstreamModel = stringPointer(upstreamModel)
	r.job.UpstreamTaskID = stringPointer(taskID)
	r.job.SubmittedAt = &submittedAt
	r.job.LeaseExpiresAt = nil
	return nil
}

func (r *imageWorkerRepo) MarkImageGenerationJobStoringFromSubmission(_ context.Context, _ string, claimVersion, accountID int64, upstreamModel, actualSize string, submittedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion || r.job.Status != ImageGenerationJobStatusSubmitting {
		return ErrImageGenerationClaimLost
	}
	r.job.Status = ImageGenerationJobStatusStoring
	r.job.AccountID = int64Pointer(accountID)
	r.job.UpstreamModel = stringPointer(upstreamModel)
	r.job.ActualSize = stringPointer(actualSize)
	r.job.SubmittedAt = &submittedAt
	return nil
}

func (r *imageWorkerRepo) ScheduleImageGenerationJobPoll(_ context.Context, _ string, claimVersion int64, next time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion {
		return ErrImageGenerationClaimLost
	}
	r.job.Status = ImageGenerationJobStatusPolling
	r.job.NextAttemptAt = &next
	r.job.LeaseExpiresAt = nil
	return nil
}

func (r *imageWorkerRepo) MarkImageGenerationJobStoring(_ context.Context, _ string, claimVersion int64, actualSize string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion {
		return ErrImageGenerationClaimLost
	}
	r.job.Status = ImageGenerationJobStatusStoring
	r.job.ActualSize = stringPointer(actualSize)
	return nil
}

func (r *imageWorkerRepo) MarkImageGenerationJobSettling(_ context.Context, _ string, claimVersion int64, refs []string, actualSize string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion || r.job.Status != ImageGenerationJobStatusStoring {
		return ErrImageGenerationClaimLost
	}
	r.job.Status = ImageGenerationJobStatusSettling
	r.job.ResultObjectRefs = append([]string(nil), refs...)
	r.job.ActualSize = stringPointer(actualSize)
	return nil
}

func (r *imageWorkerRepo) MarkImageGenerationJobCompleted(_ context.Context, _ string, claimVersion int64, cost float64, completedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion || r.job.Status != ImageGenerationJobStatusSettling {
		return ErrImageGenerationClaimLost
	}
	r.job.Status = ImageGenerationJobStatusCompleted
	r.job.SettledCost = cost
	r.job.CompletedAt = &completedAt
	r.job.LeaseExpiresAt = nil
	return nil
}

func (r *imageWorkerRepo) MarkImageGenerationJobFailed(_ context.Context, _ string, claimVersion int64, code, message string, completedAt time.Time) error {
	return r.markTerminal(claimVersion, ImageGenerationJobStatusFailed, code, message, completedAt)
}

func (r *imageWorkerRepo) MarkImageGenerationJobSubmissionUnknown(_ context.Context, _ string, claimVersion int64, code, message string, completedAt time.Time) error {
	return r.markTerminal(claimVersion, ImageGenerationJobStatusSubmissionUnknown, code, message, completedAt)
}

func (r *imageWorkerRepo) markTerminal(claimVersion int64, status, code, message string, completedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion {
		return ErrImageGenerationClaimLost
	}
	r.job.Status = status
	r.job.ErrorCode = stringPointer(code)
	r.job.ErrorMessage = stringPointer(message)
	r.job.CompletedAt = &completedAt
	r.job.LeaseExpiresAt = nil
	return nil
}

func (r *imageWorkerRepo) ReleaseImageGenerationJobForRetry(_ context.Context, _ string, claimVersion int64, status, code, message string, next time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.ClaimVersion != claimVersion {
		return ErrImageGenerationClaimLost
	}
	r.job.Status = status
	r.job.ErrorCode = stringPointer(code)
	r.job.ErrorMessage = stringPointer(message)
	r.job.NextAttemptAt = &next
	r.job.LeaseExpiresAt = nil
	return nil
}

func (r *imageWorkerRepo) RecoverExpiredImageGenerationJobLeases(_ context.Context, now time.Time, _ int) ([]ImageGenerationJobRecovery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job == nil || r.job.LeaseExpiresAt == nil || r.job.LeaseExpiresAt.After(now) {
		return nil, nil
	}
	switch r.job.Status {
	case ImageGenerationJobStatusSubmitting:
		r.job.Status = ImageGenerationJobStatusSubmissionUnknown
		r.job.ErrorCode = stringPointer("image_submission_unknown")
		r.job.ErrorMessage = stringPointer("worker lease expired while upstream submission outcome was unknown")
		r.job.CompletedAt = &now
	case ImageGenerationJobStatusSubmitted:
		r.job.Status = ImageGenerationJobStatusPolling
		r.job.NextAttemptAt = &now
	}
	r.job.LeaseExpiresAt = nil
	return []ImageGenerationJobRecovery{{JobID: r.job.JobID, Status: r.job.Status}}, nil
}

type imageWorkerPayloadStore struct {
	payload *ImageGenerationPayload
	deleted bool
}

func (s *imageWorkerPayloadStore) Save(_ context.Context, _ string, payload *ImageGenerationPayload, _ time.Duration) error {
	copyValue := *payload
	s.payload = &copyValue
	return nil
}
func (s *imageWorkerPayloadStore) Get(context.Context, string) (*ImageGenerationPayload, error) {
	if s.payload == nil {
		return nil, ErrImageGenerationPayloadNotFound
	}
	copyValue := *s.payload
	return &copyValue, nil
}
func (s *imageWorkerPayloadStore) Delete(context.Context, string) error { s.deleted = true; return nil }

type imageWorkerResultStore struct {
	calls int
	err   error
}

func (s *imageWorkerResultStore) Store(context.Context, string, []CangyuanImageData) ([]string, string, error) {
	s.calls++
	if s.err != nil {
		return nil, "", s.err
	}
	return []string{"image-results/imgjob_test/0.png"}, "1024x1024", nil
}

type imageWorkerBilling struct {
	holdCalls    int
	releaseCalls int
	settleCalls  int
	settleErr    error
}

func (b *imageWorkerBilling) Hold(context.Context, *ImageGenerationJob) error {
	b.holdCalls++
	return nil
}
func (b *imageWorkerBilling) Release(context.Context, *ImageGenerationJob) error {
	b.releaseCalls++
	return nil
}
func (b *imageWorkerBilling) Settle(_ context.Context, job *ImageGenerationJob) (float64, error) {
	b.settleCalls++
	if b.settleErr != nil {
		return 0, b.settleErr
	}
	return job.EstimatedCost, nil
}

type imageWorkerAccountSelector struct {
	account     *Account
	selectCalls int
	boundIDs    []int64
	releases    int
}

func (s *imageWorkerAccountSelector) Select(context.Context, *ImageGenerationJob) (*ImageGenerationAccountLease, error) {
	s.selectCalls++
	return &ImageGenerationAccountLease{Account: s.account, ImageOnly: s.account != nil && s.account.IsImageOnly(), Release: func() { s.releases++ }}, nil
}
func (s *imageWorkerAccountSelector) GetBoundAccount(_ context.Context, accountID int64) (*Account, error) {
	s.boundIDs = append(s.boundIDs, accountID)
	return s.account, nil
}

type imageWorkerClient struct {
	submitResults []*CangyuanImageResult
	submitErrors  []error
	pollResults   []*CangyuanImageResult
	pollErrors    []error
	submitCalls   int
	pollCalls     int
	pollTaskIDs   []string
}

type blockingImageWorkerClient struct {
	*imageWorkerClient
	started chan struct{}
	resume  chan struct{}
}

func (c *blockingImageWorkerClient) SubmitGeneration(ctx context.Context, request CangyuanImageRequest) (*CangyuanImageResult, error) {
	select {
	case <-c.started:
	default:
		close(c.started)
	}
	select {
	case <-c.resume:
		return c.imageWorkerClient.SubmitGeneration(ctx, request)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *imageWorkerClient) nextSubmit() (*CangyuanImageResult, error) {
	idx := c.submitCalls
	c.submitCalls++
	var result *CangyuanImageResult
	var err error
	if idx < len(c.submitResults) {
		result = c.submitResults[idx]
	}
	if idx < len(c.submitErrors) {
		err = c.submitErrors[idx]
	}
	return result, err
}
func (c *imageWorkerClient) nextPoll(taskID string) (*CangyuanImageResult, error) {
	idx := c.pollCalls
	c.pollCalls++
	c.pollTaskIDs = append(c.pollTaskIDs, taskID)
	var result *CangyuanImageResult
	var err error
	if idx < len(c.pollResults) {
		result = c.pollResults[idx]
	}
	if idx < len(c.pollErrors) {
		err = c.pollErrors[idx]
	}
	return result, err
}
func (c *imageWorkerClient) SubmitGeneration(context.Context, CangyuanImageRequest) (*CangyuanImageResult, error) {
	return c.nextSubmit()
}
func (c *imageWorkerClient) SubmitEdit(context.Context, CangyuanImageRequest) (*CangyuanImageResult, error) {
	return c.nextSubmit()
}
func (c *imageWorkerClient) PollGeneration(_ context.Context, taskID string) (*CangyuanImageResult, error) {
	return c.nextPoll(taskID)
}
func (c *imageWorkerClient) PollEdit(_ context.Context, taskID string) (*CangyuanImageResult, error) {
	return c.nextPoll(taskID)
}

type imageWorkerProviderFactory struct {
	client           CangyuanImageClient
	requireImageOnly []bool
}

func (f *imageWorkerProviderFactory) ForAccount(_ *Account, requireImageOnly bool) (CangyuanImageClient, error) {
	f.requireImageOnly = append(f.requireImageOnly, requireImageOnly)
	return f.client, nil
}

func newImageWorkerFixture(status string) (*ImageGenerationWorker, *imageWorkerRepo, *imageWorkerPayloadStore, *imageWorkerResultStore, *imageWorkerBilling, *imageWorkerAccountSelector, *imageWorkerClient) {
	userID, apiKeyID, groupID := int64(1), int64(2), int64(3)
	payloadRef := ImageGenerationPayloadRef("imgjob_test")
	job := &ImageGenerationJob{
		JobID: "imgjob_test", UserID: &userID, APIKeyID: &apiKeyID, GroupID: &groupID,
		Status: status, Operation: ImageGenerationJobOperationGeneration,
		PublicModel: CangyuanImageModel1K, PayloadObjectRef: &payloadRef,
		EstimatedCost: 0.1, HeldCost: 0.1,
	}
	account := &Account{
		ID: 77, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Extra: map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly},
		Credentials: map[string]any{
			"api_key": "test-credential", "base_url": "https://images.example.test",
			"model_mapping": map[string]any{CangyuanImageModel1K: CangyuanImageModel1K},
		},
	}
	repo := &imageWorkerRepo{job: job}
	payloads := &imageWorkerPayloadStore{payload: &ImageGenerationPayload{Request: CangyuanImageRequest{Model: CangyuanImageModel1K, Prompt: "draw a dog", N: 1}}}
	results := &imageWorkerResultStore{}
	billing := &imageWorkerBilling{}
	accounts := &imageWorkerAccountSelector{account: account}
	client := &imageWorkerClient{}
	worker := NewImageGenerationWorker(repo, payloads, results, billing, accounts, &imageWorkerProviderFactory{client: client}, ImageGenerationWorkerOptions{LeaseDuration: time.Hour, RetryDelay: time.Millisecond})
	return worker, repo, payloads, results, billing, accounts, client
}

func TestImageGenerationWorkerSyncCompletionStoresAndSettlesOnce(t *testing.T) {
	worker, repo, payloads, results, billing, accounts, client := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	client.submitResults = []*CangyuanImageResult{{Completed: true, Data: []CangyuanImageData{{B64JSON: "unused-by-fake"}}}}

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, int64(77), *repo.job.AccountID)
	require.Equal(t, []string{"image-results/imgjob_test/0.png"}, repo.job.ResultObjectRefs)
	require.Equal(t, "1024x1024", *repo.job.ActualSize)
	require.Equal(t, 1, client.submitCalls)
	providerFactory, ok := worker.providers.(*imageWorkerProviderFactory)
	require.True(t, ok)
	require.Equal(t, []bool{true}, providerFactory.requireImageOnly)
	require.Zero(t, client.pollCalls)
	require.Equal(t, 1, results.calls)
	require.Equal(t, 1, billing.settleCalls)
	require.Equal(t, 1, accounts.releases)
	require.True(t, payloads.deleted)
}

func TestImageGenerationWorkerCreatedJobHoldsBeforeQueueing(t *testing.T) {
	worker, repo, _, results, billing, _, client := newImageWorkerFixture(ImageGenerationJobStatusCreated)
	repo.job.HeldCost = 0

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusQueued, repo.job.Status)
	require.Equal(t, repo.job.EstimatedCost, repo.job.HeldCost)
	require.Equal(t, 1, billing.holdCalls)
	require.Zero(t, client.submitCalls)
	require.Zero(t, results.calls)
}

func TestImageGenerationWorkersDoNotDoubleClaimAnActiveLease(t *testing.T) {
	_, repo, payloads, results, billing, accounts, _ := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	client := &blockingImageWorkerClient{
		imageWorkerClient: &imageWorkerClient{
			submitResults: []*CangyuanImageResult{{Status: "queued", UpstreamTaskID: "active-lease-task"}},
		},
		started: make(chan struct{}),
		resume:  make(chan struct{}),
	}
	providers := &imageWorkerProviderFactory{client: client}
	options := ImageGenerationWorkerOptions{LeaseDuration: time.Minute, RetryDelay: time.Millisecond}
	workerOne := NewImageGenerationWorker(repo, payloads, results, billing, accounts, providers, options)
	workerTwo := NewImageGenerationWorker(repo, payloads, results, billing, accounts, providers, options)

	firstDone := make(chan error, 1)
	go func() { firstDone <- workerOne.RunOnce(context.Background()) }()
	<-client.started

	// The first worker is still inside upstream submission and owns the lease.
	// A second worker must see no available job instead of submitting the same
	// request or advancing the shared claim version.
	require.ErrorIs(t, workerTwo.RunOnce(context.Background()), ErrImageGenerationWorkerIdle)
	require.Equal(t, int64(1), repo.job.ClaimVersion)
	require.Zero(t, client.submitCalls)

	close(client.resume)
	require.NoError(t, <-firstDone)
	require.Equal(t, ImageGenerationJobStatusSubmitted, repo.job.Status)
	require.Equal(t, 1, client.submitCalls)
}

func TestImageGenerationLeaseRecoveryMakesUnknownSubmissionTerminal(t *testing.T) {
	_, repo, _, _, _, _, _ := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	now := time.Now().UTC()
	job, err := repo.ClaimNextImageGenerationJob(context.Background(), now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, ImageGenerationJobStatusSubmitting, job.Status)

	recovered, err := repo.RecoverExpiredImageGenerationJobLeases(context.Background(), now.Add(2*time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, recovered, 1)
	require.Equal(t, ImageGenerationJobStatusSubmissionUnknown, repo.job.Status)
	require.Equal(t, "image_submission_unknown", *repo.job.ErrorCode)
	require.NotNil(t, repo.job.CompletedAt)

	// A recovered submitting job is never eligible for automatic resubmission.
	_, err = repo.ClaimNextImageGenerationJob(context.Background(), now.Add(2*time.Minute), time.Minute)
	require.ErrorIs(t, err, ErrImageGenerationJobNotFound)
}

func TestImageGenerationWorkerAsyncPollStaysOnOriginalAccount(t *testing.T) {
	worker, repo, _, results, billing, accounts, client := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	client.submitResults = []*CangyuanImageResult{{Status: "queued", UpstreamTaskID: "upstream-private-task"}}
	client.pollResults = []*CangyuanImageResult{
		{Status: "processing"},
		{Status: "completed", Completed: true, Data: []CangyuanImageData{{URL: "https://temporary.example/image"}}},
	}

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusSubmitted, repo.job.Status)
	require.Equal(t, int64(77), *repo.job.AccountID)
	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusPolling, repo.job.Status)
	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, []int64{77, 77}, accounts.boundIDs)
	require.Equal(t, []string{"upstream-private-task", "upstream-private-task"}, client.pollTaskIDs)
	require.Equal(t, 1, accounts.selectCalls)
	require.Equal(t, 1, results.calls)
	require.Equal(t, 1, billing.settleCalls)
	providerFactory, ok := worker.providers.(*imageWorkerProviderFactory)
	require.True(t, ok)
	require.Equal(t, []bool{true, false, false}, providerFactory.requireImageOnly)
}

func TestImageGenerationWorkerTransientPollFailureDoesNotResubmit(t *testing.T) {
	worker, repo, _, results, billing, accounts, client := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	client.submitResults = []*CangyuanImageResult{{Status: "queued", UpstreamTaskID: "bound-task"}}
	client.pollErrors = []error{&CangyuanAdapterError{
		Code: "image_upstream_unavailable", Retryable: true, Err: errors.New("temporary poll failure"),
	}}
	// The fake client's poll result/error slices are indexed by poll call,
	// therefore the failed first poll occupies index 0 as well.
	client.pollResults = []*CangyuanImageResult{nil, &CangyuanImageResult{Completed: true, Data: []CangyuanImageData{{B64JSON: "stored-result"}}}}

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusSubmitted, repo.job.Status)
	require.Equal(t, 1, client.submitCalls)

	// A retryable polling error releases the claim back to polling. The next
	// worker pass must use the persisted account/task binding and never submit
	// a second upstream generation.
	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusPolling, repo.job.Status)
	require.Equal(t, 1, client.submitCalls)
	require.Equal(t, []int64{77}, accounts.boundIDs)

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, 1, client.submitCalls)
	require.Equal(t, 2, client.pollCalls)
	require.Equal(t, 1, results.calls)
	require.Equal(t, 1, billing.settleCalls)
}

func TestImageGenerationWorkerModelMismatchFailsTerminally(t *testing.T) {
	worker, repo, _, _, billing, _, client := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	accountSelector, ok := worker.accounts.(*imageWorkerAccountSelector)
	require.True(t, ok)
	account := accountSelector.account
	account.Credentials["model_mapping"] = map[string]any{"gpt-image-2-1k": "gpt-image-1"}

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusFailed, repo.job.Status)
	require.Equal(t, "image_model_not_allowed", *repo.job.ErrorCode)
	require.Equal(t, 1, billing.releaseCalls)
	require.Zero(t, client.submitCalls)
}

func TestImageGenerationWorkerSubmissionUnknownNeverResubmitsOrReleasesHold(t *testing.T) {
	worker, repo, _, _, billing, _, client := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	client.submitErrors = []error{&CangyuanAdapterError{Code: "image_upstream_timeout", SubmissionUnknown: true, Err: errors.New("network outcome unknown")}}

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusSubmissionUnknown, repo.job.Status)
	require.Equal(t, 1, client.submitCalls)
	require.Zero(t, billing.releaseCalls)
	require.ErrorIs(t, worker.RunOnce(context.Background()), ErrImageGenerationWorkerIdle)
	require.Equal(t, 1, client.submitCalls)
}

func TestImageGenerationWorkerServerErrorSubmissionNeverResubmits(t *testing.T) {
	worker, repo, _, _, billing, _, client := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	client.submitErrors = []error{&CangyuanAdapterError{
		Code: "image_upstream_unavailable", HTTPStatus: http.StatusBadGateway,
		Retryable: true, SubmissionUnknown: true, Err: errors.New("provider outcome unknown"),
	}}

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusSubmissionUnknown, repo.job.Status)
	require.Equal(t, 1, client.submitCalls)
	require.Zero(t, billing.releaseCalls)
	require.ErrorIs(t, worker.RunOnce(context.Background()), ErrImageGenerationWorkerIdle)
	require.Equal(t, 1, client.submitCalls)
}

func TestImageGenerationWorkerStorageRetryDoesNotRegenerate(t *testing.T) {
	worker, repo, payloads, results, billing, _, client := newImageWorkerFixture(ImageGenerationJobStatusStoring)
	payloads.payload.PendingResult = &CangyuanImageResult{Completed: true, Data: []CangyuanImageData{{B64JSON: "unused-by-fake"}}}
	results.err = errors.New("storage unavailable")

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusStoring, repo.job.Status)
	require.Zero(t, client.submitCalls)
	require.Zero(t, billing.settleCalls)

	results.err = nil
	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, 2, results.calls)
	require.Zero(t, client.submitCalls)
	require.Equal(t, 1, billing.settleCalls)
}

func TestImageGenerationWorkerSettlementRetryDoesNotStoreOrRegenerate(t *testing.T) {
	worker, repo, _, results, billing, _, client := newImageWorkerFixture(ImageGenerationJobStatusSettling)
	repo.job.ResultObjectRefs = []string{"image-results/imgjob_test/0.png"}
	billing.settleErr = errors.New("billing unavailable")

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusSettling, repo.job.Status)
	require.Zero(t, results.calls)
	require.Zero(t, client.submitCalls)

	billing.settleErr = nil
	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, 2, billing.settleCalls)
	require.Zero(t, results.calls)
	require.Zero(t, client.submitCalls)
}

func TestImageGenerationWorkerRedisLossRePollsBoundTaskWithoutRegeneration(t *testing.T) {
	worker, repo, payloads, results, billing, accounts, client := newImageWorkerFixture(ImageGenerationJobStatusStoring)
	payloads.payload = nil
	repo.job.AccountID = int64Pointer(77)
	repo.job.UpstreamTaskID = stringPointer("upstream-recoverable-task")
	client.pollResults = []*CangyuanImageResult{{Completed: true, Data: []CangyuanImageData{{B64JSON: "restaged-result"}}}}

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusPolling, repo.job.Status)
	require.Zero(t, client.submitCalls)
	require.Zero(t, results.calls)

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, []int64{77}, accounts.boundIDs)
	require.Equal(t, []string{"upstream-recoverable-task"}, client.pollTaskIDs)
	require.Zero(t, client.submitCalls)
	require.Equal(t, 1, results.calls)
	require.Equal(t, 1, billing.settleCalls)
}

func TestImageGenerationWorkerLostSynchronousResultBecomesUnknown(t *testing.T) {
	worker, repo, payloads, results, billing, _, client := newImageWorkerFixture(ImageGenerationJobStatusStoring)
	payloads.payload = nil
	repo.job.AccountID = int64Pointer(77)
	repo.job.UpstreamTaskID = nil

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusSubmissionUnknown, repo.job.Status)
	require.Zero(t, client.submitCalls)
	require.Zero(t, results.calls)
	require.Zero(t, billing.releaseCalls)
}

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// imageGenerationFlowRepo combines the small worker state machine fake with
// the create/read methods used by the orchestrator. The upstream itself is an
// httptest TLS server from cangyuan_image_fake_upstream_contract_test.go, so
// this test exercises the real adapter and result store without a live
// provider, database, Redis, or production server.
type imageGenerationFlowRepo struct {
	*imageWorkerRepo
}

func (r *imageGenerationFlowRepo) CreateImageGenerationJob(_ context.Context, params CreateImageGenerationJobParams) (*ImageGenerationJob, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job != nil {
		return nil, false, ErrImageGenerationJobExists
	}
	now := time.Now()
	status := params.Status
	if status == "" {
		status = ImageGenerationJobStatusCreated
	}
	r.job = &ImageGenerationJob{
		JobID: params.JobID, UserID: params.UserID, APIKeyID: params.APIKeyID,
		GroupID: params.GroupID, SubscriptionID: params.SubscriptionID,
		BillingType: params.BillingType, Source: params.Source,
		Operation: params.Operation, Status: status, PublicModel: params.PublicModel,
		RequestedSize: params.RequestedSize, Quality: params.Quality,
		ResponseFormat: params.ResponseFormat, IdempotencyKey: params.IdempotencyKey,
		RequestHash: params.RequestHash, PromptHash: params.PromptHash,
		PayloadObjectRef: params.PayloadObjectRef, ResultObjectRefs: []string{},
		BaseCost: params.BaseCost, RateMultiplier: params.RateMultiplier,
		EstimatedCost: params.EstimatedCost, HeldCost: params.HeldCost,
		CreatedAt: now, UpdatedAt: now,
	}
	return r.job, false, nil
}

func (r *imageGenerationFlowRepo) GetImageGenerationJobForOwner(_ context.Context, userID, apiKeyID int64, jobID string) (*ImageGenerationJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job == nil || r.job.JobID != jobID || r.job.UserID == nil || r.job.APIKeyID == nil ||
		*r.job.UserID != userID || *r.job.APIKeyID != apiKeyID {
		return nil, ErrImageGenerationJobNotFound
	}
	return r.job, nil
}

func TestImageGenerationOrchestratorWorkerFakeUpstreamCompletesDurableWorkbenchJob(t *testing.T) {
	fake := newFakeCangyuanUpstream(t)
	defer fake.server.Close()

	adapter, err := NewCangyuanImageAdapter(fake.server.URL, "fake-cangyuan-key", fake.server.Client())
	require.NoError(t, err)

	userID, apiKeyID, groupID := int64(101), int64(202), int64(303)
	account := &Account{
		ID: 707, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Extra: map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly},
		Credentials: map[string]any{
			"api_key": "fake-cangyuan-key", "base_url": fake.server.URL,
			"model_mapping": map[string]any{CangyuanImageModel1K: CangyuanImageModel1K},
		},
	}
	repo := &imageGenerationFlowRepo{imageWorkerRepo: &imageWorkerRepo{}}
	payloads := &imageWorkerPayloadStore{}
	orchestrator := NewImageGenerationOrchestrator(repo, payloads, time.Hour)

	job, replayed, err := orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: userID, APIKeyID: apiKeyID, GroupID: &groupID,
		Source: ImageGenerationJobSourceWorkbench, Operation: ImageGenerationJobOperationGeneration,
		PublicModel: CangyuanImageModel1K,
		Request: CangyuanImageRequest{
			Prompt: "a small orange puppy on a clean pastel background",
			Size:   "1024x1024", N: 1, Async: true, ResponseFormat: "b64_json",
		},
		EstimatedCost: 0.1,
	})
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, ImageGenerationJobStatusCreated, job.Status)

	results := &CangyuanImageResultStore{
		storage:       &recordingImageStorage{},
		httpClient:    fake.server.Client(),
		maxBytes:      cangyuanImageOutputMaxBytes,
		hostValidator: func(context.Context, string) (bool, error) { return false, nil },
		prefix:        "workbench-results/",
	}
	billing := &imageWorkerBilling{}
	accounts := &imageWorkerAccountSelector{account: account}
	worker := NewImageGenerationWorker(
		repo, payloads, results, billing, accounts,
		&imageWorkerProviderFactory{client: adapter},
		ImageGenerationWorkerOptions{LeaseDuration: time.Hour, RetryDelay: time.Millisecond},
	)

	// created -> queued and queued -> submitted.
	require.NoError(t, worker.RunOnce(context.Background()), "worker pass %d", 1)
	require.NoError(t, worker.RunOnce(context.Background()), "worker pass %d", 2)

	// Simulate a process restart between submission and polling. The durable
	// repository/payload store and the upstream task binding are reused by a
	// fresh worker instance; the fake returns one transient 502 and must then
	// be polled again without a second generation submission.
	worker = NewImageGenerationWorker(
		repo, payloads, results, billing, accounts,
		&imageWorkerProviderFactory{client: adapter},
		ImageGenerationWorkerOptions{LeaseDuration: time.Hour, RetryDelay: time.Millisecond},
	)
	require.NoError(t, worker.RunOnce(context.Background()), "worker pass %d", 3)
	worker = NewImageGenerationWorker(
		repo, payloads, results, billing, accounts,
		&imageWorkerProviderFactory{client: adapter},
		ImageGenerationWorkerOptions{LeaseDuration: time.Hour, RetryDelay: time.Millisecond},
	)
	require.NoError(t, worker.RunOnce(context.Background()), "worker pass %d", 4)

	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, int64(707), *repo.job.AccountID)
	require.Equal(t, CangyuanImageModel1K, *repo.job.UpstreamModel)
	require.Equal(t, "generation/task-1", *repo.job.UpstreamTaskID)
	require.Len(t, repo.job.ResultObjectRefs, 1)
	require.True(t, strings.HasPrefix(repo.job.ResultObjectRefs[0], "workbench-results/"))
	require.Equal(t, "32x16", *repo.job.ActualSize)
	require.Equal(t, 1, billing.settleCalls)
	require.Equal(t, 1, accounts.selectCalls)
	require.True(t, payloads.deleted)
	require.ErrorIs(t, worker.RunOnce(context.Background()), ErrImageGenerationWorkerIdle)

	fake.mu.Lock()
	pollCount := fake.pollCounts["generation/task-1"]
	fake.mu.Unlock()
	require.Equal(t, 2, pollCount, "a transient poll error must retry the same upstream task")
	require.Len(t, fake.Calls(), 1, "polling must not submit a second generation")
	stored := results.storage.(*recordingImageStorage)
	require.Len(t, stored.data, 1)
	require.Equal(t, fake.output, stored.data[0])
}

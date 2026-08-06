//go:build image_generation_integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// This test models two application processes with independent PostgreSQL
// repository instances, encrypted payload stores, Redis clients and Worker
// runtimes. The job is inserted only after both runtimes are idle, so a short
// Redis wake-up is required to discover it; PostgreSQL remains the source of
// truth and the claim fence must still allow only one submit.
func TestDurableImageGenerationWorkerRuntimesWakeAcrossIndependentProcesses(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("IMAGE_GENERATION_TEST_DATABASE_URL"))
	redisAddress := strings.TrimSpace(os.Getenv("IMAGE_GENERATION_TEST_REDIS_URL"))
	if dsn == "" || redisAddress == "" {
		t.Skip("set IMAGE_GENERATION_TEST_DATABASE_URL and IMAGE_GENERATION_TEST_REDIS_URL to disposable PostgreSQL and Redis endpoints")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.PingContext(ctx))
	require.NoError(t, ApplyMigrations(ctx, db))

	redisOptions := imageGenerationTestRedisOptions(t, redisAddress)
	redisA := redis.NewClient(redisOptions)
	redisB := redis.NewClient(imageGenerationTestRedisOptions(t, redisAddress))
	publisher := redis.NewClient(imageGenerationTestRedisOptions(t, redisAddress))
	t.Cleanup(func() {
		_ = redisA.Close()
		_ = redisB.Close()
		_ = publisher.Close()
	})
	require.NoError(t, redisA.Ping(ctx).Err())

	jobID := fmt.Sprintf("imgjob_multi_process_wakeup_%d", time.Now().UnixNano())
	payloadRef := service.ImageGenerationPayloadRef(jobID)
	accountName := fmt.Sprintf("multi-process-wakeup-account-%d", time.Now().UnixNano())
	var accountID int64
	require.NoError(t, db.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, credentials, extra, status)
VALUES ($1, 'openai', 'apikey', '{}'::jsonb, '{}'::jsonb, 'active')
RETURNING id`, accountName).Scan(&accountID))
	defer func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM image_generation_payloads WHERE payload_ref = $1", payloadRef)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM image_generation_jobs WHERE job_id = $1", jobID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", accountID)
	}()

	encryptor := &AESEncryptor{key: []byte("0123456789abcdef0123456789abcdef")}
	request := service.CangyuanImageRequest{
		Model:          service.CangyuanImageModel1K,
		Prompt:         "multi-process Redis wake-up integration marker",
		Size:           "1024x1024",
		N:              1,
		Async:          true,
		ResponseFormat: "b64_json",
	}
	payloadStore := NewDurableImageGenerationPayloadStore(db, redisA, encryptor)
	require.NoError(t, payloadStore.Save(ctx, payloadRef, &service.ImageGenerationPayload{Request: request}, time.Hour))

	fake := newDurableWorkerUpstream(t)
	defer fake.server.Close()
	adapter, err := service.NewCangyuanImageAdapter(fake.server.URL, "durable-worker-key", fake.server.Client())
	require.NoError(t, err)
	account := &service.Account{
		ID:       accountID,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "durable-worker-key",
			"base_url": fake.server.URL,
			"model_mapping": map[string]any{
				service.CangyuanImageModel1K: service.CangyuanImageModel1K,
			},
		},
		Extra: map[string]any{service.AccountPurposeExtraKey: service.AccountPurposeImageOnly},
	}
	selector := &durableWorkerAccountSelector{account: account}
	providers := &durableWorkerProviderFactory{client: adapter}
	billing := &durableWorkerBilling{}
	results := &durableWorkerResultStore{}

	newRuntime := func(client *redis.Client, claims *atomic.Int32) *service.ImageGenerationWorkerRuntime {
		repo := &countingImageGenerationJobRepository{
			ImageGenerationJobRepository: NewImageGenerationJobRepository(db),
			claims:                       claims,
		}
		worker := service.NewImageGenerationWorker(
			repo,
			NewDurableImageGenerationPayloadStore(db, client, encryptor),
			results, billing, selector, providers,
			service.ImageGenerationWorkerOptions{
				LeaseDuration:     time.Minute,
				PollInterval:      time.Millisecond,
				RetryDelay:        time.Millisecond,
				IdleDelay:         time.Hour,
				RecoveryInterval:  time.Hour,
				PayloadTTL:        time.Hour,
				MaxSubmitAttempts: 2,
			},
		)
		return service.NewImageGenerationWorkerRuntime(worker, NewImageGenerationWakeup(client))
	}

	var claimsA, claimsB atomic.Int32
	runtimeA := newRuntime(redisA, &claimsA)
	runtimeB := newRuntime(redisB, &claimsB)
	runtimeA.Start()
	runtimeB.Start()
	t.Cleanup(func() {
		runtimeA.Stop()
		runtimeB.Stop()
	})
	require.Eventually(t, func() bool { return claimsA.Load() > 0 && claimsB.Load() > 0 }, 5*time.Second, 10*time.Millisecond,
		"both independent runtimes did not reach the idle PostgreSQL scan")

	requestHash := "multi-process-wakeup-request-hash"
	job, replayed, err := NewImageGenerationJobRepository(db).CreateImageGenerationJob(ctx, service.CreateImageGenerationJobParams{
		JobID:            jobID,
		Source:           service.ImageGenerationJobSourceWorkbench,
		Operation:        service.ImageGenerationJobOperationGeneration,
		Status:           service.ImageGenerationJobStatusCreated,
		PublicModel:      service.CangyuanImageModel1K,
		RequestHash:      &requestHash,
		PromptHash:       "multi-process-wakeup-prompt-hash",
		PayloadObjectRef: &payloadRef,
		RateMultiplier:   1,
		EstimatedCost:    0.1,
	})
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, jobID, job.JobID)

	// Publish repeatedly only to tolerate the small Redis subscription
	// handshake window. The job did not exist during the idle scans, so it can
	// complete only after a wake-up causes a fresh claim attempt.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, NewImageGenerationWakeup(publisher).PublishImageGenerationWakeup(ctx, jobID))
		current, getErr := NewImageGenerationJobRepository(db).GetImageGenerationJob(ctx, jobID)
		require.NoError(t, getErr)
		if current.Status == service.ImageGenerationJobStatusCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	runtimeA.Stop()
	runtimeB.Stop()
	finalJob, err := NewImageGenerationJobRepository(db).GetImageGenerationJob(ctx, jobID)
	require.NoError(t, err)
	require.Equal(t, service.ImageGenerationJobStatusCompleted, finalJob.Status)
	require.Equal(t, accountID, *finalJob.AccountID)
	require.Equal(t, 1, fake.submitCalls)
	require.Equal(t, 1, billing.holdCalls)
	require.Equal(t, 1, billing.settleCalls)
	require.Equal(t, 1, results.calls)
	require.Equal(t, 1, selector.selectCalls)
	_, err = NewDurableImageGenerationPayloadStore(db, nil, encryptor).Get(ctx, payloadRef)
	require.ErrorIs(t, err, service.ErrImageGenerationPayloadNotFound)
}

type countingImageGenerationJobRepository struct {
	service.ImageGenerationJobRepository
	claims *atomic.Int32
}

func (r *countingImageGenerationJobRepository) ClaimNextImageGenerationJob(ctx context.Context, now time.Time, leaseDuration time.Duration) (*service.ImageGenerationJob, error) {
	r.claims.Add(1)
	return r.ImageGenerationJobRepository.ClaimNextImageGenerationJob(ctx, now, leaseDuration)
}

func (r *countingImageGenerationJobRepository) RecoverExpiredImageGenerationJobLeases(ctx context.Context, now time.Time, limit int) (int64, error) {
	return r.ImageGenerationJobRepository.RecoverExpiredImageGenerationJobLeases(ctx, now, limit)
}

func imageGenerationTestRedisOptions(t *testing.T, address string) *redis.Options {
	t.Helper()
	if strings.Contains(address, "://") {
		options, err := redis.ParseURL(address)
		require.NoError(t, err)
		return options
	}
	return &redis.Options{Addr: address}
}

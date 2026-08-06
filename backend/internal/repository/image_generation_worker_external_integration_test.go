//go:build image_generation_integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// This opt-in test joins the two durable boundaries that are otherwise easy to
// test separately: the encrypted PostgreSQL payload store and the PostgreSQL
// image-job worker. The upstream is an httptest server, but the production
// adapter, job repository, payload store, claim fencing, result staging, and
// process-restart handoff are all exercised together.
func TestDurableImageGenerationPayloadWorkerProcessRestartDoesNotResubmit(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("IMAGE_GENERATION_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set IMAGE_GENERATION_TEST_DATABASE_URL to a disposable PostgreSQL DSN")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.PingContext(ctx))
	require.NoError(t, ApplyMigrations(ctx, db))

	jobID := fmt.Sprintf("imgjob_durable_flow_%d", time.Now().UnixNano())
	payloadRef := service.ImageGenerationPayloadRef(jobID)
	accountName := fmt.Sprintf("durable-worker-account-%d", time.Now().UnixNano())
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
	// A durable store in production still receives the Redis client for
	// rolling-upgrade fallback and best-effort wake-up cleanup. Point it at a
	// deliberately unreachable local port to exercise the Redis-loss path while
	// proving that new payload reads/writes use PostgreSQL only.
	redisUnavailable := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  20 * time.Millisecond,
		ReadTimeout:  20 * time.Millisecond,
		WriteTimeout: 20 * time.Millisecond,
	})
	defer redisUnavailable.Close()
	initialPayloadStore := NewDurableImageGenerationPayloadStore(db, redisUnavailable, encryptor)
	request := service.CangyuanImageRequest{
		Model:          service.CangyuanImageModel1K,
		Prompt:         "durable worker process restart integration marker",
		Size:           "1024x1024",
		N:              1,
		Async:          true,
		ResponseFormat: "b64_json",
	}
	require.NoError(t, initialPayloadStore.Save(ctx, payloadRef, &service.ImageGenerationPayload{Request: request}, time.Hour))

	requestHash := "durable-worker-request-hash"
	job, replayed, err := NewImageGenerationJobRepository(db).CreateImageGenerationJob(ctx, service.CreateImageGenerationJobParams{
		JobID:            jobID,
		Source:           service.ImageGenerationJobSourceWorkbench,
		Operation:        service.ImageGenerationJobOperationGeneration,
		Status:           service.ImageGenerationJobStatusCreated,
		PublicModel:      service.CangyuanImageModel1K,
		RequestHash:      &requestHash,
		PromptHash:       "durable-worker-prompt-hash",
		PayloadObjectRef: &payloadRef,
		RateMultiplier:   1,
		EstimatedCost:    0.1,
	})
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, jobID, job.JobID)

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

	newWorker := func() *service.ImageGenerationWorker {
		// Fresh repository and payload-store instances model a process handoff.
		// Neither instance has an in-memory copy of the prompt or result.
		return service.NewImageGenerationWorker(
			NewImageGenerationJobRepository(db),
			NewDurableImageGenerationPayloadStore(db, redisUnavailable, encryptor),
			results, billing, selector, providers,
			service.ImageGenerationWorkerOptions{
				LeaseDuration:     time.Minute,
				PollInterval:      time.Nanosecond,
				RetryDelay:        time.Nanosecond,
				PayloadTTL:        time.Hour,
				MaxSubmitAttempts: 2,
			},
		)
	}

	// Prepare the job once, then make two independent worker instances race for
	// the submission lease. The fake upstream blocks the winner while the loser
	// attempts to claim the same row, which proves the database fence is applied
	// around the real Worker rather than only in a repository unit test.
	worker := newWorker()
	require.NoError(t, worker.RunOnce(ctx)) // created -> queued
	fake.submitStarted = make(chan struct{})
	fake.releaseSubmit = make(chan struct{})
	submissionDone := make(chan error, 1)
	go func() { submissionDone <- newWorker().RunOnce(ctx) }()
	select {
	case <-fake.submitStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the first worker to own the submission lease")
	}
	loserErr := newWorker().RunOnce(ctx)
	require.ErrorIs(t, loserErr, service.ErrImageGenerationWorkerIdle)
	close(fake.releaseSubmit)
	require.NoError(t, <-submissionDone)
	require.Equal(t, 1, fake.submitCalls)

	// A new process polls the already-bound upstream task. The first poll is a
	// transient 502; the next process polls the same task and completes it.
	worker = newWorker()
	require.NoError(t, worker.RunOnce(ctx)) // submitted -> polling after retry
	worker = newWorker()
	require.NoError(t, worker.RunOnce(ctx)) // polling -> storing -> settling -> completed

	finalJob, err := NewImageGenerationJobRepository(db).GetImageGenerationJob(ctx, jobID)
	require.NoError(t, err)
	require.Equal(t, service.ImageGenerationJobStatusCompleted, finalJob.Status)
	require.Equal(t, accountID, *finalJob.AccountID)
	require.Equal(t, "durable/generation-1", *finalJob.UpstreamTaskID)
	require.Equal(t, []string{"durable-results/" + jobID}, finalJob.ResultObjectRefs)
	require.Equal(t, 1, billing.holdCalls)
	require.Equal(t, 1, billing.settleCalls)
	require.Equal(t, 1, results.calls)
	require.Equal(t, 1, fake.submitCalls)
	require.Equal(t, 2, fake.pollCalls)
	require.Equal(t, 1, selector.selectCalls)
	require.Equal(t, 2, selector.boundCalls)
	require.Equal(t, []bool{true, false, false}, providers.requireImageOnly)

	// Use a fresh PostgreSQL-only reader for the terminal assertion so the
	// expected not-found result is independent of the unavailable fallback.
	_, err = NewDurableImageGenerationPayloadStore(db, nil, encryptor).Get(ctx, payloadRef)
	require.ErrorIs(t, err, service.ErrImageGenerationPayloadNotFound)
}

type durableWorkerUpstream struct {
	server        *httptest.Server
	mu            sync.Mutex
	submitCalls   int
	pollCalls     int
	submitStarted chan struct{}
	releaseSubmit chan struct{}
	submitStart   sync.Once
}

func newDurableWorkerUpstream(t *testing.T) *durableWorkerUpstream {
	t.Helper()
	fake := &durableWorkerUpstream{}
	fake.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer durable-worker-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"type":"authentication_error"}}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/v1/images/generations" {
			var request service.CangyuanImageRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Model != service.CangyuanImageModel1K {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error"}}`))
				return
			}
			if fake.submitStarted != nil {
				fake.submitStart.Do(func() { close(fake.submitStarted) })
				<-fake.releaseSubmit
			}
			fake.mu.Lock()
			fake.submitCalls++
			fake.mu.Unlock()
			_, _ = w.Write([]byte(`{"task_id":"durable/generation-1","status":"queued"}`))
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/images/generations/") {
			fake.mu.Lock()
			fake.pollCalls++
			poll := fake.pollCalls
			fake.mu.Unlock()
			if poll == 1 {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"error":{"type":"upstream_error"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"task_id":"durable/generation-1","status":"completed","data":[{"b64_json":"aGVsbG8="}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	return fake
}

type durableWorkerAccountSelector struct {
	account     *service.Account
	selectCalls int
	boundCalls  int
}

func (s *durableWorkerAccountSelector) Select(context.Context, *service.ImageGenerationJob) (*service.ImageGenerationAccountLease, error) {
	s.selectCalls++
	return &service.ImageGenerationAccountLease{Account: s.account, ImageOnly: true}, nil
}

func (s *durableWorkerAccountSelector) GetBoundAccount(context.Context, int64) (*service.Account, error) {
	s.boundCalls++
	return s.account, nil
}

type durableWorkerProviderFactory struct {
	client           service.CangyuanImageClient
	requireImageOnly []bool
}

func (f *durableWorkerProviderFactory) ForAccount(_ *service.Account, requireImageOnly bool) (service.CangyuanImageClient, error) {
	f.requireImageOnly = append(f.requireImageOnly, requireImageOnly)
	return f.client, nil
}

type durableWorkerResultStore struct{ calls int }

func (s *durableWorkerResultStore) Store(_ context.Context, jobID string, data []service.CangyuanImageData) ([]string, string, error) {
	s.calls++
	if len(data) != 1 || data[0].B64JSON != "aGVsbG8=" {
		return nil, "", errors.New("unexpected staged image result")
	}
	return []string{"durable-results/" + jobID}, "1x1", nil
}

type durableWorkerBilling struct {
	holdCalls   int
	settleCalls int
}

func (b *durableWorkerBilling) Hold(context.Context, *service.ImageGenerationJob) error {
	b.holdCalls++
	return nil
}

func (b *durableWorkerBilling) Release(context.Context, *service.ImageGenerationJob) error {
	return nil
}

func (b *durableWorkerBilling) Settle(context.Context, *service.ImageGenerationJob) (float64, error) {
	b.settleCalls++
	return 0.1, nil
}

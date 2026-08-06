package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type runtimeIdleRepo struct {
	ImageGenerationJobRepository
	claims atomic.Int32
}

func (r *runtimeIdleRepo) ClaimNextImageGenerationJob(context.Context, time.Time, time.Duration) (*ImageGenerationJob, error) {
	r.claims.Add(1)
	return nil, ErrImageGenerationJobNotFound
}

func (r *runtimeIdleRepo) RecoverExpiredImageGenerationJobLeases(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

type runtimeWakeupStub struct {
	mu       sync.Mutex
	handler  func(string)
	ready    chan struct{}
	readyOne sync.Once
}

func newRuntimeWakeupStub() *runtimeWakeupStub {
	return &runtimeWakeupStub{ready: make(chan struct{})}
}

func (s *runtimeWakeupStub) PublishImageGenerationWakeup(_ context.Context, jobID string) error {
	s.mu.Lock()
	handler := s.handler
	s.mu.Unlock()
	if handler != nil {
		handler(jobID)
	}
	return nil
}

func (s *runtimeWakeupStub) SubscribeImageGenerationWakeups(ctx context.Context, handler func(string)) error {
	s.mu.Lock()
	s.handler = handler
	s.readyOne.Do(func() { close(s.ready) })
	s.mu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

func TestProvideImageGenerationWorkerRuntimeDefaultsOffAndStopsCleanly(t *testing.T) {
	worker := &ImageGenerationWorker{opts: normalizeImageGenerationWorkerOptions(ImageGenerationWorkerOptions{RetryDelay: time.Millisecond, IdleDelay: time.Millisecond})}
	runtime := ProvideImageGenerationWorkerRuntime(worker, nil, &config.Config{})
	require.False(t, runtime.Running())

	cfg := &config.Config{DedicatedImage: config.DedicatedImageConfig{Enabled: true, WorkerEnabled: true}}
	runtime = ProvideImageGenerationWorkerRuntime(worker, nil, cfg)
	require.True(t, runtime.Running())
	runtime.Stop()
	require.False(t, runtime.Running())
}

func TestImageGenerationWorkerSleepCanBeInterruptedByWakeup(t *testing.T) {
	wake := make(chan struct{}, 1)
	wake <- struct{}{}
	started := time.Now()
	sleepOrWake(context.Background(), time.Hour, wake)
	require.Less(t, time.Since(started), time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started = time.Now()
	sleepOrWake(ctx, time.Hour, wake)
	require.Less(t, time.Since(started), time.Second)
}

func TestImageGenerationWorkerRuntimeWakeupTriggersImmediateRescan(t *testing.T) {
	repo := &runtimeIdleRepo{}
	wakeup := newRuntimeWakeupStub()
	worker := NewImageGenerationWorker(
		repo,
		&imageWorkerPayloadStore{},
		&imageWorkerResultStore{},
		&imageWorkerBilling{},
		&imageWorkerAccountSelector{},
		&imageWorkerProviderFactory{},
		ImageGenerationWorkerOptions{IdleDelay: time.Hour, RetryDelay: time.Hour, RecoveryInterval: time.Hour},
	)
	runtime := NewImageGenerationWorkerRuntime(worker, wakeup)
	runtime.Start()
	t.Cleanup(runtime.Stop)

	select {
	case <-wakeup.ready:
	case <-time.After(time.Second):
		t.Fatal("worker wakeup subscriber did not start")
	}
	require.Eventually(t, func() bool { return repo.claims.Load() >= 1 }, time.Second, 5*time.Millisecond)
	firstClaims := repo.claims.Load()
	wakeup.PublishImageGenerationWakeup(context.Background(), "imgjob_runtime_wakeup")
	require.Eventually(t, func() bool { return repo.claims.Load() > firstClaims }, time.Second, 5*time.Millisecond)
}

func TestImageGenerationWorkerRuntimeNormalizesZeroValueWorkerOptions(t *testing.T) {
	runtime := NewImageGenerationWorkerRuntime(&ImageGenerationWorker{})
	runtime.Start()
	require.True(t, runtime.Running())
	runtime.Stop()
	require.False(t, runtime.Running())
}

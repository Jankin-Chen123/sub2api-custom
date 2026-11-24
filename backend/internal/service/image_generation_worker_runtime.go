package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

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

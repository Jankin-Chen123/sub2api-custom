package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ImageGenerationCleanupService removes only reconciled terminal image jobs
// after their retention period. submission_unknown jobs are deliberately
// excluded because their billing hold requires an explicit operator decision.
type ImageGenerationCleanupService struct {
	repo      ImageGenerationJobRepository
	payloads  ImageGenerationPayloadStore
	results   ImageGenerationResultReader
	deleter   ImageStorageDeleter
	retention time.Duration
	interval  time.Duration
	batchSize int

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewImageGenerationCleanupService(
	repo ImageGenerationJobRepository,
	payloads ImageGenerationPayloadStore,
	results ImageGenerationResultReader,
	deleter ImageStorageDeleter,
	retention, interval time.Duration,
	batchSize int,
) *ImageGenerationCleanupService {
	if retention < 0 {
		retention = 0
	}
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	if batchSize <= 0 || batchSize > 1000 {
		batchSize = 100
	}
	return &ImageGenerationCleanupService{
		repo: repo, payloads: payloads, results: results, deleter: deleter,
		retention: retention, interval: interval, batchSize: batchSize,
	}
}

func (s *ImageGenerationCleanupService) Start() {
	if s == nil || s.repo == nil || s.retention <= 0 || s.interval <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	done := make(chan struct{})
	s.done = done
	go func() {
		defer close(done)
		// A cleanup pass shortly after startup prevents a long-lived process
		// from waiting for the first full interval after an upgrade.
		_, _ = s.RunOnce(ctx, time.Now())
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				_, _ = s.RunOnce(ctx, now)
			}
		}
	}()
}

func (s *ImageGenerationCleanupService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel, s.done = nil, nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *ImageGenerationCleanupService) RunOnce(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("image generation cleanup is not configured")
	}
	if s.retention <= 0 {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now()
	}
	if cleaner, ok := s.payloads.(ImageGenerationPayloadExpiryCleaner); ok {
		if _, err := cleaner.PurgeExpired(ctx, now, s.batchSize); err != nil {
			return 0, err
		}
	}
	jobs, err := s.repo.ListImageGenerationJobsForCleanup(ctx, now.Add(-s.retention), s.batchSize)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, job := range jobs {
		if job == nil || job.JobID == "" {
			continue
		}
		// Keep the guard here as well as in the repository query. This protects
		// cleanup if a legacy repository implementation returns all terminal
		// states, and preserves submission_unknown rows for reconciliation.
		if job.Status == ImageGenerationJobStatusSubmissionUnknown {
			continue
		}
		// A configured result object must be deletable before its row is
		// removed. If the storage adapter has no deletion capability, retain
		// the row and emit no destructive action.
		if len(job.ResultObjectRefs) > 0 {
			if s.deleter == nil {
				continue
			}
			failed := false
			for _, ref := range job.ResultObjectRefs {
				if ref == "" {
					continue
				}
				if err := s.deleter.Delete(ctx, ref); err != nil {
					failed = true
					break
				}
			}
			if failed {
				continue
			}
		}
		if s.payloads != nil && job.PayloadObjectRef != nil {
			if err := s.payloads.Delete(ctx, *job.PayloadObjectRef); err != nil {
				// Keep the row so the next cleanup pass can retry deleting the
				// encrypted temporary payload instead of orphaning it forever.
				continue
			}
		}
		if err := s.repo.DeleteImageGenerationJob(ctx, job.JobID); err != nil {
			if !errors.Is(err, ErrImageGenerationJobNotFound) {
				return deleted, err
			}
			continue
		}
		deleted++
	}
	return deleted, nil
}

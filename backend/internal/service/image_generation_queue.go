package service

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	imageGenerationQueueDefaultMaxActive = 1
	imageGenerationQueueDefaultMaxQueued = 100
	imageGenerationQueueMaxActiveLimit   = 1000
	imageGenerationQueueMaxQueuedLimit   = 100000
	imageGenerationServerDimensionName   = "server"
	imageGenerationQueueSettingsCacheTTL = 5 * time.Second
)

var (
	// ErrImageGenerationQueueFull is returned before a durable job is created.
	// The caller can safely retry later without being charged or creating a
	// dangling payload.
	ErrImageGenerationQueueFull = infraerrors.New(
		http.StatusTooManyRequests,
		"IMAGE_QUEUE_FULL",
		"image generation queue is full",
	)
	ErrImageGenerationQueueUnavailable = infraerrors.New(
		http.StatusServiceUnavailable,
		"IMAGE_QUEUE_UNAVAILABLE",
		"image generation queue is temporarily unavailable",
	)
)

// ImageGenerationQueueSettings controls the server-wide image execution
// guard. The settings are read from the database at runtime so an administrator
// can change them without restarting the application.
type ImageGenerationQueueSettings struct {
	Enabled   bool
	MaxActive int
	MaxQueued int
}

func defaultImageGenerationQueueSettings() ImageGenerationQueueSettings {
	return ImageGenerationQueueSettings{
		Enabled:   true,
		MaxActive: imageGenerationQueueDefaultMaxActive,
		MaxQueued: imageGenerationQueueDefaultMaxQueued,
	}
}

func normalizeImageGenerationMaxActiveJobs(value int) int {
	if value < 1 {
		return imageGenerationQueueDefaultMaxActive
	}
	if value > imageGenerationQueueMaxActiveLimit {
		return imageGenerationQueueMaxActiveLimit
	}
	return value
}

func normalizeImageGenerationMaxQueuedJobs(value int) int {
	if value < 0 {
		return 0
	}
	if value > imageGenerationQueueMaxQueuedLimit {
		return imageGenerationQueueMaxQueuedLimit
	}
	return value
}

func parsePositiveSetting(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func parseNonNegativeSetting(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

// GetImageGenerationQueueSettings reads only the three queue settings. Missing
// rows intentionally fall back to safe defaults so older installations migrate
// without a restart or a manual database edit.
func (s *SettingService) GetImageGenerationQueueSettings(ctx context.Context) (ImageGenerationQueueSettings, error) {
	settings := defaultImageGenerationQueueSettings()
	if s == nil || s.settingRepo == nil {
		return settings, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyImageGenerationQueueEnabled,
		SettingKeyImageGenerationMaxActiveJobs,
		SettingKeyImageGenerationMaxQueuedJobs,
	})
	if err != nil {
		return settings, fmt.Errorf("get image generation queue settings: %w", err)
	}
	if raw, ok := values[SettingKeyImageGenerationQueueEnabled]; ok {
		settings.Enabled = !strings.EqualFold(strings.TrimSpace(raw), "false")
	}
	if raw, ok := values[SettingKeyImageGenerationMaxActiveJobs]; ok {
		if value, parseErr := strconv.Atoi(strings.TrimSpace(raw)); parseErr == nil {
			settings.MaxActive = normalizeImageGenerationMaxActiveJobs(value)
		}
	}
	if raw, ok := values[SettingKeyImageGenerationMaxQueuedJobs]; ok {
		if value, parseErr := strconv.Atoi(strings.TrimSpace(raw)); parseErr == nil {
			settings.MaxQueued = normalizeImageGenerationMaxQueuedJobs(value)
		}
	}
	return settings, nil
}

// ImageGenerationQueueCounter is intentionally optional. Keeping it separate
// from ImageGenerationJobRepository means existing test doubles and external
// repository implementations do not need to grow a method just to use the
// worker's execution slots.
type ImageGenerationQueueCounter interface {
	CountQueuedImageGenerationJobs(ctx context.Context) (int, error)
}

// ImageGenerationQueueAdmission lets the durable repository perform the
// queue-count check and job insert under one database transaction. The
// optional interface keeps older repository/test doubles source-compatible;
// production PostgreSQL repositories implement it.
type ImageGenerationQueueAdmission interface {
	CreateImageGenerationJobWithQueueLimit(ctx context.Context, params CreateImageGenerationJobParams, maxQueued int) (*ImageGenerationJob, bool, error)
}

// ImageGenerationQueueController owns the global server-level Redis lease and
// the persistent queue admission check. It is shared by the orchestrator and
// every worker instance in the process.
type ImageGenerationQueueController struct {
	repo        ImageGenerationJobRepository
	settings    *SettingService
	concurrency *ConcurrencyService
	settingsMu  sync.RWMutex
	settingsAt  time.Time
	cached      ImageGenerationQueueSettings
}

func NewImageGenerationQueueController(
	repo ImageGenerationJobRepository,
	settings *SettingService,
	concurrency *ConcurrencyService,
) *ImageGenerationQueueController {
	return &ImageGenerationQueueController{repo: repo, settings: settings, concurrency: concurrency}
}

func (q *ImageGenerationQueueController) currentSettings(ctx context.Context) (ImageGenerationQueueSettings, error) {
	if q == nil {
		return defaultImageGenerationQueueSettings(), nil
	}
	if q.settings == nil {
		return defaultImageGenerationQueueSettings(), nil
	}
	now := time.Now()
	q.settingsMu.RLock()
	cached, cachedAt := q.cached, q.settingsAt
	q.settingsMu.RUnlock()
	if !cachedAt.IsZero() && now.Sub(cachedAt) < imageGenerationQueueSettingsCacheTTL {
		return cached, nil
	}

	q.settingsMu.Lock()
	defer q.settingsMu.Unlock()
	if !q.settingsAt.IsZero() && now.Sub(q.settingsAt) < imageGenerationQueueSettingsCacheTTL {
		return q.cached, nil
	}
	loaded, err := q.settings.GetImageGenerationQueueSettings(ctx)
	if err != nil {
		return defaultImageGenerationQueueSettings(), err
	}
	q.cached = loaded
	q.settingsAt = now
	return loaded, nil
}

// Settings returns the cached runtime queue settings to callers that need to
// coordinate durable admission with job creation.
func (q *ImageGenerationQueueController) Settings(ctx context.Context) (ImageGenerationQueueSettings, error) {
	return q.currentSettings(ctx)
}

func imageGenerationServerDimension(maxActive int) []ImageConcurrencyDimension {
	return []ImageConcurrencyDimension{{
		Name: imageGenerationServerDimensionName,
		ID:   1,
		Max:  normalizeImageGenerationMaxActiveJobs(maxActive),
	}}
}

// CanEnqueue checks the durable waiting queue before creating a job. A value of
// zero means no additional waiting jobs are admitted when another queued job
// already exists; it does not make the endpoint unusable for the first job.
func (q *ImageGenerationQueueController) CanEnqueue(ctx context.Context) (bool, error) {
	if q == nil {
		return true, nil
	}
	settings, err := q.currentSettings(ctx)
	if err != nil {
		return false, err
	}
	if !settings.Enabled || q.repo == nil {
		return true, nil
	}
	counter, ok := q.repo.(ImageGenerationQueueCounter)
	if !ok {
		return true, nil
	}
	queued, err := counter.CountQueuedImageGenerationJobs(ctx)
	if err != nil {
		return false, err
	}
	if settings.MaxQueued == 0 {
		return queued == 0, nil
	}
	return queued < settings.MaxQueued, nil
}

// Acquire reserves the server slot for the full upstream task lifecycle. It is
// idempotent for the same job ID, which also makes retries safe.
func (q *ImageGenerationQueueController) Acquire(ctx context.Context, jobID string) (bool, error) {
	if q == nil || strings.TrimSpace(jobID) == "" {
		return true, nil
	}
	settings, err := q.currentSettings(ctx)
	if err != nil {
		return false, err
	}
	if !settings.Enabled {
		return true, nil
	}
	if q.concurrency == nil {
		return false, ErrImageGenerationQueueUnavailable
	}
	return q.concurrency.acquireImageSlotSet(ctx, imageGenerationServerDimension(settings.MaxActive), jobID)
}

// Renew refreshes the Redis lease while a task is being polled or stored. If
// the administrator disables the guard, the old lease is released immediately
// so the change takes effect without waiting for the Redis TTL.
func (q *ImageGenerationQueueController) Renew(ctx context.Context, jobID string) (bool, error) {
	if q == nil || strings.TrimSpace(jobID) == "" {
		return true, nil
	}
	settings, err := q.currentSettings(ctx)
	if err != nil {
		return false, err
	}
	if !settings.Enabled {
		return true, q.Release(ctx, jobID)
	}
	if q.concurrency == nil {
		return false, ErrImageGenerationQueueUnavailable
	}
	return q.concurrency.acquireImageSlotSet(ctx, imageGenerationServerDimension(settings.MaxActive), jobID)
}

func (q *ImageGenerationQueueController) Release(ctx context.Context, jobID string) error {
	if q == nil || q.concurrency == nil || strings.TrimSpace(jobID) == "" {
		return nil
	}
	return q.concurrency.releaseImageSlotSet(ctx, imageGenerationServerDimension(imageGenerationQueueDefaultMaxActive), jobID)
}

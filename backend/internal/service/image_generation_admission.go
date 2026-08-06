package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ImageConcurrencyDimensionUser    = "user"
	ImageConcurrencyDimensionAPIKey  = "api_key"
	ImageConcurrencyDimensionGroup   = "group"
	ImageConcurrencyDimensionAccount = "account"
	ImageConcurrencyTier4K           = "4k"
)

// ImageConcurrencyDimension is one independently limited owner of an image
// request. The repository acquires all dimensions in one Redis Lua operation,
// so a failed group/account/4K check cannot leave a half-acquired request.
type ImageConcurrencyDimension struct {
	Name string
	ID   int64
	Max  int
}

type ImageGenerationAdmissionConfig struct {
	Enabled            bool
	MaxPerUser         int
	MaxPerAPIKey       int
	MaxPerGroup        int
	MaxPerAccount      int
	Max4KConcurrent    int
	OverflowMode       string
	WaitTimeout        time.Duration
	MaxWaitingRequests int
}

type ImageGenerationAdmissionRequest struct {
	UserID    int64
	APIKeyID  int64
	GroupID   int64
	AccountID int64
	Tier      string
}

var ErrImageGenerationAdmissionUnavailable = errors.New("image concurrency admission is unavailable")

// ImageGenerationAdmission is deliberately separate from normal text
// concurrency. It is only called by image entry points and therefore cannot
// route a normal chat/code request through image-specific limits.
type ImageGenerationAdmission struct {
	concurrency *ConcurrencyService
	config      ImageGenerationAdmissionConfig

	waitMu  sync.Mutex
	waiting int
}

func NewImageGenerationAdmission(concurrency *ConcurrencyService, cfg ImageGenerationAdmissionConfig) *ImageGenerationAdmission {
	return &ImageGenerationAdmission{concurrency: concurrency, config: normalizeImageGenerationAdmissionConfig(cfg)}
}

func normalizeImageGenerationAdmissionConfig(cfg ImageGenerationAdmissionConfig) ImageGenerationAdmissionConfig {
	if cfg.WaitTimeout <= 0 {
		cfg.WaitTimeout = 30 * time.Second
	}
	if strings.EqualFold(strings.TrimSpace(cfg.OverflowMode), "wait") {
		cfg.OverflowMode = "wait"
	} else {
		cfg.OverflowMode = "reject"
	}
	return cfg
}

func (a *ImageGenerationAdmission) Acquire(ctx context.Context, req ImageGenerationAdmissionRequest) (func(), bool, error) {
	if a == nil || !a.config.Enabled {
		return nil, true, nil
	}
	dimensions := a.dimensions(req)
	if len(dimensions) == 0 {
		return nil, true, nil
	}
	if a.concurrency == nil {
		return nil, false, ErrImageGenerationAdmissionUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	wait := a.config.OverflowMode == "wait"
	if wait {
		if a.config.MaxWaitingRequests > 0 {
			a.waitMu.Lock()
			if a.waiting >= a.config.MaxWaitingRequests {
				a.waitMu.Unlock()
				return nil, false, nil
			}
			a.waiting++
			a.waitMu.Unlock()
			defer func() {
				a.waitMu.Lock()
				if a.waiting > 0 {
					a.waiting--
				}
				a.waitMu.Unlock()
			}()
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.config.WaitTimeout)
		defer cancel()
	}

	requestID := generateRequestID()
	for {
		acquired, err := a.concurrency.acquireImageSlotSet(ctx, dimensions, requestID)
		if err != nil {
			return nil, false, ErrImageGenerationAdmissionUnavailable
		}
		if acquired {
			var releaseOnce sync.Once
			return func() {
				releaseOnce.Do(func() {
					releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := a.concurrency.releaseImageSlotSet(releaseCtx, dimensions, requestID); err != nil {
						// Release is best effort; Redis TTL remains the crash-safety net.
						return
					}
				})
			}, true, nil
		}
		if !wait {
			return nil, false, nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, false, nil
		case <-timer.C:
		}
	}
}

func (a *ImageGenerationAdmission) dimensions(req ImageGenerationAdmissionRequest) []ImageConcurrencyDimension {
	if a == nil {
		return nil
	}
	dimensions := make([]ImageConcurrencyDimension, 0, 5)
	appendDimension := func(name string, id int64, max int) {
		if id > 0 && max > 0 {
			dimensions = append(dimensions, ImageConcurrencyDimension{Name: name, ID: id, Max: max})
		}
	}
	appendDimension(ImageConcurrencyDimensionUser, req.UserID, a.config.MaxPerUser)
	appendDimension(ImageConcurrencyDimensionAPIKey, req.APIKeyID, a.config.MaxPerAPIKey)
	appendDimension(ImageConcurrencyDimensionGroup, req.GroupID, a.config.MaxPerGroup)
	appendDimension(ImageConcurrencyDimensionAccount, req.AccountID, a.config.MaxPerAccount)
	if strings.EqualFold(strings.TrimSpace(req.Tier), ImageConcurrencyTier4K) {
		appendDimension(ImageConcurrencyTier4K, 1, a.config.Max4KConcurrent)
	}
	sort.Slice(dimensions, func(i, j int) bool {
		if dimensions[i].Name == dimensions[j].Name {
			return dimensions[i].ID < dimensions[j].ID
		}
		return dimensions[i].Name < dimensions[j].Name
	})
	return dimensions
}

func (s *ConcurrencyService) acquireImageSlotSet(ctx context.Context, dimensions []ImageConcurrencyDimension, requestID string) (bool, error) {
	if s == nil || s.cache == nil {
		return false, ErrImageGenerationAdmissionUnavailable
	}
	cache, ok := s.cache.(ImageConcurrencyCache)
	if !ok {
		return false, ErrImageGenerationAdmissionUnavailable
	}
	return cache.AcquireImageSlots(ctx, dimensions, requestID)
}

func (s *ConcurrencyService) releaseImageSlotSet(ctx context.Context, dimensions []ImageConcurrencyDimension, requestID string) error {
	if s == nil || s.cache == nil {
		return nil
	}
	cache, ok := s.cache.(ImageConcurrencyCache)
	if !ok {
		return nil
	}
	return cache.ReleaseImageSlots(ctx, dimensions, requestID)
}

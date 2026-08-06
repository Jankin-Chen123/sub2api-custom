package service

import (
	"context"
	"errors"
)

// ImageGenerationWakeup broadcasts only the durable job ID. It is an
// optimization for workers in other processes: PostgreSQL remains the source
// of truth, and a missed message is recovered by the worker's database scan.
// Implementations must not put prompts, credentials, image bytes, or result
// URLs in the message.
type ImageGenerationWakeup interface {
	PublishImageGenerationWakeup(ctx context.Context, jobID string) error
	SubscribeImageGenerationWakeups(ctx context.Context, handler func(jobID string)) error
}

var ErrImageGenerationWakeupUnavailable = errors.New("image generation wakeup is unavailable")

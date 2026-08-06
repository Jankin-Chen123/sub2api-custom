package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrImageGenerationPayloadNotFound = errors.New("image generation payload not found")

// ImageGenerationPayload is the encrypted, short-lived part of an image job.
// Prompts, reference images, masks, base64 output, and temporary upstream URLs
// must stay here instead of the PostgreSQL job row or application logs.
type ImageGenerationPayload struct {
	Request       CangyuanImageRequest `json:"request"`
	PendingResult *CangyuanImageResult `json:"pending_result,omitempty"`
}

type ImageGenerationPayloadStore interface {
	Save(ctx context.Context, ref string, payload *ImageGenerationPayload, ttl time.Duration) error
	Get(ctx context.Context, ref string) (*ImageGenerationPayload, error)
	Delete(ctx context.Context, ref string) error
}

// ImageGenerationPayloadExpiryCleaner is optional so existing payload-store
// implementations remain source-compatible. The durable PostgreSQL store
// implements it to remove expired rows for jobs that never reached a terminal
// state while every worker was offline.
type ImageGenerationPayloadExpiryCleaner interface {
	PurgeExpired(ctx context.Context, before time.Time, limit int) (int64, error)
}

func ImageGenerationPayloadRef(jobID string) string {
	return "image-generation/" + strings.TrimSpace(jobID)
}

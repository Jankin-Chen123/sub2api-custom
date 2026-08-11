package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type CreateDedicatedImageJobParams struct {
	UserID         int64
	APIKeyID       int64
	GroupID        *int64
	SubscriptionID *int64
	BillingType    int8
	Source         string
	Operation      string
	PublicModel    string
	DisplayName    string
	Request        CangyuanImageRequest
	IdempotencyKey string
	BaseCost       float64
	RateMultiplier float64
	EstimatedCost  float64
}

type ImageGenerationOrchestrator struct {
	repo     ImageGenerationJobRepository
	payloads ImageGenerationPayloadStore
	assets   *CangyuanImageAssetResolver
	wakeup   ImageGenerationWakeup
	queue    *ImageGenerationQueueController
	ttl      time.Duration
}

func NewImageGenerationOrchestrator(repo ImageGenerationJobRepository, payloads ImageGenerationPayloadStore, ttl time.Duration, wakeups ...ImageGenerationWakeup) *ImageGenerationOrchestrator {
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}
	var wakeup ImageGenerationWakeup
	if len(wakeups) > 0 {
		wakeup = wakeups[0]
	}
	return &ImageGenerationOrchestrator{repo: repo, payloads: payloads, assets: NewCangyuanImageAssetResolver(cangyuanMaxReferenceImageBytes), wakeup: wakeup, ttl: ttl}
}

// SetQueueController keeps the constructor backwards compatible for tests and
// integrations that create the orchestrator directly.
func (s *ImageGenerationOrchestrator) SetQueueController(queue *ImageGenerationQueueController) {
	if s != nil {
		s.queue = queue
	}
}

func (s *ImageGenerationOrchestrator) Create(ctx context.Context, params CreateDedicatedImageJobParams) (*ImageGenerationJob, bool, error) {
	if s == nil || s.repo == nil || s.payloads == nil {
		return nil, false, errors.New("image generation orchestrator is not configured")
	}
	if params.UserID <= 0 || params.APIKeyID <= 0 {
		return nil, false, errors.New("image generation owner is required")
	}
	params.Source = strings.TrimSpace(params.Source)
	switch params.Source {
	case ImageGenerationJobSourceAPI, ImageGenerationJobSourceCodex, ImageGenerationJobSourceWorkbench, ImageGenerationJobSourceAdminTest:
	default:
		return nil, false, errors.New("invalid image generation source")
	}
	operation := CangyuanImageOperationGeneration
	if params.Operation == ImageGenerationJobOperationEdit {
		operation = CangyuanImageOperationEdit
	} else if params.Operation != "" && params.Operation != ImageGenerationJobOperationGeneration {
		return nil, false, errors.New("invalid image generation operation")
	}
	// Keep the persisted job operation aligned with the value used for
	// validation. Empty operation is a backwards-compatible generation
	// default, but it must not reach the database as an empty string.
	params.Operation = string(operation)
	params.PublicModel = strings.TrimSpace(params.PublicModel)
	params.Request.Model = params.PublicModel
	if err := ValidateCangyuanImageRequest(operation, params.Request); err != nil {
		return nil, false, err
	}
	if operation != CangyuanImageOperationEdit && strings.TrimSpace(params.Request.Mask) != "" {
		return nil, false, &CangyuanAdapterError{Code: "image_invalid_mask", HTTPStatus: http.StatusBadRequest, Err: errors.New("mask is only supported for edit requests")}
	}
	var atomicQueueAdmission ImageGenerationQueueAdmission
	var atomicQueueLimit int
	if s.queue != nil {
		queueSettings, err := s.queue.Settings(ctx)
		if err != nil {
			return nil, false, ErrImageGenerationQueueUnavailable
		}
		if queueSettings.Enabled {
			if admission, ok := s.repo.(ImageGenerationQueueAdmission); ok {
				atomicQueueAdmission = admission
				atomicQueueLimit = queueSettings.MaxQueued
			} else {
				allowed, countErr := s.queue.CanEnqueue(ctx)
				if countErr != nil {
					return nil, false, ErrImageGenerationQueueUnavailable
				}
				if !allowed {
					return nil, false, ErrImageGenerationQueueFull
				}
			}
		}
	}
	if len(params.Request.Images) > 0 {
		if s.assets == nil {
			return nil, false, errors.New("image reference asset resolver is unavailable")
		}
		var images []CangyuanResolvedAsset
		var mask *CangyuanResolvedAsset
		var err error
		if operation == CangyuanImageOperationEdit {
			images, mask, err = s.assets.ResolveEditAssets(ctx, params.Request.Images, params.Request.Mask)
		} else {
			images, err = s.assets.ResolveUnique(ctx, params.Request.Images)
		}
		if err != nil {
			return nil, false, err
		}
		params.Request.Images = make([]string, 0, len(images))
		for _, asset := range images {
			params.Request.Images = append(params.Request.Images, imageAssetDataURL(asset.ContentType, asset.Data))
		}
		if mask != nil {
			params.Request.Mask = imageAssetDataURL(mask.ContentType, mask.Data)
			// Cangyuan's live contract accepts masked edits reliably through
			// multipart, while its JSON mask variant currently returns an
			// upstream 502. Keep the public JSON/workbench contract stable but
			// promote the internal request after assets are normalized.
			params.Request.Multipart = true
		}
	}
	requestHash, err := hashDedicatedImageJobRequest(params.Operation, params.PublicModel, params.Request)
	if err != nil {
		return nil, false, err
	}
	promptHash := sha256.Sum256([]byte(params.Request.Prompt))
	jobID, err := NewImageGenerationJobID()
	if err != nil {
		return nil, false, err
	}
	payloadRef := ImageGenerationPayloadRef(jobID)
	payload := &ImageGenerationPayload{Request: params.Request}
	if err := s.payloads.Save(ctx, payloadRef, payload, s.ttl); err != nil {
		return nil, false, err
	}

	idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
	var idempotencyKeyPtr *string
	if idempotencyKey != "" {
		idempotencyKeyPtr = &idempotencyKey
	}
	requestHashPtr := requestHash
	requestedSize := strings.TrimSpace(params.Request.Size)
	if requestedSize == "" {
		// The public job schema predates aspect_ratio; retain the requested
		// geometry in the same safe, non-sensitive field so pending jobs remain
		// understandable without persisting the prompt.
		requestedSize = strings.TrimSpace(params.Request.AspectRatio)
	}
	var requestedSizePtr *string
	if requestedSize != "" {
		requestedSizePtr = &requestedSize
	}
	quality := strings.TrimSpace(params.Request.Quality)
	var qualityPtr *string
	if quality != "" {
		qualityPtr = &quality
	}
	displayName := truncateRunes(strings.TrimSpace(params.DisplayName), 80)
	var displayNamePtr *string
	if displayName != "" {
		displayNamePtr = &displayName
	}
	responseFormat := strings.TrimSpace(params.Request.ResponseFormat)
	var responseFormatPtr *string
	if responseFormat != "" {
		responseFormatPtr = &responseFormat
	}
	createParams := CreateImageGenerationJobParams{
		JobID:            jobID,
		UserID:           int64Pointer(params.UserID),
		APIKeyID:         int64Pointer(params.APIKeyID),
		GroupID:          params.GroupID,
		SubscriptionID:   params.SubscriptionID,
		BillingType:      params.BillingType,
		Source:           params.Source,
		Operation:        params.Operation,
		Status:           ImageGenerationJobStatusCreated,
		PublicModel:      params.PublicModel,
		DisplayName:      displayNamePtr,
		RequestedSize:    requestedSizePtr,
		Quality:          qualityPtr,
		ResponseFormat:   responseFormatPtr,
		IdempotencyKey:   idempotencyKeyPtr,
		RequestHash:      &requestHashPtr,
		PromptHash:       hex.EncodeToString(promptHash[:]),
		PayloadObjectRef: &payloadRef,
		BaseCost:         params.BaseCost,
		RateMultiplier:   params.RateMultiplier,
		EstimatedCost:    params.EstimatedCost,
		HeldCost:         0,
	}
	var job *ImageGenerationJob
	var replayed bool
	if atomicQueueAdmission != nil {
		job, replayed, err = atomicQueueAdmission.CreateImageGenerationJobWithQueueLimit(ctx, createParams, atomicQueueLimit)
	} else {
		job, replayed, err = s.repo.CreateImageGenerationJob(ctx, createParams)
	}
	if err != nil {
		_ = s.payloads.Delete(ctx, payloadRef)
		return nil, false, err
	}
	recordImageGenerationCreated(replayed)
	if replayed {
		// The candidate payload belongs to the discarded new job ID. The
		// existing job keeps its own encrypted payload reference.
		_ = s.payloads.Delete(ctx, payloadRef)
	}
	// Publish only after the durable row exists. A failed publish is harmless:
	// the Worker still discovers the job through PostgreSQL polling/recovery.
	if s.wakeup != nil && job != nil && strings.TrimSpace(job.JobID) != "" {
		_ = s.wakeup.PublishImageGenerationWakeup(ctx, job.JobID)
	}
	return job, replayed, nil
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func imageAssetDataURL(contentType string, data []byte) string {
	return "data:" + strings.TrimSpace(contentType) + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func hashDedicatedImageJobRequest(operation, publicModel string, request CangyuanImageRequest) (string, error) {
	raw, err := json.Marshal(struct {
		Operation   string               `json:"operation"`
		PublicModel string               `json:"public_model"`
		Request     CangyuanImageRequest `json:"request"`
	}{Operation: operation, PublicModel: publicModel, Request: request})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

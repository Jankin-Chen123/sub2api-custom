package service

import (
	"context"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageOrchestratorRepo struct {
	ImageGenerationJobRepository
	params   []CreateImageGenerationJobParams
	existing *ImageGenerationJob
	replayed bool
	err      error
}

func (r *imageOrchestratorRepo) CreateImageGenerationJob(_ context.Context, params CreateImageGenerationJobParams) (*ImageGenerationJob, bool, error) {
	r.params = append(r.params, params)
	if r.err != nil {
		return nil, false, r.err
	}
	if r.existing != nil {
		return r.existing, r.replayed, nil
	}
	return &ImageGenerationJob{
		JobID: params.JobID, UserID: params.UserID, APIKeyID: params.APIKeyID, GroupID: params.GroupID,
		Source: params.Source, Operation: params.Operation, Status: params.Status,
		PublicModel: params.PublicModel, DisplayName: params.DisplayName, RequestedSize: params.RequestedSize,
		IdempotencyKey: params.IdempotencyKey, RequestHash: params.RequestHash,
		PromptHash: params.PromptHash, PayloadObjectRef: params.PayloadObjectRef,
		EstimatedCost: params.EstimatedCost, HeldCost: params.HeldCost,
	}, false, nil
}

type imageOrchestratorPayloadStore struct {
	saved   map[string]*ImageGenerationPayload
	deleted []string
	ttl     time.Duration
}

func (s *imageOrchestratorPayloadStore) Save(_ context.Context, ref string, payload *ImageGenerationPayload, ttl time.Duration) error {
	if s.saved == nil {
		s.saved = make(map[string]*ImageGenerationPayload)
	}
	copyValue := *payload
	s.saved[ref] = &copyValue
	s.ttl = ttl
	return nil
}
func (s *imageOrchestratorPayloadStore) Get(_ context.Context, ref string) (*ImageGenerationPayload, error) {
	if payload := s.saved[ref]; payload != nil {
		copyValue := *payload
		return &copyValue, nil
	}
	return nil, ErrImageGenerationPayloadNotFound
}
func (s *imageOrchestratorPayloadStore) Delete(_ context.Context, ref string) error {
	delete(s.saved, ref)
	s.deleted = append(s.deleted, ref)
	return nil
}

type imageGenerationWakeupRecorder struct {
	published []string
	err       error
}

func (r *imageGenerationWakeupRecorder) PublishImageGenerationWakeup(_ context.Context, jobID string) error {
	r.published = append(r.published, jobID)
	return r.err
}

func (r *imageGenerationWakeupRecorder) SubscribeImageGenerationWakeups(context.Context, func(string)) error {
	return nil
}

func TestImageGenerationOrchestratorPublishesAfterDurableCreateWithoutMakingRedisRequired(t *testing.T) {
	repo := &imageOrchestratorRepo{}
	payloads := &imageOrchestratorPayloadStore{}
	wakeup := &imageGenerationWakeupRecorder{}
	orchestrator := NewImageGenerationOrchestrator(repo, payloads, time.Hour, wakeup)

	job, _, err := orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: 1, APIKeyID: 2, Source: ImageGenerationJobSourceWorkbench,
		PublicModel: CangyuanImageModel1K,
		Request:     CangyuanImageRequest{Prompt: "wake the worker", Size: "1024x1024", N: 1},
	})
	require.NoError(t, err)
	require.NotNil(t, job)
	firstJobID := job.JobID
	require.Equal(t, []string{firstJobID}, wakeup.published)

	wakeup.err = ErrImageGenerationWakeupUnavailable
	job, _, err = orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: 1, APIKeyID: 2, Source: ImageGenerationJobSourceWorkbench,
		PublicModel: CangyuanImageModel1K,
		Request:     CangyuanImageRequest{Prompt: "database fallback", Size: "1024x1024", N: 1},
	})
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, []string{firstJobID, job.JobID}, wakeup.published)
}

func TestImageGenerationOrchestratorStoresSensitivePayloadOutsidePostgres(t *testing.T) {
	repo := &imageOrchestratorRepo{}
	payloads := &imageOrchestratorPayloadStore{}
	orchestrator := NewImageGenerationOrchestrator(repo, payloads, 2*time.Hour)
	groupID := int64(9)

	job, replayed, err := orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: 1, APIKeyID: 2, GroupID: &groupID,
		Source: ImageGenerationJobSourceWorkbench, Operation: ImageGenerationJobOperationGeneration,
		PublicModel:    CangyuanImageModel2K,
		Request:        CangyuanImageRequest{Prompt: "sensitive prompt text", Size: "2048x2048", N: 1, ResponseFormat: "url"},
		IdempotencyKey: "idem-1", EstimatedCost: 0.25,
	})
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, ImageGenerationJobStatusCreated, job.Status)
	require.Len(t, repo.params, 1)
	stored := repo.params[0]
	require.NotEqual(t, "sensitive prompt text", stored.PromptHash)
	require.Len(t, stored.PromptHash, 64)
	require.NotNil(t, stored.RequestHash)
	require.Len(t, *stored.RequestHash, 64)
	require.NotNil(t, stored.PayloadObjectRef)
	require.NotContains(t, *stored.PayloadObjectRef, "sensitive prompt text")
	require.Equal(t, "sensitive prompt text", payloads.saved[*stored.PayloadObjectRef].Request.Prompt)
	require.Equal(t, CangyuanImageModel2K, payloads.saved[*stored.PayloadObjectRef].Request.Model)
	require.Equal(t, 2*time.Hour, payloads.ttl)
}

func TestImageGenerationOrchestratorNormalizesDisplayName(t *testing.T) {
	repo := &imageOrchestratorRepo{}
	orchestrator := NewImageGenerationOrchestrator(repo, &imageOrchestratorPayloadStore{}, time.Hour)
	displayName := "  " + strings.Repeat("图", 81) + "  "

	job, _, err := orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: 1, APIKeyID: 2, Source: ImageGenerationJobSourceWorkbench,
		PublicModel: CangyuanImageModel1K, DisplayName: displayName,
		Request: CangyuanImageRequest{Prompt: "a prompt", Size: "1024x1024", N: 1},
	})

	require.NoError(t, err)
	require.NotNil(t, job.DisplayName)
	require.Equal(t, strings.Repeat("图", 80), *job.DisplayName)
}

func TestImageGenerationOrchestratorNormalizesEmptyOperationToGeneration(t *testing.T) {
	repo := &imageOrchestratorRepo{}
	payloads := &imageOrchestratorPayloadStore{}
	orchestrator := NewImageGenerationOrchestrator(repo, payloads, time.Hour)

	job, _, err := orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: 1, APIKeyID: 2, Source: ImageGenerationJobSourceAPI,
		// Operation intentionally omitted for compatibility with older callers.
		PublicModel: CangyuanImageModel1K,
		Request:     CangyuanImageRequest{Prompt: "a small orange puppy", Size: "1024x1024", N: 1},
	})
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, ImageGenerationJobOperationGeneration, job.Operation)
	require.Len(t, repo.params, 1)
	require.Equal(t, ImageGenerationJobOperationGeneration, repo.params[0].Operation)
}

func TestImageGenerationOrchestratorReplayDeletesOnlyDiscardedCandidatePayload(t *testing.T) {
	existingPayloadRef := ImageGenerationPayloadRef("imgjob_existing")
	existing := &ImageGenerationJob{JobID: "imgjob_existing", Status: ImageGenerationJobStatusQueued, PayloadObjectRef: &existingPayloadRef}
	repo := &imageOrchestratorRepo{existing: existing, replayed: true}
	payloads := &imageOrchestratorPayloadStore{saved: map[string]*ImageGenerationPayload{
		existingPayloadRef: {Request: CangyuanImageRequest{Prompt: "existing"}},
	}}
	orchestrator := NewImageGenerationOrchestrator(repo, payloads, time.Hour)

	job, replayed, err := orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: 1, APIKeyID: 2, Source: ImageGenerationJobSourceAPI,
		Operation: ImageGenerationJobOperationGeneration, PublicModel: CangyuanImageModel1K,
		Request:        CangyuanImageRequest{Prompt: "same request", Size: "1024x1024", N: 1},
		IdempotencyKey: "same-idempotency-key", EstimatedCost: 0.1,
	})
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, existing, job)
	require.Contains(t, payloads.saved, existingPayloadRef)
	require.Len(t, payloads.deleted, 1)
	require.NotEqual(t, existingPayloadRef, payloads.deleted[0])
}

func TestImageGenerationOrchestratorRejectsInvalidRequestBeforePersistence(t *testing.T) {
	repo := &imageOrchestratorRepo{}
	payloads := &imageOrchestratorPayloadStore{}
	orchestrator := NewImageGenerationOrchestrator(repo, payloads, time.Hour)

	_, _, err := orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: 1, APIKeyID: 2, Source: ImageGenerationJobSourceAPI,
		Operation: ImageGenerationJobOperationGeneration, PublicModel: CangyuanImageModel1K,
		Request: CangyuanImageRequest{Prompt: "invalid", Size: "2048x2048", N: 1},
	})
	require.Error(t, err)
	require.Empty(t, repo.params)
	require.Empty(t, payloads.saved)
}

func TestImageGenerationOrchestratorResolvesGenerationReferencesBeforePersistence(t *testing.T) {
	imageBytes := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()

	repo := &imageOrchestratorRepo{}
	payloads := &imageOrchestratorPayloadStore{}
	orchestrator := NewImageGenerationOrchestrator(repo, payloads, time.Hour)
	orchestrator.assets = newCangyuanImageAssetResolver(server.Client(), cangyuanMaxReferenceImageBytes, func(context.Context, string) (bool, error) { return false, nil })

	job, _, err := orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: 1, APIKeyID: 2, Source: ImageGenerationJobSourceAPI,
		Operation: ImageGenerationJobOperationGeneration, PublicModel: CangyuanImageModel1K,
		Request: CangyuanImageRequest{
			Prompt: "transform this reference", Size: "1024x1024", N: 1,
			Images: []string{server.URL + "/source.png", server.URL + "/source.png"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, job)
	stored := payloads.saved[*repo.params[0].PayloadObjectRef].Request
	require.Len(t, stored.Images, 1)
	require.True(t, strings.HasPrefix(stored.Images[0], "data:image/png;base64,"))
}

func TestImageGenerationOrchestratorPromotesMaskedEditToMultipart(t *testing.T) {
	input := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	maskImage := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	maskImage.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 128})
	mask := encodePNG(t, maskImage)
	repo := &imageOrchestratorRepo{}
	payloads := &imageOrchestratorPayloadStore{}
	orchestrator := NewImageGenerationOrchestrator(repo, payloads, time.Hour)

	job, _, err := orchestrator.Create(context.Background(), CreateDedicatedImageJobParams{
		UserID: 1, APIKeyID: 2, Source: ImageGenerationJobSourceWorkbench,
		Operation: ImageGenerationJobOperationEdit, PublicModel: CangyuanImageModel1K,
		Request: CangyuanImageRequest{
			Prompt: "replace the background", Size: "1024x1024", N: 1,
			Images: []string{imageAssetDataURL("image/png", input)},
			Mask:   imageAssetDataURL("image/png", mask),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, job)
	stored := payloads.saved[*repo.params[0].PayloadObjectRef].Request
	require.True(t, stored.Multipart)
	require.Len(t, stored.Images, 1)
	require.True(t, strings.HasPrefix(stored.Images[0], "data:image/png;base64,"))
	require.True(t, strings.HasPrefix(stored.Mask, "data:image/png;base64,"))
}

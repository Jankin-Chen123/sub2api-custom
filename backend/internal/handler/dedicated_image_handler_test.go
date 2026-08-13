package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dedicatedImageHandlerRepo struct {
	service.ImageGenerationJobRepository
	job       *service.ImageGenerationJob
	ownerUser int64
	ownerKey  int64
}

func (r *dedicatedImageHandlerRepo) GetImageGenerationJobForOwner(_ context.Context, userID, apiKeyID int64, jobID string) (*service.ImageGenerationJob, error) {
	r.ownerUser, r.ownerKey = userID, apiKeyID
	if r.job == nil || r.job.JobID != jobID {
		return nil, service.ErrImageGenerationJobNotFound
	}
	return r.job, nil
}

type dedicatedImageReader struct {
	ref string
	raw []byte
}

type dedicatedImagePayloadStore struct {
	payload *service.ImageGenerationPayload
	gotRef  string
}

func (s *dedicatedImagePayloadStore) Save(context.Context, string, *service.ImageGenerationPayload, time.Duration) error {
	return nil
}

func (s *dedicatedImagePayloadStore) Get(_ context.Context, ref string) (*service.ImageGenerationPayload, error) {
	s.gotRef = ref
	if s.payload == nil {
		return nil, service.ErrImageGenerationPayloadNotFound
	}
	return s.payload, nil
}

func (s *dedicatedImagePayloadStore) Delete(context.Context, string) error {
	return nil
}

func (r *dedicatedImageReader) Open(_ context.Context, ref string) (io.ReadCloser, string, int64, error) {
	r.ref = ref
	return io.NopCloser(bytes.NewReader(r.raw)), "image/png", int64(len(r.raw)), nil
}

func TestDedicatedImageDispatchDisabledPreservesFallbackBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-image-2-4k","prompt":"dog"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	h := &DedicatedImageHandler{enabled: false}
	called := false

	h.Dispatch(c, func(c *gin.Context) {
		called = true
		got, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, body, got)
		c.Status(http.StatusNoContent)
		c.Writer.WriteHeaderNow()
	})

	require.True(t, called)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestNormalizeCodexNativeImageRequestUsesDedicated1KAndBase64(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Header.Set("originator", "codex-tui")
	parsed := &service.OpenAIImagesRequest{
		Model: "gpt-image-2", Size: "auto", ExplicitSize: true, Quality: "auto",
	}

	require.True(t, (&DedicatedImageHandler{}).normalizeCodexNativeImageRequest(c, parsed))
	require.Equal(t, service.CangyuanImageModel1K, parsed.Model)
	require.Empty(t, parsed.Size)
	require.False(t, parsed.ExplicitSize)
	require.Equal(t, "b64_json", parsed.ResponseFormat)
}

func TestNormalizeCodexNativeImageRequestKeepsDesktopDedicatedTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Header.Set("originator", "codex-tui")
	parsed := &service.OpenAIImagesRequest{
		Model: service.CangyuanImageModel2K, Size: "auto", ExplicitSize: true, ResponseFormat: "url",
	}

	require.True(t, (&DedicatedImageHandler{}).normalizeCodexNativeImageRequest(c, parsed))
	require.Equal(t, service.CangyuanImageModel2K, parsed.Model)
	require.Empty(t, parsed.Size)
	require.False(t, parsed.ExplicitSize)
	require.Equal(t, "b64_json", parsed.ResponseFormat)
}

func TestNormalizeCodexNativeImageRequestDoesNotAliasOrdinaryClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Header.Set("User-Agent", "openai-node/6")
	parsed := &service.OpenAIImagesRequest{Model: "gpt-image-2", Size: "auto"}

	require.False(t, (&DedicatedImageHandler{}).normalizeCodexNativeImageRequest(c, parsed))
	require.Equal(t, "gpt-image-2", parsed.Model)
	require.Equal(t, "auto", parsed.Size)
}

func TestCodexImageJSONHeartbeatWritesOnlyWhitespaceAndStops(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	stop := service.StartOpenAIImagesJSONKeepalive(c, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	stop()

	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Equal(t, "no", rec.Header().Get("X-Accel-Buffering"))
	require.Empty(t, bytes.TrimSpace(rec.Body.Bytes()))
}

func TestDedicatedImageIdempotencyKeyPrefersExplicitAndScopesFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Header.Set("Idempotency-Key", " client-key ")
	require.Equal(t, "client-key", dedicatedImageIdempotencyKey(c, "generation"))

	c.Request.Header.Del("Idempotency-Key")
	c.Writer.Header().Set("X-Request-ID", "request-123")
	require.Equal(t, "request:edit:request-123", dedicatedImageIdempotencyKey(c, "edit"))
}

func TestDedicatedCangyuanRequestConvertsUploadsWithoutChangingTier(t *testing.T) {
	request, err := dedicatedCangyuanRequest(&service.OpenAIImagesRequest{
		Model: service.CangyuanImageModel4K, Prompt: "edit", Size: "3840x2160", N: 1,
		ResponseFormat: "url", InputImageURLs: []string{"https://example.test/input.png"},
		Uploads:    []service.OpenAIImagesUpload{{ContentType: "image/png", Data: []byte("image-bytes")}},
		MaskUpload: &service.OpenAIImagesUpload{ContentType: "image/png", Data: []byte("mask-bytes")},
	})
	require.NoError(t, err)
	require.Equal(t, "4K", request.ImageSize)
	require.Equal(t, "4K", request.OutputResolution)
	require.True(t, request.Async)
	require.Len(t, request.Images, 2)
	require.Contains(t, request.Images[1], "data:image/png;base64,")
	require.Contains(t, request.Mask, "data:image/png;base64,")
}

func TestDedicatedCodexRequestForcesSynchronousProviderBase64WhenGlobalBase64IsDisabled(t *testing.T) {
	parsed := &service.OpenAIImagesRequest{
		Model: service.CangyuanImageModel1K, Prompt: "TCP map", N: 1, ResponseFormat: "b64_json",
	}
	request, err := dedicatedCangyuanRequest(parsed)
	require.NoError(t, err)

	configureCodexNativeImageDelivery(&request)

	require.Equal(t, "b64_json", request.ResponseFormat)
	require.False(t, request.Async)
}

func TestDedicatedImageContentUsesOwnerScopeAndPrivateCacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	job := &service.ImageGenerationJob{
		JobID: "imgjob_content", Status: service.ImageGenerationJobStatusCompleted,
		ResultObjectRefs: []string{"images/cangyuan/imgjob_content/0.png"}, CreatedAt: now,
	}
	repo := &dedicatedImageHandlerRepo{job: job}
	reader := &dedicatedImageReader{raw: []byte("png-result")}
	h := &DedicatedImageHandler{repo: repo, results: reader}
	req := httptest.NewRequest(http.MethodGet, "/v1/images/tasks/imgjob_content/content", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "task_id", Value: "imgjob_content"}}
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 22, UserID: 11})

	h.Content(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	require.Equal(t, "png-result", rec.Body.String())
	require.Equal(t, int64(11), repo.ownerUser)
	require.Equal(t, int64(22), repo.ownerKey)
	require.Equal(t, job.ResultObjectRefs[0], reader.ref)
}

func TestDedicatedImageCodexCompletionReturnsEncryptedPayloadBase64WithoutObjectStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payloadRef := service.ImageGenerationPayloadRef("imgjob_codex")
	encoded := base64.StdEncoding.EncodeToString([]byte("provider-image"))
	payloads := &dedicatedImagePayloadStore{payload: &service.ImageGenerationPayload{CodexResult: &service.CodexImageResult{
		Base64: encoded, OutputFormat: "png", ActualSize: "1024x1024",
	}}}
	h := &DedicatedImageHandler{payloads: payloads}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	job := &service.ImageGenerationJob{
		JobID: "imgjob_codex", Source: service.ImageGenerationJobSourceCodex,
		PayloadObjectRef: &payloadRef, CreatedAt: time.Unix(123, 0), ResultObjectRefs: []string{},
	}

	h.writeCompletedImage(c, job, true)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Contains(t, rec.Body.String(), encoded)
	require.Equal(t, payloadRef, payloads.gotRef)
}

func TestDedicatedImageOrdinaryCompletionWithMissingObjectRefReturnsErrorInsteadOfPanicking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &DedicatedImageHandler{}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	responseFormat := "b64_json"
	job := &service.ImageGenerationJob{
		JobID: "imgjob_missing_ref", ResponseFormat: &responseFormat,
		CreatedAt: time.Unix(123, 0), ResultObjectRefs: []string{},
	}

	require.NotPanics(t, func() { h.writeCompletedImage(c, job, false) })
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), `image_result_payload_missing`)
}

func TestDedicatedImageTaskResponseDoesNotExposePrivateBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accountID := int64(91)
	upstreamTaskID := "private-upstream-task"
	job := &service.ImageGenerationJob{
		JobID: "imgjob_public", Status: service.ImageGenerationJobStatusPolling,
		PublicModel: service.CangyuanImageModel2K, AccountID: &accountID, UpstreamTaskID: &upstreamTaskID,
		CreatedAt: time.Unix(100, 0),
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/images/tasks/imgjob_public", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	response := dedicatedImageTaskResponse(c, job)
	require.Equal(t, "imgjob_public", response["id"])
	require.Equal(t, "in_progress", response["status"])
	require.NotContains(t, response, "account_id")
	require.NotContains(t, response, "upstream_task_id")
}

func TestDedicatedImageWriteServiceErrorMapsTransportFailuresToGatewayStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	h := &DedicatedImageHandler{}
	h.writeServiceError(c, &service.CangyuanAdapterError{
		Code: "image_upstream_timeout", Err: errors.New("context deadline exceeded"),
	})
	require.Equal(t, http.StatusGatewayTimeout, recorder.Code)
	require.Contains(t, recorder.Body.String(), "image_upstream_timeout")
}

func TestDedicatedImagePublicErrorMessageDoesNotExposeProviderDetails(t *testing.T) {
	message := dedicatedImagePublicErrorMessage(
		"image_upstream_rejected",
		"Authorization: Bearer sk-provider-secret https://provider.example/image.png?signature=private",
	)
	require.Equal(t, "the image provider rejected or could not complete the request", message)
	require.NotContains(t, message, "sk-provider-secret")
	require.NotContains(t, message, "provider.example")
}

func TestDedicatedImageGroupIDPreservesAPIKeyGroupScope(t *testing.T) {
	groupID := int64(41)
	apiKey := &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: 99}}
	resolved := dedicatedImageGroupID(apiKey)
	require.NotNil(t, resolved)
	require.Equal(t, int64(41), *resolved)

	apiKey.GroupID = nil
	resolved = dedicatedImageGroupID(apiKey)
	require.NotNil(t, resolved)
	require.Equal(t, int64(99), *resolved)

	require.Nil(t, dedicatedImageGroupID(&service.APIKey{}))
	require.Nil(t, dedicatedImageGroupID(nil))
}

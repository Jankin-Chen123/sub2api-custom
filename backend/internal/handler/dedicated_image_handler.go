package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	pkgopenai "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DedicatedImageHandler struct {
	orchestrator   *service.ImageGenerationOrchestrator
	repo           service.ImageGenerationJobRepository
	results        service.ImageGenerationResultReader
	billing        *service.BillingService
	worker         *service.ImageGenerationWorkerRuntime
	openAI         *OpenAIGatewayHandler
	imageStorage   *service.ImageStorageSettingService
	enabled        bool
	forceCodexCLI  bool
	syncTimeout    time.Duration
	codexHeartbeat time.Duration
	maxReadBytes   int64
}

func NewDedicatedImageHandler(
	orchestrator *service.ImageGenerationOrchestrator,
	repo service.ImageGenerationJobRepository,
	results service.ImageGenerationResultReader,
	billing *service.BillingService,
	worker *service.ImageGenerationWorkerRuntime,
	openAI *OpenAIGatewayHandler,
	imageStorage *service.ImageStorageSettingService,
	cfg *config.Config,
) *DedicatedImageHandler {
	h := &DedicatedImageHandler{
		orchestrator: orchestrator, repo: repo, results: results, billing: billing,
		worker: worker, openAI: openAI, imageStorage: imageStorage,
		syncTimeout: 3 * time.Minute, codexHeartbeat: 15 * time.Second, maxReadBytes: 64 << 20,
	}
	if cfg != nil {
		h.enabled = cfg.DedicatedImage.Enabled
		h.forceCodexCLI = cfg.Gateway.ForceCodexCLI
		if cfg.Gateway.ImageNonstreamKeepaliveInterval > 0 {
			h.codexHeartbeat = time.Duration(cfg.Gateway.ImageNonstreamKeepaliveInterval) * time.Second
		}
		if cfg.DedicatedImage.SyncWaitTimeoutSeconds > 0 {
			h.syncTimeout = time.Duration(cfg.DedicatedImage.SyncWaitTimeoutSeconds) * time.Second
		}
		if cfg.DedicatedImage.MaxOutputBytes > 0 {
			h.maxReadBytes = cfg.DedicatedImage.MaxOutputBytes
		}
	}
	return h
}

// Dispatch sends only the three explicit Cangyuan tier models through the
// durable dedicated path. Every other request is restored byte-for-byte and
// delegated to the existing Images implementation.
func (h *DedicatedImageHandler) Dispatch(c *gin.Context, fallback gin.HandlerFunc) {
	if h == nil || !h.enabled || h.openAI == nil || h.openAI.gatewayService == nil {
		fallback(c)
		return
	}
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		h.writeError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	restoreRequestBody(c, body)
	parsed, err := h.openAI.gatewayService.ParseOpenAIImagesRequest(c, body)
	if err != nil {
		// Preserve the existing endpoint's validation and error behavior for
		// requests that do not cleanly identify a dedicated tier.
		fallback(c)
		return
	}
	codexNativeImage := h.normalizeCodexNativeImageRequest(c, parsed)
	if !isDedicatedCangyuanModel(parsed.Model) {
		fallback(c)
		return
	}
	if parsed.Stream {
		h.writeError(c, http.StatusBadRequest, "image_stream_not_supported", "dedicated Cangyuan image models do not support streaming responses")
		return
	}
	h.submit(c, body, parsed, parsed.Async || strings.Contains(c.Request.URL.Path, "/async"), codexNativeImage)
}

func (h *DedicatedImageHandler) submit(c *gin.Context, body []byte, parsed *service.OpenAIImagesRequest, forceAsync, codexNativeImage bool) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		h.writeError(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.writeError(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	if apiKey.Group == nil || apiKey.Group.Platform != service.PlatformOpenAI || !service.GroupAllowsImageGeneration(apiKey.Group) {
		h.writeError(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	if h.worker == nil || !h.worker.Running() {
		h.writeError(c, http.StatusServiceUnavailable, "image_orchestration_unavailable", "dedicated image worker is not enabled")
		return
	}
	reqLog := requestLogger(c, "handler.dedicated_image", dedicatedImageLogFields(
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.String("model", parsed.Model),
	)...)
	if decision := h.openAI.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, parsed.Model, parsed.ModerationBody()); decision != nil && !decision.AllowNextStage {
		h.openAI.openAISecurityAuditError(c, decision)
		return
	}
	imageRelease, acquired := h.openAI.acquireImageGenerationSlot(c, false, parsed.Model)
	if !acquired {
		return
	}
	if imageRelease != nil {
		defer func() {
			if imageRelease != nil {
				imageRelease()
			}
		}()
	}
	userRelease, acquired := h.openAI.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, new(bool), reqLog)
	if !acquired {
		return
	}
	if userRelease != nil {
		defer userRelease()
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if h.openAI.billingCacheService == nil {
		h.writeError(c, http.StatusServiceUnavailable, "image_orchestration_unavailable", "billing service is unavailable")
		return
	}
	if err := h.openAI.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.writeError(c, status, code, message)
		return
	}
	billingType := service.BillingTypeBalance
	var subscriptionID *int64
	if subscription != nil && apiKey.Group.IsSubscriptionType() {
		billingType = service.BillingTypeSubscription
		subscriptionID = &subscription.ID
	}

	tier := dedicatedImageModelTier(parsed.Model)
	costEstimate, err := service.EstimateDedicatedImageCostSnapshot(h.billing, apiKey, parsed.Model, tier)
	if err != nil {
		h.writeError(c, http.StatusServiceUnavailable, "image_orchestration_unavailable", "image price could not be resolved")
		return
	}
	request, err := dedicatedCangyuanRequest(parsed)
	if err != nil {
		h.writeError(c, http.StatusBadRequest, "image_invalid_reference", err.Error())
		return
	}
	if strings.TrimSpace(request.ResponseFormat) == "" {
		request.ResponseFormat = "url"
	}
	if err := h.enforceResponseFormatPolicy(c.Request.Context(), request.ResponseFormat); err != nil {
		h.writeServiceError(c, err)
		return
	}
	operation := service.ImageGenerationJobOperationGeneration
	if parsed.IsEdits() {
		operation = service.ImageGenerationJobOperationEdit
	}
	if err := service.ValidateCangyuanImageRequest(service.CangyuanImageOperation(operation), request); err != nil {
		h.writeServiceError(c, err)
		return
	}
	source := service.ImageGenerationJobSourceAPI
	if codexNativeImage {
		source = service.ImageGenerationJobSourceCodex
	}
	job, _, err := h.orchestrator.Create(c.Request.Context(), service.CreateDedicatedImageJobParams{
		UserID: apiKey.UserID, APIKeyID: apiKey.ID, GroupID: dedicatedImageGroupID(apiKey),
		SubscriptionID: subscriptionID, BillingType: billingType,
		Source: source, Operation: operation,
		PublicModel: parsed.Model, Request: request,
		IdempotencyKey: dedicatedImageIdempotencyKey(c, string(operation)),
		BaseCost:       costEstimate.BaseCost, RateMultiplier: costEstimate.RateMultiplier, EstimatedCost: costEstimate.ActualCost,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	// The durable Worker owns the execution lease from this point onward.
	// Release inbound dimensions before waiting so a synchronous request cannot
	// deadlock the Worker on the same user/key/group lease.
	if imageRelease != nil {
		imageRelease()
		imageRelease = nil
	}
	if forceAsync {
		h.writeTaskAccepted(c, job)
		return
	}
	h.waitForCompletion(c, apiKey, job, codexNativeImage)
}

// normalizeCodexNativeImageRequest maps the fixed request emitted by Codex's
// client-side image_gen extension onto the default dedicated Cangyuan tier.
// The extension always sends model=gpt-image-2 with size=auto and expects a
// synchronous data[].b64_json response. Restrict the alias to official Codex
// clients (or the existing force_codex_cli compatibility switch) so ordinary
// Images API callers keep the upstream gpt-image-2 behavior unchanged.
func (h *DedicatedImageHandler) normalizeCodexNativeImageRequest(c *gin.Context, parsed *service.OpenAIImagesRequest) bool {
	if h == nil || c == nil || parsed == nil || strings.TrimSpace(parsed.Model) != "gpt-image-2" {
		return false
	}
	if !h.forceCodexCLI && !pkgopenai.IsCodexOfficialClientByHeaders(c.GetHeader("User-Agent"), c.GetHeader("originator")) {
		return false
	}
	parsed.Model = service.CangyuanImageModel1K
	if strings.EqualFold(strings.TrimSpace(parsed.Size), "auto") {
		parsed.Size = ""
		parsed.ExplicitSize = false
	}
	parsed.ResponseFormat = "b64_json"
	return true
}

// dedicatedImageGroupID keeps the task's scheduling scope stable even when
// an API key was loaded with its Group relation but without the nullable
// GroupID column populated. The worker must never widen a request to all
// groups because of that representation difference.
func dedicatedImageGroupID(apiKey *service.APIKey) *int64 {
	if apiKey == nil {
		return nil
	}
	if apiKey.GroupID != nil && *apiKey.GroupID > 0 {
		groupID := *apiKey.GroupID
		return &groupID
	}
	if apiKey.Group != nil && apiKey.Group.ID > 0 {
		groupID := apiKey.Group.ID
		return &groupID
	}
	return nil
}

func (h *DedicatedImageHandler) waitForCompletion(c *gin.Context, apiKey *service.APIKey, job *service.ImageGenerationJob, codexNativeImage bool) {
	stopHeartbeat := func() {}
	if codexNativeImage {
		// Cloudflare proxies abort silent origin reads after roughly two minutes.
		// Codex's image client expects one JSON document and does not support the
		// async task response, so send legal leading JSON whitespace before that
		// deadline and periodically thereafter. The guarded writer stops the
		// heartbeat before terminal JSON is emitted, preventing concurrent writes.
		interval := h.codexHeartbeat
		if interval <= 0 {
			interval = 15 * time.Second
		}
		stopHeartbeat = service.StartOpenAIImagesJSONKeepalive(c, interval)
	}
	defer stopHeartbeat()
	deadline := time.NewTimer(h.syncTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, err := h.repo.GetImageGenerationJobForOwner(c.Request.Context(), apiKey.UserID, apiKey.ID, job.JobID)
		if err != nil {
			h.writeServiceError(c, err)
			return
		}
		switch current.Status {
		case service.ImageGenerationJobStatusCompleted:
			h.writeCompletedImage(c, current)
			return
		case service.ImageGenerationJobStatusFailed:
			code := imageJobString(current.ErrorCode, "image_upstream_rejected")
			h.writeError(c, http.StatusBadGateway, code, imageJobString(current.ErrorMessage, "image generation failed"))
			return
		case service.ImageGenerationJobStatusSubmissionUnknown:
			h.writeError(c, http.StatusServiceUnavailable, "image_submission_unknown", "image submission outcome is unknown; query the task instead of resubmitting")
			return
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-deadline.C:
			h.writeTaskAccepted(c, current)
			return
		case <-ticker.C:
		}
	}
}

func (h *DedicatedImageHandler) Get(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		h.writeError(c, http.StatusNotFound, "image_task_not_found", "image task not found")
		return
	}
	job, err := h.repo.GetImageGenerationJobForOwner(c.Request.Context(), apiKey.UserID, apiKey.ID, c.Param("task_id"))
	if err != nil {
		h.writeError(c, http.StatusNotFound, "image_task_not_found", "image task not found")
		return
	}
	c.Header("Cache-Control", "no-store")
	if !isTerminalDedicatedImageStatus(job.Status) {
		c.Header("Retry-After", "2")
	}
	c.JSON(http.StatusOK, dedicatedImageTaskResponse(c, job))
}

func (h *DedicatedImageHandler) Content(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		h.writeError(c, http.StatusNotFound, "image_task_not_found", "image task not found")
		return
	}
	job, err := h.repo.GetImageGenerationJobForOwner(c.Request.Context(), apiKey.UserID, apiKey.ID, c.Param("task_id"))
	if err != nil || job.Status != service.ImageGenerationJobStatusCompleted || len(job.ResultObjectRefs) == 0 {
		h.writeError(c, http.StatusNotFound, "image_task_not_found", "image task result not found")
		return
	}
	body, contentType, contentLength, err := h.results.Open(c.Request.Context(), job.ResultObjectRefs[0])
	if err != nil {
		h.writeError(c, http.StatusBadGateway, "image_storage_failed", "image result could not be read")
		return
	}
	defer func() { _ = body.Close() }()
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s%s"`, job.JobID, imageContentExtension(contentType)))
	if contentLength >= 0 {
		c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, body)
}

func (h *DedicatedImageHandler) writeCompletedImage(c *gin.Context, job *service.ImageGenerationJob) {
	created := job.CreatedAt.Unix()
	if imageJobString(job.ResponseFormat, "url") == "b64_json" && h.imageStorage != nil && h.imageStorage.Base64ResponsesAllowed(c.Request.Context()) {
		body, _, _, err := h.results.Open(c.Request.Context(), job.ResultObjectRefs[0])
		if err != nil {
			h.writeError(c, http.StatusBadGateway, "image_storage_failed", "image result could not be read")
			return
		}
		defer func() { _ = body.Close() }()
		raw, err := io.ReadAll(io.LimitReader(body, h.maxReadBytes+1))
		if err != nil || int64(len(raw)) > h.maxReadBytes {
			h.writeError(c, http.StatusBadGateway, "image_storage_failed", "image result is too large to encode")
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"created": created, "data": []gin.H{{"b64_json": base64.StdEncoding.EncodeToString(raw)}}})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"created": created, "data": []gin.H{{"url": dedicatedImageContentPath(c, job.JobID)}}})
}

func (h *DedicatedImageHandler) writeTaskAccepted(c *gin.Context, job *service.ImageGenerationJob) {
	pollURL := dedicatedImageTaskPath(c, job.JobID)
	c.Header("Cache-Control", "no-store")
	c.Header("Location", pollURL)
	c.Header("Retry-After", "2")
	c.JSON(http.StatusAccepted, dedicatedImageTaskResponse(c, job))
}

func (h *DedicatedImageHandler) writeServiceError(c *gin.Context, err error) {
	var adapterErr *service.CangyuanAdapterError
	if errors.As(err, &adapterErr) && adapterErr != nil {
		status := adapterErr.HTTPStatus
		if status < http.StatusBadRequest || status > 599 {
			switch {
			case strings.HasPrefix(adapterErr.Code, "image_invalid_"), adapterErr.Code == "image_prompt_required", adapterErr.Code == "image_prompt_too_long", adapterErr.Code == "image_model_not_allowed":
				status = http.StatusBadRequest
			case adapterErr.Code == "image_upstream_timeout":
				status = http.StatusGatewayTimeout
			case strings.HasPrefix(adapterErr.Code, "image_upstream_"), strings.HasPrefix(adapterErr.Code, "image_reference_download_"):
				status = http.StatusBadGateway
			default:
				status = http.StatusServiceUnavailable
			}
		}
		message := dedicatedImagePublicErrorMessage(adapterErr.Code, adapterErr.Error())
		h.writeError(c, status, adapterErr.Code, message)
		return
	}
	if errors.Is(err, service.ErrImageGenerationIdempotency) {
		h.writeError(c, http.StatusConflict, "idempotency_conflict", "idempotency key was reused with different image parameters")
		return
	}
	if errors.Is(err, service.ErrImageGenerationQueueFull) {
		h.writeError(c, http.StatusTooManyRequests, "IMAGE_QUEUE_FULL", "image generation queue is full")
		return
	}
	if errors.Is(err, service.ErrImageGenerationQueueUnavailable) {
		h.writeError(c, http.StatusServiceUnavailable, "IMAGE_QUEUE_UNAVAILABLE", "image generation queue is temporarily unavailable")
		return
	}
	h.writeError(c, http.StatusServiceUnavailable, "image_orchestration_unavailable", "image job could not be created")
}

func dedicatedImagePublicErrorMessage(code, fallback string) string {
	code = strings.TrimSpace(code)
	switch {
	case code == "image_prompt_too_long":
		return "the image prompt is too long"
	case code == "image_upstream_timeout":
		return "the image provider timed out; the task may still be queryable"
	case code == "image_submission_unknown":
		return "the image submission outcome is unknown; query the task before retrying"
	case strings.HasPrefix(code, "image_upstream_"), code == "image_endpoint_invalid", code == "image_adapter_unavailable", code == "image_request_invalid", code == "image_request_encode_failed":
		return "the image provider rejected or could not complete the request"
	default:
		message := service.RedactImageGenerationErrorMessage(fallback, 1024)
		if message == "" {
			return "dedicated image request failed"
		}
		return message
	}
}

func (h *DedicatedImageHandler) writeError(c *gin.Context, status int, code, message string) {
	if h != nil && h.openAI != nil {
		h.openAI.errorResponse(c, status, code, message)
		return
	}
	c.JSON(status, gin.H{"error": gin.H{"type": code, "message": message}})
}

func dedicatedCangyuanRequest(parsed *service.OpenAIImagesRequest) (service.CangyuanImageRequest, error) {
	if parsed == nil {
		return service.CangyuanImageRequest{}, fmt.Errorf("image request is missing")
	}
	request := service.CangyuanImageRequest{
		Model: parsed.Model, Prompt: parsed.Prompt, Size: parsed.Size, N: parsed.N,
		AspectRatio: parsed.AspectRatio,
		Quality:     parsed.Quality, ResponseFormat: parsed.ResponseFormat, Async: true,
		ImageSize: dedicatedImageModelTier(parsed.Model), OutputResolution: dedicatedImageModelTier(parsed.Model), Multipart: parsed.Multipart,
		Images: append([]string(nil), parsed.InputImageURLs...), Mask: parsed.MaskImageURL,
	}
	for _, upload := range parsed.Uploads {
		if len(upload.Data) == 0 {
			continue
		}
		contentType := strings.TrimSpace(upload.ContentType)
		if contentType == "" {
			contentType = http.DetectContentType(upload.Data)
		}
		request.Images = append(request.Images, "data:"+contentType+";base64,"+base64.StdEncoding.EncodeToString(upload.Data))
	}
	if parsed.MaskUpload != nil && len(parsed.MaskUpload.Data) > 0 {
		contentType := strings.TrimSpace(parsed.MaskUpload.ContentType)
		if contentType == "" {
			contentType = http.DetectContentType(parsed.MaskUpload.Data)
		}
		request.Mask = "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(parsed.MaskUpload.Data)
	}
	return request, nil
}

func (h *DedicatedImageHandler) enforceResponseFormatPolicy(ctx context.Context, responseFormat string) error {
	responseFormat = strings.ToLower(strings.TrimSpace(responseFormat))
	if responseFormat == "" || responseFormat == "url" {
		return nil
	}
	if responseFormat != "b64_json" {
		return nil
	}
	if h != nil && h.imageStorage != nil && h.imageStorage.Base64ResponsesAllowed(ctx) {
		return nil
	}
	return &service.CangyuanAdapterError{
		Code:       "image_response_format_disabled",
		HTTPStatus: http.StatusForbidden,
		Err:        errors.New("b64_json image responses are disabled by the administrator; use response_format=url"),
	}
}

func dedicatedImageTaskResponse(c *gin.Context, job *service.ImageGenerationJob) gin.H {
	response := gin.H{
		"id": job.JobID, "task_id": job.JobID, "object": "image.generation.task",
		"status": publicDedicatedImageStatus(job.Status), "created_at": job.CreatedAt.Unix(),
		"model": job.PublicModel, "size": imageJobString(job.ActualSize, imageJobString(job.RequestedSize, "")),
		"poll_url": dedicatedImageTaskPath(c, job.JobID),
	}
	if job.Status == service.ImageGenerationJobStatusCompleted {
		response["data"] = []gin.H{{"url": dedicatedImageContentPath(c, job.JobID)}}
	}
	if job.Status == service.ImageGenerationJobStatusFailed || job.Status == service.ImageGenerationJobStatusSubmissionUnknown {
		response["error"] = gin.H{"code": imageJobString(job.ErrorCode, "image_upstream_rejected"), "message": imageJobString(job.ErrorMessage, "image generation failed")}
	}
	return response
}

func publicDedicatedImageStatus(status string) string {
	switch status {
	case service.ImageGenerationJobStatusCreated, service.ImageGenerationJobStatusQueued:
		return "queued"
	case service.ImageGenerationJobStatusCompleted:
		return "completed"
	case service.ImageGenerationJobStatusFailed:
		return "failed"
	case service.ImageGenerationJobStatusSubmissionUnknown:
		return "submission_unknown"
	default:
		return "in_progress"
	}
}

func isTerminalDedicatedImageStatus(status string) bool {
	return status == service.ImageGenerationJobStatusCompleted || status == service.ImageGenerationJobStatusFailed || status == service.ImageGenerationJobStatusSubmissionUnknown
}

func isDedicatedCangyuanModel(model string) bool {
	switch strings.TrimSpace(model) {
	case service.CangyuanImageModel1K, service.CangyuanImageModel2K, service.CangyuanImageModel4K:
		return true
	default:
		return false
	}
}

func dedicatedImageModelTier(model string) string {
	switch strings.TrimSpace(model) {
	case service.CangyuanImageModel1K:
		return "1K"
	case service.CangyuanImageModel2K:
		return "2K"
	case service.CangyuanImageModel4K:
		return "4K"
	default:
		return ""
	}
}

func dedicatedImageTaskPath(c *gin.Context, jobID string) string {
	prefix := ""
	if c != nil && c.Request != nil && strings.HasPrefix(c.Request.URL.Path, "/v1/") {
		prefix = "/v1"
	}
	return prefix + "/images/tasks/" + jobID
}

func dedicatedImageContentPath(c *gin.Context, jobID string) string {
	return dedicatedImageTaskPath(c, jobID) + "/content"
}

func imageJobString(value *string, fallback string) string {
	if value != nil && strings.TrimSpace(*value) != "" {
		return strings.TrimSpace(*value)
	}
	return fallback
}

func imageContentExtension(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func restoreRequestBody(c *gin.Context, body []byte) {
	if c == nil || c.Request == nil {
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
}

// dedicatedImageIdempotencyKey keeps explicit client keys authoritative and
// falls back to the request correlation ID injected by the gateway middleware.
// The operation namespace prevents a generation and an edit sharing one
// correlation ID from accidentally replaying each other.
func dedicatedImageIdempotencyKey(c *gin.Context, operation string) string {
	if c == nil {
		return ""
	}
	if key := strings.TrimSpace(c.GetHeader("Idempotency-Key")); key != "" {
		return key
	}
	requestID := strings.TrimSpace(c.Writer.Header().Get("X-Request-ID"))
	if requestID == "" {
		requestID = strings.TrimSpace(c.GetHeader("X-Request-ID"))
	}
	if requestID == "" {
		return ""
	}
	return "request:" + strings.TrimSpace(operation) + ":" + requestID
}

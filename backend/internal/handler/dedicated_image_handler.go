package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DedicatedImageHandler struct {
	orchestrator   *service.ImageGenerationOrchestrator
	repo           service.ImageGenerationJobRepository
	results        service.ImageGenerationResultReader
	payloads       service.ImageGenerationPayloadStore
	billing        *service.BillingService
	worker         *service.ImageGenerationWorkerRuntime
	openAI         *OpenAIGatewayHandler
	imageStorage   *service.ImageStorageSettingService
	enabled        bool
	forceCodexCLI  bool
	syncTimeout    time.Duration
	codexHeartbeat time.Duration
	maxReadBytes   int64
	cfg            *config.Config
}

func NewDedicatedImageHandler(
	orchestrator *service.ImageGenerationOrchestrator,
	repo service.ImageGenerationJobRepository,
	results service.ImageGenerationResultReader,
	payloads service.ImageGenerationPayloadStore,
	billing *service.BillingService,
	worker *service.ImageGenerationWorkerRuntime,
	openAI *OpenAIGatewayHandler,
	imageStorage *service.ImageStorageSettingService,
	cfg *config.Config,
) *DedicatedImageHandler {
	h := &DedicatedImageHandler{
		orchestrator: orchestrator, repo: repo, results: results, payloads: payloads, billing: billing,
		worker: worker, openAI: openAI, imageStorage: imageStorage,
		syncTimeout: 3 * time.Minute, codexHeartbeat: 15 * time.Second, maxReadBytes: 64 << 20,
	}
	if cfg != nil {
		h.enabled = cfg.DedicatedImage.Enabled
		h.cfg = cfg
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

func (h *DedicatedImageHandler) runtimeEnabled() bool {
	if h == nil {
		return false
	}
	if h.cfg != nil {
		return h.cfg.DedicatedImageRuntime().Enabled
	}
	return h.enabled
}

func (h *DedicatedImageHandler) generalFallbackEnabled() bool {
	return h != nil && h.cfg != nil && h.cfg.DedicatedImageRuntime().FallbackToGeneral
}

// Dispatch sends explicit Cangyuan tier models and compatible GPT image aliases
// through the durable dedicated path. Every other request is restored
// byte-for-byte and delegated to the existing Images implementation.
func (h *DedicatedImageHandler) Dispatch(c *gin.Context, fallback gin.HandlerFunc) {
	if h == nil || !h.runtimeEnabled() || h.openAI == nil || h.openAI.gatewayService == nil {
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
	if isDedicatedCangyuanModel(parsed.Model) {
		if err := normalizeExplicitDedicatedImageRequest(parsed); err != nil {
			h.writeServiceError(c, err)
			return
		}
	} else {
		matched, normalizeErr := normalizeDedicatedImageAliasRequest(parsed)
		if normalizeErr != nil {
			if h.generalFallbackEnabled() {
				fallback(c)
				return
			}
			h.writeServiceError(c, normalizeErr)
			return
		}
		if !matched {
			fallback(c)
			return
		}
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
	if codexNativeImage {
		configureCodexNativeImageDelivery(&request)
	} else if err := h.enforceResponseFormatPolicy(c.Request.Context(), request.ResponseFormat); err != nil {
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
			h.writeCompletedImage(c, current, codexNativeImage)
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

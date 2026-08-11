package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type imageWorkbenchCreateRequest struct {
	APIKeyID       int64    `json:"api_key_id" binding:"required"`
	Operation      string   `json:"operation"`
	Model          string   `json:"model" binding:"required"`
	Prompt         string   `json:"prompt" binding:"required"`
	Size           string   `json:"size"`
	AspectRatio    string   `json:"aspect_ratio"`
	Quality        string   `json:"quality"`
	ResponseFormat string   `json:"response_format"`
	Images         []string `json:"images"`
	Mask           string   `json:"mask"`
}

type imageWorkbenchEstimateRequest struct {
	APIKeyID int64  `json:"api_key_id" binding:"required"`
	Model    string `json:"model" binding:"required"`
}

type imageWorkbenchRenameRequest struct {
	Name string `json:"name"`
}

// EstimateWorkbenchCost returns the current non-binding price snapshot for a
// workbench submission. It never creates a job, holds balance, or contacts the
// upstream image provider.
func (h *DedicatedImageHandler) EstimateWorkbenchCost(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		h.writeError(c, http.StatusUnauthorized, "authentication_error", "User not authenticated")
		return
	}
	if h == nil || !h.enabled || h.worker == nil || !h.worker.Running() || h.openAI == nil || h.openAI.apiKeyService == nil || h.billing == nil {
		h.writeError(c, http.StatusServiceUnavailable, "image_orchestration_unavailable", "image workbench is not enabled")
		return
	}
	var input imageWorkbenchEstimateRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		h.writeError(c, http.StatusBadRequest, "invalid_request_error", "invalid image workbench estimate request")
		return
	}
	if !isDedicatedCangyuanModel(input.Model) {
		h.writeError(c, http.StatusBadRequest, "image_model_not_allowed", "workbench model must be a dedicated 1K, 2K, or 4K Cangyuan model")
		return
	}
	apiKey, err := h.openAI.apiKeyService.GetByID(c.Request.Context(), input.APIKeyID)
	if err != nil || !workbenchAPIKeyEligible(subject.UserID, apiKey) {
		// Keep ownership and key availability indistinguishable to the panel.
		h.writeError(c, http.StatusNotFound, "image_task_not_found", "API key is not available for image generation")
		return
	}
	tier := dedicatedImageModelTier(input.Model)
	estimate, err := service.EstimateDedicatedImageCostSnapshot(h.billing, apiKey, input.Model, tier)
	if err != nil {
		h.writeError(c, http.StatusServiceUnavailable, "image_orchestration_unavailable", "image price could not be resolved")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"model":           input.Model,
		"size_tier":       tier,
		"base_cost":       estimate.BaseCost,
		"rate_multiplier": estimate.RateMultiplier,
		"estimated_cost":  estimate.ActualCost,
	})
}

func (h *DedicatedImageHandler) CreateWorkbenchJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		h.writeError(c, http.StatusUnauthorized, "authentication_error", "User not authenticated")
		return
	}
	if h == nil || !h.enabled || h.worker == nil || !h.worker.Running() || h.openAI == nil || h.openAI.apiKeyService == nil {
		h.writeError(c, http.StatusServiceUnavailable, "image_orchestration_unavailable", "image workbench is not enabled")
		return
	}
	var input imageWorkbenchCreateRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		h.writeError(c, http.StatusBadRequest, "invalid_request_error", "invalid image workbench request")
		return
	}
	apiKey, err := h.openAI.apiKeyService.GetByID(c.Request.Context(), input.APIKeyID)
	if err != nil || !workbenchAPIKeyEligible(subject.UserID, apiKey) {
		h.writeError(c, http.StatusNotFound, "image_task_not_found", "API key is not available for image generation")
		return
	}
	var subscription *service.UserSubscription
	if apiKey.Group.IsSubscriptionType() {
		if apiKey.GroupID == nil {
			h.writeError(c, http.StatusNotFound, "image_task_not_found", "API key is not available for image generation")
			return
		}
		subscription, err = h.openAI.apiKeyService.GetActiveSubscription(c.Request.Context(), subject.UserID, *apiKey.GroupID)
		if err != nil || subscription == nil {
			h.writeError(c, http.StatusNotFound, "image_task_not_found", "API key is not available for image generation")
			return
		}
	}
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
	if !isDedicatedCangyuanModel(input.Model) {
		h.writeError(c, http.StatusBadRequest, "image_model_not_allowed", "workbench model must be a dedicated 1K, 2K, or 4K Cangyuan model")
		return
	}
	operation := strings.TrimSpace(input.Operation)
	if operation == "" {
		operation = service.ImageGenerationJobOperationGeneration
		if len(input.Images) > 0 || strings.TrimSpace(input.Mask) != "" {
			operation = service.ImageGenerationJobOperationEdit
		}
	}
	if operation != service.ImageGenerationJobOperationGeneration && operation != service.ImageGenerationJobOperationEdit {
		h.writeError(c, http.StatusBadRequest, "invalid_request_error", "operation must be generation or edit")
		return
	}
	tier := dedicatedImageModelTier(input.Model)
	request := service.CangyuanImageRequest{
		Model: input.Model, Prompt: input.Prompt, Size: input.Size, AspectRatio: input.AspectRatio, N: 1,
		Quality: input.Quality, ResponseFormat: input.ResponseFormat, Async: true,
		ImageSize: tier, OutputResolution: tier,
		Images: append([]string(nil), input.Images...), Mask: input.Mask,
	}
	if strings.TrimSpace(request.ResponseFormat) == "" {
		request.ResponseFormat = "url"
	}
	if err := h.enforceResponseFormatPolicy(c.Request.Context(), request.ResponseFormat); err != nil {
		h.writeServiceError(c, err)
		return
	}
	if err := service.ValidateCangyuanImageRequest(service.CangyuanImageOperation(operation), request); err != nil {
		h.writeServiceError(c, err)
		return
	}
	moderationBody, _ := json.Marshal(gin.H{"prompt": input.Prompt, "images": input.Images})
	reqLog := requestLogger(c, "handler.image_workbench", dedicatedImageLogFields(
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.String("model", input.Model),
	)...)
	if decision := h.openAI.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, input.Model, moderationBody); decision != nil && !decision.AllowNextStage {
		h.openAI.openAISecurityAuditError(c, decision)
		return
	}
	var selectedGroupID int64
	if apiKey.GroupID != nil {
		selectedGroupID = *apiKey.GroupID
	} else if apiKey.Group != nil {
		selectedGroupID = apiKey.Group.ID
	}
	imageRelease, acquired := h.openAI.acquireImageGenerationSlotForIdentity(c, false, input.Model, subject.UserID, apiKey.ID, selectedGroupID)
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
	costEstimate, err := service.EstimateDedicatedImageCostSnapshot(h.billing, apiKey, input.Model, tier)
	if err != nil {
		h.writeError(c, http.StatusServiceUnavailable, "image_orchestration_unavailable", "image price could not be resolved")
		return
	}
	job, _, err := h.orchestrator.Create(c.Request.Context(), service.CreateDedicatedImageJobParams{
		UserID: subject.UserID, APIKeyID: apiKey.ID, GroupID: dedicatedImageGroupID(apiKey),
		SubscriptionID: imageWorkbenchSubscriptionID(subscription), BillingType: imageWorkbenchBillingType(subscription),
		Source: service.ImageGenerationJobSourceWorkbench, Operation: operation,
		PublicModel: input.Model, DisplayName: input.Prompt, Request: request,
		IdempotencyKey: dedicatedImageIdempotencyKey(c, operation),
		BaseCost:       costEstimate.BaseCost, RateMultiplier: costEstimate.RateMultiplier, EstimatedCost: costEstimate.ActualCost,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	// The Worker acquires the job's execution dimensions. Do not keep the
	// inbound workbench admission while the asynchronous job starts.
	if imageRelease != nil {
		imageRelease()
		imageRelease = nil
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Retry-After", "2")
	c.JSON(http.StatusAccepted, workbenchImageJobResponse(job))
}

func imageWorkbenchSubscriptionID(subscription *service.UserSubscription) *int64 {
	if subscription == nil {
		return nil
	}
	return &subscription.ID
}

func imageWorkbenchBillingType(subscription *service.UserSubscription) int8 {
	if subscription != nil {
		return service.BillingTypeSubscription
	}
	return service.BillingTypeBalance
}

func workbenchAPIKeyEligible(userID int64, apiKey *service.APIKey) bool {
	return userID > 0 &&
		apiKey != nil &&
		apiKey.UserID == userID &&
		apiKey.IsActive() &&
		!apiKey.IsExpired() &&
		apiKey.Group != nil &&
		apiKey.Group.Platform == service.PlatformOpenAI &&
		service.GroupAllowsImageGeneration(apiKey.Group)
}

func (h *DedicatedImageHandler) ListWorkbenchJobs(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		h.writeError(c, http.StatusUnauthorized, "authentication_error", "User not authenticated")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	status := strings.TrimSpace(c.Query("status"))
	if !validWorkbenchStatusFilter(status) {
		h.writeError(c, http.StatusBadRequest, "invalid_request_error", "invalid image workbench status filter")
		return
	}
	operation := strings.TrimSpace(c.Query("operation"))
	if operation != "" && operation != service.ImageGenerationJobOperationGeneration && operation != service.ImageGenerationJobOperationEdit {
		h.writeError(c, http.StatusBadRequest, "invalid_request_error", "invalid image workbench operation filter")
		return
	}
	jobs, err := h.repo.ListImageGenerationJobsForOwner(c.Request.Context(), subject.UserID, service.ImageGenerationJobFilter{
		Source: service.ImageGenerationJobSourceWorkbench,
		Status: status, Operation: operation, Limit: limit, Offset: offset,
	})
	if err != nil {
		h.writeError(c, http.StatusServiceUnavailable, "image_orchestration_unavailable", "image jobs could not be listed")
		return
	}
	items := make([]gin.H, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, workbenchImageJobResponse(job))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": items, "limit": limit, "offset": offset})
}

func validWorkbenchStatusFilter(status string) bool {
	switch status {
	case "", "queued", "in_progress", "completed", "failed", "submission_unknown":
		return true
	default:
		return false
	}
}

func (h *DedicatedImageHandler) GetWorkbenchJob(c *gin.Context) {
	job, ok := h.workbenchOwnedJob(c)
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	if !isTerminalDedicatedImageStatus(job.Status) {
		c.Header("Retry-After", "2")
	}
	c.JSON(http.StatusOK, workbenchImageJobResponse(job))
}

func (h *DedicatedImageHandler) RenameWorkbenchJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		h.writeError(c, http.StatusUnauthorized, "authentication_error", "User not authenticated")
		return
	}
	var input imageWorkbenchRenameRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		h.writeError(c, http.StatusBadRequest, "invalid_request_error", "invalid image workbench rename request")
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > 80 {
		h.writeError(c, http.StatusBadRequest, "invalid_request_error", "artwork name must contain 1 to 80 characters")
		return
	}
	job, err := h.repo.RenameImageGenerationJobForUser(c.Request.Context(), subject.UserID, c.Param("id"), name)
	if err != nil || job == nil {
		h.writeError(c, http.StatusNotFound, "image_task_not_found", "image task not found")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, workbenchImageJobResponse(job))
}

func (h *DedicatedImageHandler) WorkbenchContent(c *gin.Context) {
	job, ok := h.workbenchOwnedJob(c)
	if !ok {
		return
	}
	if job.Status != service.ImageGenerationJobStatusCompleted || len(job.ResultObjectRefs) == 0 {
		h.writeError(c, http.StatusConflict, "image_task_not_ready", "image task result is not ready")
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

func (h *DedicatedImageHandler) workbenchOwnedJob(c *gin.Context) (*service.ImageGenerationJob, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		h.writeError(c, http.StatusUnauthorized, "authentication_error", "User not authenticated")
		return nil, false
	}
	job, err := h.repo.GetImageGenerationJobForUser(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil || job == nil || job.Source != service.ImageGenerationJobSourceWorkbench {
		h.writeError(c, http.StatusNotFound, "image_task_not_found", "image task not found")
		return nil, false
	}
	return job, true
}

func workbenchImageJobResponse(job *service.ImageGenerationJob) gin.H {
	response := gin.H{
		"id": job.JobID, "status": publicDedicatedImageStatus(job.Status),
		"operation": job.Operation, "model": job.PublicModel, "name": imageJobString(job.DisplayName, ""),
		"requested_size": imageJobString(job.RequestedSize, ""), "actual_size": imageJobString(job.ActualSize, ""),
		"quality":        imageJobString(job.Quality, "auto"),
		"estimated_cost": job.EstimatedCost, "settled_cost": job.SettledCost,
		"created_at": job.CreatedAt, "updated_at": job.UpdatedAt,
	}
	if job.Status == service.ImageGenerationJobStatusCompleted {
		response["content_url"] = "/api/v1/user/image-workbench/jobs/" + job.JobID + "/content"
	}
	if job.Status == service.ImageGenerationJobStatusFailed || job.Status == service.ImageGenerationJobStatusSubmissionUnknown {
		response["error"] = gin.H{"code": imageJobString(job.ErrorCode, "image_upstream_rejected"), "message": imageJobString(job.ErrorMessage, "image generation failed")}
	}
	return response
}

package handler

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *DedicatedImageHandler) writeCompletedImage(c *gin.Context, job *service.ImageGenerationJob, codexNativeImage bool) {
	created := job.CreatedAt.Unix()
	if codexNativeImage {
		if h.payloads == nil || job.PayloadObjectRef == nil {
			h.writeError(c, http.StatusBadGateway, "image_result_payload_missing", "Codex image output could not be delivered")
			return
		}
		payload, err := h.payloads.Get(c.Request.Context(), *job.PayloadObjectRef)
		if err != nil || payload == nil || payload.CodexResult == nil || strings.TrimSpace(payload.CodexResult.Base64) == "" {
			h.writeError(c, http.StatusBadGateway, "image_result_payload_missing", "Codex image output could not be delivered")
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"created": created, "data": []gin.H{{"b64_json": payload.CodexResult.Base64}}})
		return
	}
	if imageJobString(job.ResponseFormat, "url") == "b64_json" && h.imageStorage != nil && h.imageStorage.Base64ResponsesAllowed(c.Request.Context()) {
		if h.results == nil || len(job.ResultObjectRefs) == 0 {
			h.writeError(c, http.StatusBadGateway, "image_result_payload_missing", "image result could not be delivered")
			return
		}
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

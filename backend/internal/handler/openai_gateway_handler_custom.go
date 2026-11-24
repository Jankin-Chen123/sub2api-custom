package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// openAIDedicatedImagePlanningCapability is intentionally chat-completions:
// the dedicated bridge performs image execution itself, so its general account
// only needs to understand the planner function. Native Responses accounts are
// still eligible because Responses capability is a strict subset of the
// account's chat capability, while Chat Completions-only API-key accounts are
// served through the existing Responses-to-Chat compatibility bridge.
func openAIDedicatedImagePlanningCapability() service.OpenAIEndpointCapability {
	return service.OpenAIEndpointCapabilityChatCompletions
}

func isOpenAIRemoteCompactPath(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	normalizedPath := strings.TrimRight(strings.TrimSpace(c.Request.URL.Path), "/")
	return strings.HasSuffix(normalizedPath, "/responses/compact")
}

func isOpenAIRemoteCompactionV2Request(arg any, rest ...[]byte) bool {
	var body []byte
	if ctx, ok := arg.(*gin.Context); ok {
		_ = ctx // retained for source compatibility with the context-aware call site
		if len(rest) == 0 {
			return false
		}
		body = rest[0]
	} else if payload, ok := arg.([]byte); ok {
		body = payload
	}
	stream, valid := parseOpenAICompatibleStream(body)
	if !valid || !stream {
		return false
	}
	// The streaming body signal is the canonical remote-compaction-v2 marker.
	// Header/user-agent hints are optional and must not force a legacy
	// /responses/compact rewrite when the request is already a streamed
	// compaction turn.
	return service.HasCompactionTriggerInInput(body)
}

func (h *OpenAIGatewayHandler) acquireImageGenerationSlot(c *gin.Context, streamStarted bool, models ...string) (func(), bool) {
	if h == nil {
		return nil, true
	}
	model := ""
	if len(models) > 0 {
		model = models[0]
	}
	var release func()
	acquired := true
	if h.cfg != nil && h.imageLimiter != nil {
		imageConcurrency := h.cfg.Gateway.ImageConcurrency
		wait := strings.TrimSpace(imageConcurrency.OverflowMode) == config.ImageConcurrencyOverflowModeWait
		release, acquired = h.imageLimiter.Acquire(
			c.Request.Context(),
			imageConcurrency.Enabled,
			imageConcurrency.MaxConcurrentRequests,
			wait,
			time.Duration(imageConcurrency.WaitTimeoutSeconds)*time.Second,
			imageConcurrency.MaxWaitingRequests,
		)
	}
	if acquired {
		apiKey, _ := middleware2.GetAPIKeyFromContext(c)
		subject, _ := middleware2.GetAuthSubjectFromContext(c)
		var userID, apiKeyID, groupID int64
		if subject.UserID > 0 {
			userID = subject.UserID
		}
		if apiKey != nil {
			apiKeyID = apiKey.ID
			if apiKey.GroupID != nil {
				groupID = *apiKey.GroupID
			} else if apiKey.Group != nil {
				groupID = apiKey.Group.ID
			}
			if userID <= 0 {
				userID = apiKey.UserID
			}
		}
		return h.acquireImageGenerationAdmission(c, release, model, userID, apiKeyID, groupID, 0, streamStarted)
	}
	c.Header("Retry-After", "2")
	h.handleStreamingAwareErrorWithCode(c, http.StatusTooManyRequests, "rate_limit_error", gatewayConcurrencyLimitCode, "Image generation concurrency limit exceeded, please retry later", streamStarted, false)
	return nil, false
}

func (h *OpenAIGatewayHandler) acquireImageGenerationSlotForIdentity(
	c *gin.Context, streamStarted bool, model string, userID, apiKeyID, groupID int64,
) (func(), bool) {
	if h == nil {
		return nil, true
	}
	var release func()
	acquired := true
	if h.cfg != nil && h.imageLimiter != nil {
		imageConcurrency := h.cfg.Gateway.ImageConcurrency
		wait := strings.TrimSpace(imageConcurrency.OverflowMode) == config.ImageConcurrencyOverflowModeWait
		release, acquired = h.imageLimiter.Acquire(
			c.Request.Context(), imageConcurrency.Enabled, imageConcurrency.MaxConcurrentRequests,
			wait, time.Duration(imageConcurrency.WaitTimeoutSeconds)*time.Second,
			imageConcurrency.MaxWaitingRequests,
		)
	}
	if !acquired {
		c.Header("Retry-After", "2")
		h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Image generation concurrency limit exceeded, please retry later", streamStarted)
		return nil, false
	}
	return h.acquireImageGenerationAdmission(c, release, model, userID, apiKeyID, groupID, 0, streamStarted)
}

func (h *OpenAIGatewayHandler) acquireImageGenerationAdmission(
	c *gin.Context, release func(), model string, userID, apiKeyID, groupID, accountID int64, streamStarted bool,
) (func(), bool) {
	if h == nil || h.imageAdmission == nil {
		return release, true
	}
	tier := ""
	if strings.Contains(strings.ToLower(strings.TrimSpace(model)), "4k") {
		tier = service.ImageConcurrencyTier4K
	}
	admissionRelease, acquired, err := h.imageAdmission.Acquire(c.Request.Context(), service.ImageGenerationAdmissionRequest{
		UserID: userID, APIKeyID: apiKeyID, GroupID: groupID, AccountID: accountID, Tier: tier,
	})
	if err != nil {
		if release != nil {
			release()
		}
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "rate_limit_error", "Image concurrency service is temporarily unavailable, please retry later", streamStarted)
		return nil, false
	}
	if !acquired {
		if release != nil {
			release()
		}
		c.Header("Retry-After", "2")
		h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Image generation concurrency limit exceeded, please retry later", streamStarted)
		return nil, false
	}
	if release == nil {
		return admissionRelease, true
	}
	if admissionRelease == nil {
		return release, true
	}
	return func() {
		admissionRelease()
		release()
	}, true
}

package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const (
	codexDedicatedImagePlannerToolName = "sub2api_generate_image"
	codexDedicatedImagePlannerMarker   = "codex_dedicated_image_planner"
	codexDedicatedImageResponsePrefix  = "resp_img_"
)

var ErrCodexDedicatedImageReplayNotFound = errors.New("codex dedicated image replay not found")
var ErrCodexDedicatedImageReplayCorrupt = errors.New("codex dedicated image replay is corrupt")

// CodexDedicatedImageReplayStore is the optional cross-instance backing store
// for synthetic Responses IDs. The local map remains the hot path; a Redis
// implementation lets a later request land on another application instance.
// The raw value is an internal, redacted replay record and must never be
// exposed through a public API.
type CodexDedicatedImageReplayStore interface {
	GetCodexDedicatedImageReplay(ctx context.Context, responseID string) ([]byte, error)
	SetCodexDedicatedImageReplay(ctx context.Context, responseID string, value []byte, ttl time.Duration) error
	DeleteCodexDedicatedImageReplay(ctx context.Context, responseID string) error
}

func isCodexDedicatedImagePlannerContext(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, ok := c.Get(codexDedicatedImagePlannerMarker)
	flag, _ := value.(bool)
	return ok && flag
}

// CodexDedicatedImageBridge is the two-stage Codex path:
//
//  1. a normal OpenAI-compatible account receives the complete Responses
//     context and decides whether to call the private planner tool;
//  2. the generated, self-contained plan becomes a durable image job whose
//     execution is pinned to an image_only account.
//
// The bridge is deliberately separate from the existing image_generation
// passthrough logic. It is disabled unless dedicated_image.codex_bridge_enabled
// is explicitly enabled in addition to the durable worker.
type CodexDedicatedImageBridge struct {
	gateway       *OpenAIGatewayService
	orchestrator  *ImageGenerationOrchestrator
	repo          ImageGenerationJobRepository
	results       ImageGenerationResultReader
	payloads      ImageGenerationPayloadStore
	queue         *ImageGenerationQueueController
	billing       *BillingService
	worker        *ImageGenerationWorkerRuntime
	enabled       bool
	forceCodexCLI bool
	syncTimeout   time.Duration
	sseKeepalive  time.Duration
	maxReadBytes  int64
	cfg           *config.Config
	replayStore   CodexDedicatedImageReplayStore
	replayMu      sync.RWMutex
	replays       map[string]codexDedicatedImageReplay
}

// codexDedicatedImageReplay connects the public synthetic response id to the
// real planner response id. It also carries the private tool result that must
// be supplied on the next HTTP turn; otherwise a later previous_response_id
// would point at a response that never existed at the planner upstream.
type codexDedicatedImageReplay struct {
	UpstreamResponseID string
	FunctionCallOutput json.RawMessage
	ExpiresAt          time.Time
}

func NewCodexDedicatedImageBridge(
	gateway *OpenAIGatewayService,
	orchestrator *ImageGenerationOrchestrator,
	repo ImageGenerationJobRepository,
	results ImageGenerationResultReader,
	payloads ImageGenerationPayloadStore,
	queue *ImageGenerationQueueController,
	billing *BillingService,
	worker *ImageGenerationWorkerRuntime,
	cfg *config.Config,
) *CodexDedicatedImageBridge {
	b := &CodexDedicatedImageBridge{
		gateway: gateway, orchestrator: orchestrator, repo: repo, results: results, payloads: payloads, queue: queue,
		billing: billing, worker: worker, syncTimeout: 3 * time.Minute,
		sseKeepalive: 10 * time.Second,
		maxReadBytes: 64 << 20,
		replays:      make(map[string]codexDedicatedImageReplay),
	}
	if gateway != nil && gateway.cache != nil {
		if store, ok := gateway.cache.(CodexDedicatedImageReplayStore); ok {
			b.replayStore = store
		}
	}
	if cfg != nil {
		b.cfg = cfg
		b.enabled = cfg.DedicatedImage.Enabled && cfg.DedicatedImage.WorkerEnabled && cfg.DedicatedImage.CodexBridgeEnabled
		b.forceCodexCLI = cfg.Gateway.ForceCodexCLI
		if cfg.DedicatedImage.SyncWaitTimeoutSeconds > 0 {
			b.syncTimeout = time.Duration(cfg.DedicatedImage.SyncWaitTimeoutSeconds) * time.Second
		}
		if cfg.DedicatedImage.MaxOutputBytes > 0 {
			b.maxReadBytes = cfg.DedicatedImage.MaxOutputBytes
		}
		// The gateway setting defaults to 10 seconds. Preserve its documented
		// zero=disabled behavior when an explicit Config is supplied.
		b.sseKeepalive = time.Duration(cfg.Gateway.ImageStreamKeepaliveInterval) * time.Second
	}
	return b
}

func (b *CodexDedicatedImageBridge) runtimeEnabled() bool {
	if b == nil {
		return false
	}
	if b.cfg != nil {
		settings := b.cfg.DedicatedImageRuntime()
		return settings.Enabled && settings.WorkerEnabled && settings.CodexBridgeEnabled
	}
	return b.enabled
}

func (b *CodexDedicatedImageBridge) shouldHandleCommon(c *gin.Context, body []byte, apiKey *APIKey, requireImageDeclaration bool) bool {
	if b == nil || !b.runtimeEnabled() || b.gateway == nil || b.worker == nil || !b.worker.Running() || apiKey == nil {
		return false
	}
	if apiKey.Group != nil {
		if apiKey.Group.Platform != PlatformOpenAI || !GroupAllowsImageGeneration(apiKey.Group) {
			return false
		}
	}
	if c == nil || c.Request == nil {
		return false
	}
	if c.Request.URL != nil && isOpenAIResponsesLiteHeader(c.GetHeader(responsesLiteHeader)) {
		return false
	}
	if isOpenAIResponsesLiteWebSocketPayload(body) {
		return false
	}
	if !b.forceCodexCLI && !openai.IsCodexOfficialClientByHeaders(c.GetHeader("User-Agent"), c.GetHeader("originator")) {
		return false
	}
	return !requireImageDeclaration || openAIRequestBodyHasImageGenerationDeclaration(body)
}

// ShouldHandle recognizes an official Codex Responses session whose group has
// image-generation permission. The client is not required to declare a
// native image_generation tool: the real Codex CLI may omit that declaration,
// while a later turn can still explicitly request an image. Forward injects a
// private planner tool into the general-account request and the planner decides
// whether the current turn actually needs an image, so keyword matching is
// avoided. The feature flag and group permission remain the hard gates.
func (b *CodexDedicatedImageBridge) ShouldHandle(c *gin.Context, body []byte, apiKey *APIKey) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil || !isBareCodexResponsesPath(c.Request.URL.Path) {
		return false
	}
	// A synthetic image response must remain routable even when a later
	// ordinary text turn omits Codex's passive image namespace declaration.
	// Otherwise the general account would receive an unknown synthetic
	// previous_response_id and the conversation would break after an image.
	// The same rule also applies to the first ordinary turn: the client may not
	// advertise image_generation until it needs it, so the bridge must be
	// selected for the whole eligible Responses conversation.
	return b.shouldHandleCommon(c, body, apiKey, false)
}

// AllowsHTTPContinuation reports whether the HTTP Responses handler may let
// this request pass its compatibility guard. The request must still be an
// official Codex request handled by the enabled bridge. This covers both a
// real planner response ID on the first image turn and a synthetic resp_img_*
// ID on the ordinary follow-up after an image; ordinary HTTP continuations do
// not become generally enabled by this method.
func (b *CodexDedicatedImageBridge) AllowsHTTPContinuation(c *gin.Context, body []byte, apiKey *APIKey) bool {
	return b.ShouldHandle(c, body, apiKey)
}

// ShouldHandleWebSocket enables the HTTP bridge for the whole official Codex
// WebSocket session when dedicated routing is enabled. A session may start as
// ordinary text and request an image several turns later, so checking only the
// first frame's image declaration would make the transport choice irreversible
// and lose the later image request.
func (b *CodexDedicatedImageBridge) ShouldHandleWebSocket(c *gin.Context, body []byte, apiKey *APIKey) bool {
	return b.shouldHandleCommon(c, body, apiKey, false)
}

// Forward runs the planner using the already selected general account. A
// normal planner response is replayed unchanged. When the private tool is
// called, the planner response is suppressed and replaced with a synthetic
// Responses image_generation_call containing the durable job result.
func (b *CodexDedicatedImageBridge) Forward(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	apiKey *APIKey,
	subscription *UserSubscription,
	body []byte,
) (*OpenAIForwardResult, error) {
	if b == nil || b.gateway == nil || account == nil || apiKey == nil {
		return nil, errors.New("codex dedicated image bridge is not configured")
	}
	if c == nil {
		return nil, errors.New("codex image result context is unavailable")
	}
	stopSSEKeepalive := func() {}
	if codexDedicatedImageRequestStream(body) {
		stopSSEKeepalive = StartCodexDedicatedImageSSEKeepalive(c, b.sseKeepalive)
	}
	defer stopSSEKeepalive()
	plannerSourceBody, err := b.resolveDedicatedImageReplay(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("resolve Codex image replay: %w", err)
	}
	plannerBody, err := buildCodexDedicatedImagePlannerBody(plannerSourceBody)
	if err != nil {
		return nil, err
	}

	recorder := httptest.NewRecorder()
	plannerCtx, err := cloneGinContextForCodexPlanner(c, ctx, plannerBody, recorder)
	if err != nil {
		return nil, err
	}
	plannerCtx.Set(codexDedicatedImagePlannerMarker, true)
	plannerResult, forwardErr := b.gateway.Forward(ctx, plannerCtx, account, plannerBody)
	if forwardErr != nil {
		return plannerResult, forwardErr
	}
	if plannerResult == nil {
		return nil, errors.New("codex planner returned no forwarding result")
	}

	plan, found, err := extractCodexDedicatedImagePlan(recorder.Body.Bytes(), plannerBody)
	if err != nil {
		return plannerResult, err
	}
	if !found {
		if err := replayCodexPlannerResponse(c, recorder); err != nil {
			return plannerResult, err
		}
		return plannerResult, nil
	}
	job, err := b.createCodexImageJob(ctx, c, apiKey, subscription, plan, body)
	if err != nil {
		return plannerResult, err
	}
	completed, err := b.waitForCompletion(ctx, apiKey, job)
	if err != nil {
		return plannerResult, err
	}
	responseID, response, item, err := b.buildCodexImageResponse(ctx, plannerResult, completed, plan)
	if err != nil {
		b.finishCodexImageDelivery(completed, false)
		return plannerResult, err
	}
	// Persist the synthetic response replay before writing any bytes. A client
	// may receive the response ID even when its connection drops immediately;
	// the next turn must be able to resolve that ID across instances.
	if err := b.rememberDedicatedImageReplay(ctx, responseID, plannerResult.ResponseID, plan, codexDedicatedImageGroupID(apiKey), account.ID); err != nil {
		b.finishCodexImageDelivery(completed, false)
		return plannerResult, err
	}
	delivered := false
	defer func() { b.finishCodexImageDelivery(completed, delivered) }()
	if plannerResult.Stream {
		if err := writeCodexDedicatedImageSSE(c, response, item); err != nil {
			return plannerResult, err
		}
	} else {
		c.Header("Content-Type", "application/json")
		c.Header("Cache-Control", "no-cache")
		if err := writeJSONBytes(c, response); err != nil {
			return plannerResult, err
		}
	}
	delivered = true
	plannerResult.ResponseID = responseID
	plannerResult.RequestID = responseID
	plannerResult.ImageCount = 0 // image billing is settled by the durable job.
	plannerResult.ResponseHeaders = c.Writer.Header().Clone()
	return plannerResult, nil
}

// ForwardWebSocket plans a single Responses WebSocket turn through the same
// general text account used by HTTP/SSE and returns protocol event payloads for
// the WebSocket relay. The image-only account is used only by the durable job;
// it never receives the conversation context.
func (b *CodexDedicatedImageBridge) ForwardWebSocket(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	apiKey *APIKey,
	subscription *UserSubscription,
	body []byte,
) (*OpenAIForwardResult, []json.RawMessage, error) {
	if b == nil || b.gateway == nil || account == nil || apiKey == nil {
		return nil, nil, errors.New("codex dedicated image bridge is not configured")
	}
	plannerSourceBody, err := b.resolveDedicatedImageReplay(ctx, body)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve Codex image replay: %w", err)
	}
	plannerBody, err := buildCodexDedicatedImagePlannerBody(plannerSourceBody)
	if err != nil {
		return nil, nil, err
	}
	// A Responses WebSocket frame is not itself an HTTP Responses request:
	// remove the frame envelope and force streaming so the planner result can
	// be relayed back as WebSocket event payloads. Replay input, when present,
	// remains in the body and carries the prior conversation context.
	plannerBody, err = prepareCodexDedicatedImagePlannerHTTPBody(plannerBody)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare Codex planner HTTP body: %w", err)
	}
	recorder := httptest.NewRecorder()
	plannerCtx, err := cloneGinContextForCodexPlanner(c, ctx, plannerBody, recorder)
	if err != nil {
		return nil, nil, err
	}
	plannerCtx.Set(codexDedicatedImagePlannerMarker, true)
	plannerResult, forwardErr := b.gateway.Forward(ctx, plannerCtx, account, plannerBody)
	if forwardErr != nil {
		return plannerResult, nil, forwardErr
	}
	if plannerResult == nil {
		return nil, nil, errors.New("codex planner returned no forwarding result")
	}
	plannerResult.OpenAIWSMode = true

	plan, found, err := extractCodexDedicatedImagePlan(recorder.Body.Bytes(), plannerBody)
	if err != nil {
		return plannerResult, nil, err
	}
	if !found {
		events, err := extractCodexPlannerEventPayloads(recorder.Body.Bytes(), plannerBody)
		if err != nil {
			return plannerResult, nil, err
		}
		replayInput, replayErr := extractCodexPlannerReplayInput(recorder.Body.Bytes(), plannerBody)
		if replayErr != nil {
			return plannerResult, nil, replayErr
		}
		if len(replayInput) > 0 {
			plannerResult.wsReplayInput = replayInput
			plannerResult.wsReplayInputExists = true
		}
		return plannerResult, events, nil
	}
	job, err := b.createCodexImageJob(ctx, c, apiKey, subscription, plan, body)
	if err != nil {
		return plannerResult, nil, err
	}
	completed, err := b.waitForCompletion(ctx, apiKey, job)
	if err != nil {
		return plannerResult, nil, err
	}
	responseID, response, item, err := b.buildCodexImageResponse(ctx, plannerResult, completed, plan)
	if err != nil {
		b.finishCodexImageDelivery(completed, false)
		return plannerResult, nil, err
	}
	events, err := buildCodexDedicatedImageEventPayloads(response, item)
	if err != nil {
		b.finishCodexImageDelivery(completed, false)
		return plannerResult, nil, err
	}
	if err := b.rememberDedicatedImageReplay(ctx, responseID, plannerResult.ResponseID, plan, codexDedicatedImageGroupID(apiKey), account.ID); err != nil {
		b.finishCodexImageDelivery(completed, false)
		return plannerResult, nil, err
	}
	plannerResult.ResponseID = responseID
	plannerResult.RequestID = responseID
	plannerResult.ImageCount = 0
	plannerResult.wsReplayInput = []json.RawMessage{
		b.buildDedicatedImageFunctionCallContext(plan),
		b.buildDedicatedImageFunctionCallOutput(plan),
	}
	plannerResult.wsReplayInputExists = true
	plannerResult.ResponseHeaders = recorder.Header().Clone()
	plannerResult.codexImageDeliveryCleanup = func(delivered bool) {
		b.finishCodexImageDelivery(completed, delivered)
	}
	return plannerResult, events, nil
}

func (b *CodexDedicatedImageBridge) createCodexImageJob(
	ctx context.Context,
	c *gin.Context,
	apiKey *APIKey,
	subscription *UserSubscription,
	plan *codexDedicatedImagePlan,
	originalBody []byte,
) (*ImageGenerationJob, error) {
	if b.orchestrator == nil || b.billing == nil || apiKey == nil || plan == nil {
		return nil, errors.New("codex image job dependencies are unavailable")
	}
	billingType := BillingTypeBalance
	var subscriptionID *int64
	if subscription != nil && apiKey.Group != nil && apiKey.Group.IsSubscriptionType() {
		billingType = BillingTypeSubscription
		subscriptionID = &subscription.ID
	}
	tier := dedicatedImageTierForModel(plan.Model)
	cost, err := EstimateDedicatedImageCostSnapshot(b.billing, apiKey, plan.Model, tier)
	if err != nil {
		return nil, err
	}
	request := CangyuanImageRequest{
		Model: plan.Model, Prompt: plan.Prompt, Size: plan.Size, AspectRatio: plan.AspectRatio, N: 1,
		Quality: plan.Quality, ResponseFormat: "b64_json", Async: false,
		ImageSize: tier, OutputResolution: tier,
	}
	if err := ValidateCangyuanImageRequest(CangyuanImageOperationGeneration, request); err != nil {
		return nil, err
	}
	idempotencyKey := codexDedicatedImageIdempotencyKey(plan, originalBody)
	job, _, err := b.orchestrator.Create(ctx, CreateDedicatedImageJobParams{
		UserID: apiKey.UserID, APIKeyID: apiKey.ID, GroupID: codexDedicatedImageGroupID(apiKey),
		SubscriptionID: subscriptionID, BillingType: billingType,
		Source: ImageGenerationJobSourceCodex, Operation: ImageGenerationJobOperationGeneration,
		PublicModel: plan.Model, Request: request,
		IdempotencyKey: idempotencyKey,
		BaseCost:       cost.BaseCost, RateMultiplier: cost.RateMultiplier, EstimatedCost: cost.ActualCost,
	})
	if err != nil {
		return nil, err
	}
	_ = c // kept in the signature for future per-request audit metadata.
	return job, nil
}

// codexDedicatedImageIdempotencyKey prefers the planner tool call ID because a
// reconnect may resend the same tool call with a different surrounding
// request envelope (stream flags, replay input, or transport metadata). When
// a client/upstream omits a call ID, the complete request hash remains the
// conservative fallback: identical requests still deduplicate, while two
// independent calls are not collapsed onto the generic tool name.
func codexDedicatedImageIdempotencyKey(plan *codexDedicatedImagePlan, originalBody []byte) string {
	if plan != nil {
		callID := strings.TrimSpace(plan.CallID)
		if callID != "" && callID != codexDedicatedImagePlannerToolName {
			callSum := sha256.Sum256([]byte(callID))
			// Call IDs are intended to be stable across transport retries, but
			// an upstream/client may reuse one across independent sessions. The
			// plan fingerprint keeps a retry idempotent while preventing a new
			// image request with the same call ID from colliding with an older
			// durable job and being reported as a parameter conflict.
			if planBytes, err := json.Marshal(plan); err == nil {
				planSum := sha256.Sum256(planBytes)
				return "codex_call_" + fmt.Sprintf("%x_%x", callSum[:], planSum[:])
			}
			return "codex_call_" + fmt.Sprintf("%x", callSum[:])
		}
	}
	sum := sha256.Sum256(originalBody)
	return "codex_request_" + fmt.Sprintf("%x", sum[:])
}

func codexDedicatedImageGroupID(apiKey *APIKey) *int64 {
	if apiKey == nil {
		return nil
	}
	if apiKey.GroupID != nil && *apiKey.GroupID > 0 {
		return apiKey.GroupID
	}
	if apiKey.Group != nil && apiKey.Group.ID > 0 {
		groupID := apiKey.Group.ID
		return &groupID
	}
	return nil
}

func (b *CodexDedicatedImageBridge) waitForCompletion(ctx context.Context, apiKey *APIKey, job *ImageGenerationJob) (*ImageGenerationJob, error) {
	if b.repo == nil || apiKey == nil || job == nil {
		return nil, errors.New("codex image job repository is unavailable")
	}
	deadline := time.NewTimer(b.syncTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, err := b.repo.GetImageGenerationJobForOwner(ctx, apiKey.UserID, apiKey.ID, job.JobID)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, errors.New("codex image job lookup returned no task")
		}
		waitingForDeliverySlot := false
		switch current.Status {
		case ImageGenerationJobStatusCompleted:
			if b.queue != nil {
				acquired, acquireErr := b.queue.Acquire(ctx, current.JobID)
				if acquireErr != nil {
					return nil, errors.New("codex image delivery capacity is unavailable")
				}
				if !acquired {
					waitingForDeliverySlot = true
					break
				}
			}
			if waitingForDeliverySlot {
				break
			}
			return current, nil
		case ImageGenerationJobStatusFailed:
			return nil, codexDedicatedImageJobError(current.ErrorCode, current.ErrorMessage, http.StatusBadGateway)
		case ImageGenerationJobStatusSubmissionUnknown:
			return nil, codexDedicatedImageJobError(stringPointer("image_submission_unknown"), current.ErrorMessage, http.StatusServiceUnavailable)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, &CangyuanAdapterError{
				Code: "image_upstream_timeout", HTTPStatus: http.StatusGatewayTimeout, Retryable: true,
				Err: errors.New("codex image generation timed out; the durable image task remains queryable"),
			}
		case <-ticker.C:
		}
	}
}

func codexDedicatedImageJobError(code, message *string, defaultStatus int) error {
	errorCode := strings.TrimSpace(imageGenerationStringValue(code))
	if errorCode == "" {
		errorCode = "image_upstream_rejected"
	}
	status := defaultStatus
	switch {
	case errorCode == "image_upstream_timeout":
		status = http.StatusGatewayTimeout
	case errorCode == "image_submission_unknown":
		status = http.StatusServiceUnavailable
	case strings.HasPrefix(errorCode, "image_invalid_"), errorCode == "image_plan_invalid":
		status = http.StatusBadRequest
	case strings.HasPrefix(errorCode, "image_upstream_"):
		status = http.StatusBadGateway
	}
	publicMessage := strings.TrimSpace(imageGenerationStringValue(message))
	if publicMessage == "" {
		publicMessage = "Codex image generation failed"
	}
	return &CangyuanAdapterError{Code: errorCode, HTTPStatus: status, Err: errors.New(RedactImageGenerationErrorMessage(publicMessage, 1024))}
}

func (b *CodexDedicatedImageBridge) buildCodexImageResponse(ctx context.Context, plannerResult *OpenAIForwardResult, job *ImageGenerationJob, plan *codexDedicatedImagePlan) (string, map[string]any, map[string]any, error) {
	if plannerResult == nil || job == nil || plan == nil {
		return "", nil, nil, errors.New("codex image result is unavailable")
	}
	result, err := b.loadCodexImageResult(ctx, job)
	if err != nil {
		return "", nil, nil, err
	}
	responseID := codexDedicatedImageResponsePrefix + uuid.NewString()
	itemID := "ig_" + uuid.NewString()
	item := map[string]any{
		"id": itemID, "type": "image_generation_call", "status": "completed",
		"result": result.Base64, "revised_prompt": plan.Prompt,
		"output_format": result.OutputFormat,
	}
	output := make([]any, 0, len(plan.PartialText)+1)
	for index, text := range plan.PartialText {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		output = append(output, codexDedicatedImageMessageItem(responseID, index, text, true))
	}
	output = append(output, item)
	usage := plannerResult.Usage
	// Codex CLI requires total_tokens on response.completed. The planner's
	// internal usage model historically tracked input/output separately, so
	// derive the Responses-compatible total for this synthetic response without
	// changing the usage value used by billing and logging.
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	response := map[string]any{
		"id": responseID, "object": "response", "created_at": time.Now().Unix(),
		"status": "completed", "model": plannerResult.Model, "output": output,
		"usage": usage,
	}
	return responseID, response, item, nil
}

func (b *CodexDedicatedImageBridge) loadCodexImageResult(ctx context.Context, job *ImageGenerationJob) (*CodexImageResult, error) {
	if b.payloads != nil && job.PayloadObjectRef != nil {
		payload, err := b.payloads.Get(ctx, *job.PayloadObjectRef)
		if err == nil && payload != nil && payload.CodexResult != nil {
			return payload.CodexResult, nil
		}
		if err != nil && !errors.Is(err, ErrImageGenerationPayloadNotFound) {
			return nil, err
		}
	}
	// Rolling-upgrade compatibility: jobs completed by the previous version
	// still point at object storage and remain deliverable during deployment.
	if b.results == nil || len(job.ResultObjectRefs) == 0 {
		return nil, errors.New("completed Codex image job has no result")
	}
	reader, contentType, _, err := b.results.Open(ctx, job.ResultObjectRefs[0])
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	raw, err := io.ReadAll(io.LimitReader(reader, b.maxReadBytes+1))
	if err != nil || int64(len(raw)) > b.maxReadBytes {
		return nil, errors.New("codex image result is too large to return")
	}
	return &CodexImageResult{
		Base64:       base64.StdEncoding.EncodeToString(raw),
		OutputFormat: imageOutputFormat(contentType),
	}, nil
}

func (b *CodexDedicatedImageBridge) finishCodexImageDelivery(job *ImageGenerationJob, delivered bool) {
	if b == nil || job == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if delivered && b.payloads != nil && job.PayloadObjectRef != nil {
		_ = b.payloads.Delete(ctx, *job.PayloadObjectRef)
	}
	if b.queue != nil {
		_ = b.queue.Release(ctx, job.JobID)
	}
}

func writeCodexDedicatedImageSSE(c *gin.Context, response, item map[string]any) error {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	events, err := buildCodexDedicatedImageEvents(response, item)
	if err != nil {
		return err
	}
	for _, event := range events {
		raw, err := json.Marshal(event.data)
		if err != nil {
			return err
		}
		if object, ok := event.data.(map[string]any); ok {
			object["sequence_number"] = event.sequence
			raw, err = json.Marshal(object)
			if err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.kind, raw); err != nil {
			return err
		}
	}
	return nil
}

type codexDedicatedImageEvent struct {
	kind     string
	data     any
	sequence int
}

func buildCodexDedicatedImageEvents(response, item map[string]any) ([]codexDedicatedImageEvent, error) {
	if response == nil || item == nil {
		return nil, errors.New("codex image response is incomplete")
	}
	created := cloneMap(response)
	created["status"] = "in_progress"
	created["output"] = []any{}
	events := []codexDedicatedImageEvent{
		{kind: "response.created", data: map[string]any{"type": "response.created", "response": created}, sequence: 0},
		{kind: "response.in_progress", data: map[string]any{"type": "response.in_progress", "response": created}, sequence: 1},
	}
	outputItems, _ := response["output"].([]any)
	sequence := 2
	for index, rawOutput := range outputItems {
		outputItem, ok := rawOutput.(map[string]any)
		if !ok || strings.TrimSpace(codexStringValue(outputItem["type"])) != "message" {
			continue
		}
		added := map[string]any{"type": "response.output_item.added", "output_index": index, "item": map[string]any{
			"id": outputItem["id"], "type": "message", "role": "assistant", "status": "in_progress",
		}}
		done := map[string]any{"type": "response.output_item.done", "output_index": index, "item": outputItem}
		events = append(events,
			codexDedicatedImageEvent{kind: "response.output_item.added", data: added, sequence: sequence},
			codexDedicatedImageEvent{kind: "response.output_item.done", data: done, sequence: sequence + 1},
		)
		sequence += 2
	}
	imageIndex := len(outputItems) - 1
	if imageIndex < 0 {
		imageIndex = 0
	}
	imageID := codexStringValue(item["id"])
	added := map[string]any{"type": "response.output_item.added", "output_index": imageIndex, "item": map[string]any{
		"id": item["id"], "type": "image_generation_call", "status": "in_progress",
	}}
	imageInProgress := map[string]any{
		"type": "response.image_generation_call.in_progress", "output_index": imageIndex, "item_id": imageID,
	}
	imageGenerating := map[string]any{
		"type": "response.image_generation_call.generating", "output_index": imageIndex, "item_id": imageID,
	}
	done := map[string]any{"type": "response.output_item.done", "output_index": imageIndex, "item": item}
	imageCompleted := map[string]any{
		"type": "response.image_generation_call.completed", "output_index": imageIndex, "item_id": imageID,
	}
	completed := map[string]any{"type": "response.completed", "response": response}
	events = append(events,
		codexDedicatedImageEvent{kind: "response.output_item.added", data: added, sequence: sequence},
		codexDedicatedImageEvent{kind: "response.image_generation_call.in_progress", data: imageInProgress, sequence: sequence + 1},
		codexDedicatedImageEvent{kind: "response.image_generation_call.generating", data: imageGenerating, sequence: sequence + 2},
		codexDedicatedImageEvent{kind: "response.image_generation_call.completed", data: imageCompleted, sequence: sequence + 3},
		codexDedicatedImageEvent{kind: "response.output_item.done", data: done, sequence: sequence + 4},
		codexDedicatedImageEvent{kind: "response.completed", data: completed, sequence: sequence + 5},
	)
	return events, nil
}

func codexDedicatedImageMessageItem(responseID string, index int, text string, completed bool) map[string]any {
	status := "in_progress"
	if completed {
		status = "completed"
	}
	return map[string]any{
		"id": "msg_" + responseID + "_" + strconv.Itoa(index), "type": "message", "role": "assistant", "status": status,
		"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}},
	}
}

func buildCodexDedicatedImageEventPayloads(response, item map[string]any) ([]json.RawMessage, error) {
	events, err := buildCodexDedicatedImageEvents(response, item)
	if err != nil {
		return nil, err
	}
	result := make([]json.RawMessage, 0, len(events))
	for _, event := range events {
		data := event.data
		if object, ok := event.data.(map[string]any); ok {
			cloned := cloneMap(object)
			cloned["sequence_number"] = event.sequence
			data = cloned
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		result = append(result, raw)
	}
	return result, nil
}

func extractCodexPlannerEventPayloads(raw, requestBody []byte) ([]json.RawMessage, error) {
	if codexDedicatedImageRequestStream(requestBody) {
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(make([]byte, 64*1024), 8<<20)
		result := make([]json.RawMessage, 0)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			if !json.Valid([]byte(payload)) {
				return nil, errors.New("codex planner returned invalid SSE JSON")
			}
			result = append(result, json.RawMessage(payload))
		}
		return result, scanner.Err()
	}
	if !json.Valid(raw) {
		return nil, errors.New("codex planner returned invalid JSON")
	}
	return []json.RawMessage{json.RawMessage(bytes.TrimSpace(raw))}, nil
}

// extractCodexPlannerReplayInput returns the completed planner output items
// that must be carried into the next HTTP-bridged WebSocket turn. This is
// required when the selected general account is an OpenAI-compatible account
// whose gateway path does not retain Responses state upstream: the next turn
// may contain only a previous_response_id and the new user input.
func extractCodexPlannerReplayInput(raw, requestBody []byte) ([]json.RawMessage, error) {
	seen := make(map[string]struct{})
	result := make([]json.RawMessage, 0)
	appendItem := func(item json.RawMessage) {
		item = json.RawMessage(bytes.TrimSpace(item))
		if len(item) == 0 || !json.Valid(item) {
			return
		}
		itemType := strings.TrimSpace(gjson.GetBytes(item, "type").String())
		if !isCodexPlannerReplayItemType(itemType) {
			return
		}
		key := strings.TrimSpace(gjson.GetBytes(item, "id").String())
		if key == "" {
			key = strings.TrimSpace(gjson.GetBytes(item, "call_id").String())
		}
		if key == "" {
			key = string(item)
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	appendOutput := func(value gjson.Result) {
		if !value.Exists() || !value.IsArray() {
			return
		}
		for _, item := range value.Array() {
			appendItem(json.RawMessage(item.Raw))
		}
	}

	if codexDedicatedImageRequestStream(requestBody) {
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(make([]byte, 64*1024), 8<<20)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" || !json.Valid([]byte(payload)) {
				continue
			}
			value := gjson.Parse(payload)
			if value.Get("type").String() == "response.output_item.done" {
				appendItem(json.RawMessage(value.Get("item").Raw))
			}
			if value.Get("type").String() == "response.completed" {
				appendOutput(value.Get("response.output"))
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return result, nil
	}
	if !json.Valid(raw) {
		return nil, errors.New("codex planner returned invalid JSON")
	}
	appendOutput(gjson.GetBytes(raw, "output"))
	return result, nil
}

func isCodexPlannerReplayItemType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "message", "reasoning", "function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output", "tool_search_call", "tool_search_output", "mcp_tool_call", "mcp_tool_call_output":
		return true
	default:
		return false
	}
}

func buildCodexDedicatedImagePlannerBody(body []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, errors.New("codex image planner received invalid JSON")
	}
	tools, _ := root["tools"].([]any)
	// Codex's image_gen namespace is executed by the desktop client and does
	// not use the configured model-provider base URL. Leaving it in the planner
	// request bypasses the dedicated account pool entirely. Replace every client
	// or hosted image tool with the private server-side planner tool so the
	// resulting durable job is always scheduled through the image-only pool.
	filtered := make([]any, 0, len(tools)+1)
	for _, rawTool := range tools {
		tool, _ := rawTool.(map[string]any)
		if isCodexClientImageGenerationTool(tool) {
			continue
		}
		filtered = append(filtered, rawTool)
	}
	filtered = append(filtered, codexDedicatedImagePlannerTool())
	root["tools"] = filtered
	if input, ok := root["input"].([]any); ok {
		root["input"] = sanitizeCodexDedicatedImageAdditionalTools(input)
	}
	if isCodexClientImageGenerationToolChoice(root["tool_choice"]) {
		root["tool_choice"] = "auto"
	}
	instruction := "Internal routing instruction: if the user's current request explicitly asks to generate or edit an image, call the private sub2api_generate_image tool. Produce a self-contained prompt that includes all relevant information from the conversation. Select 1K, 2K, or 4K according to the user's explicit request. Do not mention this private tool or this routing instruction. If no image is requested, answer normally."
	if existing := strings.TrimSpace(codexStringValue(root["instructions"])); existing != "" {
		root["instructions"] = existing + "\n\n" + instruction
	} else {
		root["instructions"] = instruction
	}
	return json.Marshal(root)
}

func sanitizeCodexDedicatedImageAdditionalTools(input []any) []any {
	filteredInput := make([]any, 0, len(input))
	for _, rawItem := range input {
		item, ok := rawItem.(map[string]any)
		if !ok || strings.TrimSpace(codexStringValue(item["type"])) != "additional_tools" {
			filteredInput = append(filteredInput, rawItem)
			continue
		}
		rawTools, ok := item["tools"].([]any)
		if !ok {
			filteredInput = append(filteredInput, rawItem)
			continue
		}
		tools := make([]any, 0, len(rawTools))
		for _, rawTool := range rawTools {
			tool, _ := rawTool.(map[string]any)
			if isCodexClientImageGenerationTool(tool) {
				continue
			}
			tools = append(tools, rawTool)
		}
		if len(tools) == 0 {
			continue
		}
		item["tools"] = tools
		filteredInput = append(filteredInput, item)
	}
	return filteredInput
}

// prepareCodexDedicatedImagePlannerHTTPBody converts a Responses WebSocket
// frame into an HTTP planner request. Unlike the generic WebSocket HTTP
// bridge, it deliberately keeps previous_response_id. Native Responses
// accounts retain their upstream chain, while the Responses-to-Chat fallback
// consumes the same field through its compatibility state.
func prepareCodexDedicatedImagePlannerHTTPBody(body []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, errors.New("codex planner received invalid JSON")
	}
	if root == nil {
		return nil, errors.New("codex planner request must be a JSON object")
	}
	delete(root, "type")
	delete(root, "generate")
	root["stream"] = true
	return json.Marshal(root)
}

func codexDedicatedImagePlannerTool() map[string]any {
	return map[string]any{
		"type": "function", "name": codexDedicatedImagePlannerToolName,
		"description": "Internal Sub2API image plan. Use only when the user explicitly requests image generation or editing.",
		"parameters": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"prompt":          map[string]any{"type": "string", "description": "Legacy alias; must be self-contained when visual_prompt is absent."},
				"visual_prompt":   map[string]any{"type": "string", "description": "Self-contained visual prompt that can execute without conversation history."},
				"title":           map[string]any{"type": "string"},
				"summary":         map[string]any{"type": "string"},
				"sections":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"relationships":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"must_include":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"must_not_invent": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"layout":          map[string]any{"type": "string", "enum": []string{"auto", "mind_map", "flowchart", "timeline", "infographic", "poster", "diagram"}},
				"language":        map[string]any{"type": "string", "enum": []string{"auto", "zh", "zh-cn", "zh-tw", "en", "ja", "ko"}},
				"resolution":      map[string]any{"type": "string", "enum": []string{"1K", "2K", "4K", "1k", "2k", "4k"}},
				"aspect_ratio":    map[string]any{"type": "string", "description": "Optional WIDTH:HEIGHT ratio."},
				"model":           map[string]any{"type": "string", "enum": []string{"gpt-image-2-1k", "gpt-image-2-2k", "gpt-image-2-4k"}},
				"size":            map[string]any{"type": "string", "description": "Optional WIDTHxHEIGHT output dimensions."},
				"quality":         map[string]any{"type": "string", "enum": []string{"auto", "low", "medium", "high"}},
			},
			"required": []string{"prompt"},
		},
		// This planner intentionally has optional enrichment fields. Responses
		// strict mode requires every property to be listed in required (optional
		// values must be nullable), so strict=true with only prompt required is
		// rejected by conforming upstreams before the model runs. The bridge
		// validates and normalizes every returned plan itself.
		"strict": false,
	}
}

func isCodexClientImageGenerationTool(tool map[string]any) bool {
	if tool == nil {
		return false
	}
	toolType := strings.TrimSpace(codexStringValue(tool["type"]))
	name := strings.TrimSpace(codexStringValue(tool["name"]))
	namespace := strings.TrimSpace(codexStringValue(tool["namespace"]))
	if toolType == "image_generation" {
		return true
	}
	if toolType == "namespace" && (isOpenAIImageGenNamespaceName(name) || isOpenAIImageGenNamespaceName(namespace)) {
		return true
	}
	if isOpenAIImageGenFunctionReference(namespace, name) {
		return true
	}
	if function, ok := tool["function"].(map[string]any); ok {
		return isOpenAIImageGenFunctionReference(
			strings.TrimSpace(codexStringValue(function["namespace"])),
			strings.TrimSpace(codexStringValue(function["name"])),
		)
	}
	return false
}

func isCodexClientImageGenerationToolChoice(choice any) bool {
	if openAIAnyToolChoiceSelectsImageGeneration(choice) {
		return true
	}
	switch value := choice.(type) {
	case string:
		value = strings.TrimSpace(value)
		return isOpenAIImageGenNamespaceName(value) || isOpenAIImageGenFunctionReference("", value)
	case map[string]any:
		if isOpenAIImageGenFunctionReference(
			strings.TrimSpace(codexStringValue(value["namespace"])),
			strings.TrimSpace(codexStringValue(value["name"])),
		) {
			return true
		}
		if tool, ok := value["tool"].(map[string]any); ok && isCodexClientImageGenerationToolChoice(tool) {
			return true
		}
		if function, ok := value["function"].(map[string]any); ok {
			return isCodexClientImageGenerationToolChoice(function)
		}
	}
	return false
}

type codexDedicatedImagePlan struct {
	Prompt        string   `json:"prompt"`
	VisualPrompt  string   `json:"visual_prompt"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Sections      []string `json:"sections"`
	Relationships []string `json:"relationships"`
	MustInclude   []string `json:"must_include"`
	MustNotInvent []string `json:"must_not_invent"`
	Layout        string   `json:"layout"`
	Language      string   `json:"language"`
	Resolution    string   `json:"resolution"`
	AspectRatio   string   `json:"aspect_ratio"`
	Model         string   `json:"model"`
	Size          string   `json:"size"`
	Quality       string   `json:"quality"`
	CallID        string   `json:"-"`
	PartialText   []string `json:"-"`
}

func extractCodexDedicatedImagePlan(raw []byte, requestBody []byte) (*codexDedicatedImagePlan, bool, error) {
	stream := codexDedicatedImageRequestStream(requestBody)
	var candidates []map[string]any
	if stream {
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(make([]byte, 64*1024), 8<<20)
		argumentDeltas := make(map[string]*codexDedicatedImageArgumentsAccumulator)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var value map[string]any
			if json.Unmarshal([]byte(payload), &value) == nil {
				candidates = append(candidates, value)
				collectCodexDedicatedImageArgumentDelta(value, argumentDeltas)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, false, err
		}
		for _, accumulator := range argumentDeltas {
			if accumulator == nil || accumulator.name != codexDedicatedImagePlannerToolName || accumulator.arguments.Len() == 0 {
				continue
			}
			candidate := map[string]any{
				"name":      codexDedicatedImagePlannerToolName,
				"arguments": accumulator.arguments.String(),
			}
			if accumulator.callID != "" {
				candidate["call_id"] = accumulator.callID
			}
			candidates = append(candidates, candidate)
		}
	} else {
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false, nil // an opaque non-plan response is replayed.
		}
		candidates = append(candidates, value)
	}
	var selectedArgs []byte
	var selectedCallID string
	var selectedPlan *codexDedicatedImagePlan
	for _, value := range candidates {
		for _, candidate := range findAllCodexDedicatedImageArguments(value) {
			args, callID := candidate.arguments, candidate.callID
			callID = strings.TrimSpace(callID)
			callIDsDiffer := selectedCallID != "" && callID != "" && callID != selectedCallID
			if selectedArgs != nil && (callIDsDiffer || !codexDedicatedImageArgumentsEqual(args, selectedArgs)) {
				return nil, true, codexDedicatedImagePlanError("multiple image tool calls cannot be completed in one response")
			}
			if selectedArgs != nil {
				continue
			}
			selectedArgs = append([]byte(nil), args...)
			selectedCallID = callID
			var plan codexDedicatedImagePlan
			if err := json.Unmarshal(args, &plan); err != nil {
				return nil, true, errors.New("codex image planner returned invalid arguments")
			}
			if err := normalizeAndValidateCodexDedicatedImagePlan(&plan); err != nil {
				return nil, true, err
			}
			plan.CallID = callID
			if plan.CallID == "" {
				plan.CallID = codexDedicatedImagePlannerToolName
			}
			plan.PartialText = extractCodexPlannerPartialText(raw, stream)
			selectedPlan = &plan
		}
	}
	if selectedPlan != nil {
		return selectedPlan, true, ValidateCangyuanImageRequest(CangyuanImageOperationGeneration, CangyuanImageRequest{Model: selectedPlan.Model, Prompt: selectedPlan.Prompt, Size: selectedPlan.Size, AspectRatio: selectedPlan.AspectRatio, Quality: selectedPlan.Quality, N: 1, ResponseFormat: "b64_json", Async: false, ImageSize: dedicatedImageTierForModel(selectedPlan.Model), OutputResolution: dedicatedImageTierForModel(selectedPlan.Model)})
	}
	return nil, false, nil
}

func codexDedicatedImageArgumentsEqual(left, right []byte) bool {
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil {
		leftCanonical, leftErr := json.Marshal(leftValue)
		rightCanonical, rightErr := json.Marshal(rightValue)
		if leftErr == nil && rightErr == nil {
			return bytes.Equal(leftCanonical, rightCanonical)
		}
	}
	return bytes.Equal(bytes.TrimSpace(left), bytes.TrimSpace(right))
}

const (
	codexDedicatedImagePlanMaxPromptBytes = 16 << 10
	codexDedicatedImagePlanMaxFieldBytes  = 4 << 10
	codexDedicatedImagePlanMaxItems       = 64
)

func normalizeAndValidateCodexDedicatedImagePlan(plan *codexDedicatedImagePlan) error {
	if plan == nil {
		return codexDedicatedImagePlanError("plan is missing")
	}
	plan.Prompt = strings.TrimSpace(plan.Prompt)
	plan.VisualPrompt = strings.TrimSpace(plan.VisualPrompt)
	plan.Title = strings.TrimSpace(plan.Title)
	plan.Summary = strings.TrimSpace(plan.Summary)
	plan.Layout = strings.ToLower(strings.TrimSpace(plan.Layout))
	plan.Language = strings.TrimSpace(plan.Language)
	plan.Resolution = strings.ToUpper(strings.TrimSpace(plan.Resolution))
	plan.AspectRatio = strings.TrimSpace(plan.AspectRatio)
	plan.Size = strings.TrimSpace(plan.Size)
	// Codex planners often use the public 1K/2K/4K labels in `size`, even
	// though Cangyuan receives those labels through image_size/output_resolution
	// and expects `size` to be WIDTHxHEIGHT. Also, Cangyuan rejects a request
	// that carries both an explicit size and aspect_ratio. Normalize these
	// equivalent planner representations before validating the provider request.
	switch strings.ToUpper(plan.Size) {
	case "1K", "2K", "4K":
		plan.Size = ""
	}
	if plan.AspectRatio != "" {
		plan.Size = ""
	}
	plan.Quality = normalizeCodexDedicatedImageQuality(plan.Quality)
	rawModel := strings.TrimSpace(plan.Model)
	plan.Model = normalizeDedicatedImageModel(rawModel)
	if rawModel != "" && plan.Model == "" {
		return codexDedicatedImagePlanError("model is not an allowed Cangyuan tier")
	}

	if plan.Model == "" {
		switch plan.Resolution {
		case "1K":
			plan.Model = CangyuanImageModel1K
		case "2K":
			plan.Model = CangyuanImageModel2K
		case "4K":
			plan.Model = CangyuanImageModel4K
		default:
			plan.Model = CangyuanImageModel1K
		}
	}
	if plan.Resolution == "" {
		plan.Resolution = dedicatedImageTierForModel(plan.Model)
	}
	if plan.Resolution != dedicatedImageTierForModel(plan.Model) {
		return codexDedicatedImagePlanError("resolution conflicts with model")
	}
	if plan.Prompt == "" && plan.VisualPrompt == "" {
		return codexDedicatedImagePlanError("prompt or visual_prompt is required")
	}
	if len(plan.Prompt) > codexDedicatedImagePlanMaxPromptBytes || len(plan.VisualPrompt) > codexDedicatedImagePlanMaxPromptBytes {
		return codexDedicatedImagePlanError("prompt is too long")
	}
	if len(plan.Title) > codexDedicatedImagePlanMaxFieldBytes || len(plan.Summary) > codexDedicatedImagePlanMaxFieldBytes {
		return codexDedicatedImagePlanError("plan field is too long")
	}
	if err := validateCodexDedicatedImagePlanItems(plan.Sections); err != nil {
		return err
	}
	if err := validateCodexDedicatedImagePlanItems(plan.Relationships); err != nil {
		return err
	}
	if err := validateCodexDedicatedImagePlanItems(plan.MustInclude); err != nil {
		return err
	}
	if err := validateCodexDedicatedImagePlanItems(plan.MustNotInvent); err != nil {
		return err
	}
	if !validCodexDedicatedImageLayout(plan.Layout) {
		return codexDedicatedImagePlanError("layout is not supported")
	}
	if !validCodexDedicatedImageLanguage(plan.Language) {
		return codexDedicatedImagePlanError("language is not supported")
	}
	if plan.AspectRatio != "" {
		if err := validateCangyuanAspectRatio(plan.AspectRatio); err != nil {
			return codexDedicatedImagePlanError("aspect_ratio is invalid")
		}
	}
	effectiveVisualPrompt := plan.VisualPrompt
	if effectiveVisualPrompt == "" {
		effectiveVisualPrompt = plan.Prompt
	}
	if codexDedicatedImagePlanHasUnexpandedReference(effectiveVisualPrompt) {
		return codexDedicatedImagePlanError("visual prompt contains an unexpanded conversation reference")
	}
	plan.Prompt = codexDedicatedImagePlanPrompt(plan)
	if len(plan.Prompt) == 0 || len(plan.Prompt) > codexDedicatedImagePlanMaxPromptBytes {
		return codexDedicatedImagePlanError("self-contained visual prompt is invalid")
	}
	// Structured fields are appended after the primary visual_prompt check.
	// Recheck the assembled prompt so a planner cannot hide an unresolved
	// conversation reference in summary, sections, or relationships.
	if codexDedicatedImagePlanHasUnexpandedReference(plan.Prompt) {
		return codexDedicatedImagePlanError("self-contained visual prompt contains an unexpanded conversation reference")
	}
	return nil
}

func normalizeCodexDedicatedImageQuality(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "standard", "default", "normal":
		// OpenAI-compatible planners commonly use the legacy Images API
		// spelling "standard" even when a provider exposes the newer
		// low/medium/high/auto vocabulary. Preserve the user's lack of an
		// explicit quality preference by delegating the choice to Cangyuan.
		return "auto"
	case "hd":
		return "high"
	default:
		return value
	}
}

func validateCodexDedicatedImagePlanItems(items []string) error {
	if len(items) > codexDedicatedImagePlanMaxItems {
		return codexDedicatedImagePlanError("plan contains too many items")
	}
	for _, item := range items {
		if len(strings.TrimSpace(item)) == 0 || len(item) > codexDedicatedImagePlanMaxFieldBytes {
			return codexDedicatedImagePlanError("plan item is invalid")
		}
	}
	return nil
}

func validCodexDedicatedImageLayout(layout string) bool {
	switch layout {
	case "", "auto", "mind_map", "flowchart", "timeline", "infographic", "poster", "diagram":
		return true
	default:
		return false
	}
}

func validCodexDedicatedImageLanguage(language string) bool {
	switch strings.ToLower(language) {
	case "", "auto", "zh", "zh-cn", "zh-tw", "en", "ja", "ko":
		return true
	default:
		return false
	}
}

func codexDedicatedImagePlanHasUnexpandedReference(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	for _, pattern := range codexDedicatedImageUnexpandedReferencePatterns {
		matches := pattern.FindAllStringIndex(value, -1)
		for _, match := range matches {
			if !codexDedicatedImageReferenceHasConcreteContext(value, match[0], match[1]) {
				return true
			}
		}
	}
	return false
}

var codexDedicatedImageUnexpandedReferencePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?:按|根据|依照)?(?:我们)?(?:之前|上述|刚才|前面|上文)(?:讨论|提到|说过|提供)?(?:过的)?(?:内容|信息|要点|观点|细节|资料|需求)?`),
	regexp.MustCompile(`(?:based\s+on\s+)?(?:the\s+)?(?:previous|earlier|above)(?:ly)?(?:\s+(?:discussion|conversation|content|points?|information|messages?|details?|requirements?))?`),
	regexp.MustCompile(`as\s+discussed(?:\s+(?:above|earlier|previously))?`),
}

func codexDedicatedImageReferenceHasConcreteContext(value string, start, end int) bool {
	nearby := value[:start] + " " + value[end:]
	nearby = codexDedicatedImageGenericReferenceWords.ReplaceAllString(nearby, " ")
	concrete := 0
	for _, token := range codexDedicatedImageConcreteTokenPattern.FindAllString(nearby, -1) {
		if len([]rune(token)) >= 2 {
			concrete += len([]rune(token))
		}
	}
	// A short subject name alone does not expand a conversational reference.
	// Requiring several concrete terms still accepts compact technical prompts
	// such as "TCP 三次握手: SYN, SYN-ACK, ACK".
	return concrete >= 18
}

var codexDedicatedImageGenericReferenceWords = regexp.MustCompile(`(?i)\b(?:create|draw|generate|make|image|diagram|visual|visualize|include|show|summarize|summary|mind|map|using|with|from|about|please|the|a|an|of|and|or|all|those|these|them|it|this|that|our|we|discussed|mentioned|provided|content|points?|information|details?|requirements?)\b|(?:生成|创建|绘制|制作|图片|图像|图表|示意图|思维导图|可视化|展示|包含|总结|所有|这些|那些|这个|那个|内容|信息|要点|观点|细节|资料|需求|讨论|提到|说过|提供|请|按照|根据|基于|关于|以及|并且|用于)`)
var codexDedicatedImageConcreteTokenPattern = regexp.MustCompile(`[a-z0-9][a-z0-9_+./:-]*|[\p{Han}]+`)

func codexDedicatedImagePlanError(message string) error {
	return &CangyuanAdapterError{Code: "image_plan_invalid", HTTPStatus: http.StatusBadRequest, Err: errors.New(message)}
}

func codexDedicatedImagePlanPrompt(plan *codexDedicatedImagePlan) string {
	if plan == nil {
		return ""
	}
	parts := make([]string, 0, 10)
	base := strings.TrimSpace(plan.VisualPrompt)
	if base == "" {
		base = strings.TrimSpace(plan.Prompt)
	}
	if base != "" {
		parts = append(parts, base)
	}
	if plan.Title != "" {
		parts = append(parts, "Title: "+plan.Title)
	}
	if plan.Summary != "" {
		parts = append(parts, "Summary: "+plan.Summary)
	}
	appendItems := func(label string, values []string) {
		if len(values) == 0 {
			return
		}
		parts = append(parts, label+": "+strings.Join(values, "; "))
	}
	appendItems("Sections", plan.Sections)
	appendItems("Relationships", plan.Relationships)
	appendItems("Must include", plan.MustInclude)
	appendItems("Do not invent", plan.MustNotInvent)
	if plan.Layout != "" {
		parts = append(parts, "Layout: "+plan.Layout)
	}
	if plan.Language != "" {
		parts = append(parts, "Language: "+plan.Language)
	}
	if plan.AspectRatio != "" {
		parts = append(parts, "Aspect ratio: "+plan.AspectRatio)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// extractCodexPlannerPartialText keeps user-visible planner text that was
// emitted before the private image tool call. A planner is allowed to say
// something short such as "I will turn those points into a mind map" before
// invoking the tool; dropping that text makes a streamed Codex response look
// truncated. Only text emitted by the planner is collected here. It is never
// copied into the Cangyuan prompt (the prompt is still built exclusively from
// the validated, self-contained image plan).
func extractCodexPlannerPartialText(raw []byte, stream bool) []string {
	if !stream {
		var value map[string]any
		if json.Unmarshal(raw, &value) != nil {
			return nil
		}
		return responseMessageText(value)
	}

	var completed []string
	var deltas []string
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 8<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var value map[string]any
		if json.Unmarshal([]byte(payload), &value) != nil {
			continue
		}
		if text := responseMessageText(value); len(text) > 0 {
			completed = text
		}
		if strings.TrimSpace(codexStringValue(value["type"])) == "response.output_text.delta" {
			if delta := strings.TrimSpace(codexStringValue(value["delta"])); delta != "" {
				deltas = append(deltas, delta)
			}
		}
	}
	if len(completed) > 0 {
		return completed
	}
	if len(deltas) == 0 {
		return nil
	}
	return []string{strings.TrimSpace(strings.Join(deltas, ""))}
}

func responseMessageText(value map[string]any) []string {
	if value == nil {
		return nil
	}
	if response, ok := value["response"].(map[string]any); ok {
		if text := responseMessageText(response); len(text) > 0 {
			return text
		}
	}
	if item, ok := value["item"].(map[string]any); ok {
		if text := responseMessageText(item); len(text) > 0 {
			return text
		}
	}
	if output, ok := value["output"].([]any); ok {
		result := make([]string, 0)
		for _, rawItem := range output {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, responseMessageText(item)...)
		}
		return result
	}
	if itemType := strings.TrimSpace(codexStringValue(value["type"])); itemType != "message" && itemType != "output_item" {
		return nil
	}
	content, ok := value["content"].([]any)
	if !ok {
		if text := strings.TrimSpace(codexStringValue(value["text"])); text != "" {
			return []string{text}
		}
		return nil
	}
	var result []string
	for _, rawPart := range content {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		partType := strings.TrimSpace(codexStringValue(part["type"]))
		if partType != "output_text" && partType != "text" {
			continue
		}
		text := strings.TrimSpace(codexStringValue(part["text"]))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

type codexDedicatedImageArgumentsAccumulator struct {
	name      string
	callID    string
	arguments strings.Builder
}

type codexDedicatedImageArgumentCandidate struct {
	arguments []byte
	callID    string
}

func collectCodexDedicatedImageArgumentDelta(value map[string]any, accumulators map[string]*codexDedicatedImageArgumentsAccumulator) {
	if value == nil {
		return
	}
	eventType := strings.TrimSpace(codexStringValue(value["type"]))
	if eventType == "response.output_item.added" || eventType == "response.output_item.done" {
		item, ok := value["item"].(map[string]any)
		if !ok || strings.TrimSpace(codexStringValue(item["type"])) != "function_call" {
			return
		}
		key := strings.TrimSpace(codexStringValue(item["id"]))
		if key == "" {
			key = strings.TrimSpace(codexStringValue(item["call_id"]))
		}
		if key == "" {
			key = strings.TrimSpace(codexStringValue(value["output_index"]))
		}
		if key == "" {
			return
		}
		accumulator := accumulators[key]
		if accumulator == nil {
			accumulator = &codexDedicatedImageArgumentsAccumulator{}
			accumulators[key] = accumulator
		}
		if accumulator.name == "" {
			accumulator.name = strings.TrimSpace(codexStringValue(item["name"]))
		}
		if accumulator.callID == "" {
			accumulator.callID = strings.TrimSpace(codexStringValue(item["call_id"]))
			if accumulator.callID == "" {
				accumulator.callID = strings.TrimSpace(codexStringValue(item["id"]))
			}
		}
		return
	}
	if eventType != "response.function_call_arguments.delta" && eventType != "response.function_call_arguments.done" {
		return
	}
	key := strings.TrimSpace(codexStringValue(value["item_id"]))
	if key == "" {
		key = strings.TrimSpace(codexStringValue(value["call_id"]))
	}
	if key == "" {
		key = strings.TrimSpace(codexStringValue(value["output_index"]))
	}
	if key == "" {
		return
	}
	accumulator := accumulators[key]
	if accumulator == nil {
		accumulator = &codexDedicatedImageArgumentsAccumulator{}
		accumulators[key] = accumulator
	}
	if accumulator.name == "" {
		accumulator.name = strings.TrimSpace(codexStringValue(value["name"]))
	}
	if accumulator.name == "" {
		if item, ok := value["item"].(map[string]any); ok {
			accumulator.name = strings.TrimSpace(codexStringValue(item["name"]))
		}
	}
	if accumulator.callID == "" {
		accumulator.callID = strings.TrimSpace(codexStringValue(value["call_id"]))
		if accumulator.callID == "" {
			accumulator.callID = strings.TrimSpace(codexStringValue(value["item_id"]))
		}
	}
	if eventType == "response.function_call_arguments.done" {
		if arguments := codexStringValue(value["arguments"]); arguments != "" {
			accumulator.arguments.Reset()
			_, _ = accumulator.arguments.WriteString(arguments)
		}
		return
	}
	if delta := codexStringValue(value["delta"]); delta != "" {
		_, _ = accumulator.arguments.WriteString(delta)
	}
}

func findAllCodexDedicatedImageArguments(value map[string]any) []codexDedicatedImageArgumentCandidate {
	if value == nil {
		return nil
	}
	result := make([]codexDedicatedImageArgumentCandidate, 0)
	if strings.TrimSpace(codexStringValue(value["name"])) == codexDedicatedImagePlannerToolName {
		callID := strings.TrimSpace(codexStringValue(value["call_id"]))
		if callID == "" {
			callID = strings.TrimSpace(codexStringValue(value["id"]))
		}
		var arguments []byte
		switch args := value["arguments"].(type) {
		case string:
			arguments = []byte(args)
		case map[string]any:
			arguments, _ = json.Marshal(args)
		}
		if len(arguments) > 0 {
			result = append(result, codexDedicatedImageArgumentCandidate{arguments: arguments, callID: callID})
		}
	}
	if item, ok := value["item"].(map[string]any); ok {
		result = append(result, findAllCodexDedicatedImageArguments(item)...)
	}
	if output, ok := value["output"].([]any); ok {
		for _, rawItem := range output {
			if item, ok := rawItem.(map[string]any); ok {
				result = append(result, findAllCodexDedicatedImageArguments(item)...)
			}
		}
	}
	return result
}

func cloneGinContextForCodexPlanner(parent *gin.Context, ctx context.Context, body []byte, recorder *httptest.ResponseRecorder) (*gin.Context, error) {
	if parent == nil || parent.Request == nil {
		return nil, errors.New("codex planner request context is unavailable")
	}
	planner, _ := gin.CreateTestContext(recorder)
	planner.Request = parent.Request.Clone(ctx)
	planner.Request.Body = io.NopCloser(bytes.NewReader(body))
	planner.Request.ContentLength = int64(len(body))
	planner.Params = append(planner.Params[:0], parent.Params...)
	for key, value := range parent.Keys {
		planner.Set(key, value)
	}
	return planner, nil
}

func replayCodexPlannerResponse(c *gin.Context, recorder *httptest.ResponseRecorder) error {
	if c == nil || recorder == nil {
		return errors.New("codex planner response context is unavailable")
	}
	for key, values := range recorder.Header() {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	status := recorder.Code
	if status == 0 {
		status = http.StatusOK
	}
	c.Status(status)
	_, err := c.Writer.Write(recorder.Body.Bytes())
	return err
}

func isBareCodexResponsesPath(path string) bool {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	return strings.HasSuffix(path, "/responses") && !strings.HasSuffix(path, "/responses/compact")
}

func dedicatedImageTierForModel(model string) string {
	switch normalizeDedicatedImageModel(model) {
	case CangyuanImageModel2K:
		return "2K"
	case CangyuanImageModel4K:
		return "4K"
	default:
		return "1K"
	}
}

func normalizeDedicatedImageModel(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "1k", "gpt-image-2-1k":
		return CangyuanImageModel1K
	case "2k", "gpt-image-2-2k":
		return CangyuanImageModel2K
	case "4k", "gpt-image-2-4k":
		return CangyuanImageModel4K
	default:
		return ""
	}
}

func imageOutputFormat(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	switch strings.ToLower(mediaType) {
	case "image/jpeg":
		return "jpeg"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}

func codexDedicatedImageRequestStream(body []byte) bool {
	var envelope struct {
		Stream *bool `json:"stream"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Stream == nil {
		return false
	}
	return *envelope.Stream
}

const (
	codexDedicatedImageReplayTTL        = 7 * 24 * time.Hour
	codexDedicatedImageReplayMaxEntries = 65536
)

func (b *CodexDedicatedImageBridge) buildDedicatedImageFunctionCallOutput(plan *codexDedicatedImagePlan) json.RawMessage {
	callID := codexDedicatedImagePlannerToolName
	if plan != nil && strings.TrimSpace(plan.CallID) != "" {
		callID = strings.TrimSpace(plan.CallID)
	}
	output := "Dedicated image generation completed. The generated image was returned to the user."
	if plan != nil {
		output = fmt.Sprintf(
			"Dedicated image generation completed for model %s at %s. The generated image was returned to the user.",
			normalizeDedicatedImageModel(plan.Model), dedicatedImageTierForModel(plan.Model),
		)
	}
	value := map[string]any{
		"type":    "function_call_output",
		"call_id": callID,
		"output":  output,
	}
	raw, _ := json.Marshal(value)
	return raw
}

func (b *CodexDedicatedImageBridge) buildDedicatedImageFunctionCallContext(plan *codexDedicatedImagePlan) json.RawMessage {
	callID := codexDedicatedImagePlannerToolName
	if plan != nil && strings.TrimSpace(plan.CallID) != "" {
		callID = strings.TrimSpace(plan.CallID)
	}
	arguments := map[string]any{}
	if plan != nil {
		arguments["prompt"] = plan.Prompt
		arguments["model"] = normalizeDedicatedImageModel(plan.Model)
		if strings.TrimSpace(plan.VisualPrompt) != "" {
			arguments["visual_prompt"] = plan.VisualPrompt
		}
		if strings.TrimSpace(plan.Title) != "" {
			arguments["title"] = plan.Title
		}
		if strings.TrimSpace(plan.Summary) != "" {
			arguments["summary"] = plan.Summary
		}
		if len(plan.Sections) > 0 {
			arguments["sections"] = plan.Sections
		}
		if len(plan.Relationships) > 0 {
			arguments["relationships"] = plan.Relationships
		}
		if len(plan.MustInclude) > 0 {
			arguments["must_include"] = plan.MustInclude
		}
		if len(plan.MustNotInvent) > 0 {
			arguments["must_not_invent"] = plan.MustNotInvent
		}
		if strings.TrimSpace(plan.Layout) != "" {
			arguments["layout"] = plan.Layout
		}
		if strings.TrimSpace(plan.Language) != "" {
			arguments["language"] = plan.Language
		}
		if strings.TrimSpace(plan.Resolution) != "" {
			arguments["resolution"] = plan.Resolution
		}
		if strings.TrimSpace(plan.AspectRatio) != "" {
			arguments["aspect_ratio"] = plan.AspectRatio
		}
		if strings.TrimSpace(plan.Size) != "" {
			arguments["size"] = plan.Size
		}
		if strings.TrimSpace(plan.Quality) != "" {
			arguments["quality"] = plan.Quality
		}
	}
	argumentBytes, _ := json.Marshal(arguments)
	value := map[string]any{
		"type":      "function_call",
		"id":        callID,
		"call_id":   callID,
		"name":      codexDedicatedImagePlannerToolName,
		"arguments": string(argumentBytes),
		"status":    "completed",
	}
	raw, _ := json.Marshal(value)
	return raw
}

func (b *CodexDedicatedImageBridge) rememberDedicatedImageReplay(ctx context.Context, responseID, upstreamResponseID string, plan *codexDedicatedImagePlan, groupID *int64, accountID int64) error {
	if b == nil {
		return nil
	}
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return nil
	}
	replay := codexDedicatedImageReplay{
		UpstreamResponseID: strings.TrimSpace(upstreamResponseID),
		FunctionCallOutput: b.buildDedicatedImageFunctionCallOutput(plan),
		ExpiresAt:          time.Now().Add(codexDedicatedImageReplayTTL),
	}
	if b.replayStore != nil {
		raw, err := json.Marshal(replay)
		if err != nil {
			return err
		}
		if err := b.replayStore.SetCodexDedicatedImageReplay(ctx, responseID, raw, codexDedicatedImageReplayTTL); err != nil {
			return fmt.Errorf("persist Codex image replay: %w", err)
		}
	}
	b.replayMu.Lock()
	if b.replays == nil {
		b.replays = make(map[string]codexDedicatedImageReplay)
	}
	if len(b.replays) >= codexDedicatedImageReplayMaxEntries {
		for key := range b.replays {
			delete(b.replays, key)
			break
		}
	}
	b.replays[responseID] = replay
	b.replayMu.Unlock()
	// The public synthetic response ID is also used by the normal scheduler on
	// the next HTTP turn. Bind it to the original planner account so that a
	// follow-up text request cannot land on a different general account before
	// resolveDedicatedImageReplay restores the real upstream response ID.
	if b.gateway != nil && groupID != nil && *groupID > 0 && accountID > 0 {
		store := b.gateway.getOpenAIWSStateStore()
		if store != nil {
			bindingTTL := b.gateway.openAIWSResponseStickyTTL()
			if bindingTTL < codexDedicatedImageReplayTTL {
				bindingTTL = codexDedicatedImageReplayTTL
			}
			if err := store.BindResponseAccount(ctx, *groupID, responseID, accountID, bindingTTL); err != nil {
				return fmt.Errorf("persist Codex image planner account binding: %w", err)
			}
		}
	}
	return nil
}

func (b *CodexDedicatedImageBridge) dedicatedImageReplay(ctx context.Context, responseID string) (codexDedicatedImageReplay, bool, error) {
	if b == nil {
		return codexDedicatedImageReplay{}, false, nil
	}
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return codexDedicatedImageReplay{}, false, nil
	}
	b.replayMu.RLock()
	replay, ok := b.replays[responseID]
	b.replayMu.RUnlock()
	if ok {
		if !replay.ExpiresAt.IsZero() && time.Now().After(replay.ExpiresAt) {
			b.replayMu.Lock()
			delete(b.replays, responseID)
			b.replayMu.Unlock()
			if b.replayStore != nil {
				_ = b.replayStore.DeleteCodexDedicatedImageReplay(ctx, responseID)
			}
			return codexDedicatedImageReplay{}, false, nil
		}
		return replay, true, nil
	}
	if b.replayStore == nil {
		return codexDedicatedImageReplay{}, false, nil
	}
	raw, err := b.replayStore.GetCodexDedicatedImageReplay(ctx, responseID)
	if err != nil {
		if errors.Is(err, ErrCodexDedicatedImageReplayNotFound) {
			return codexDedicatedImageReplay{}, false, nil
		}
		return codexDedicatedImageReplay{}, false, fmt.Errorf("load Codex image replay: %w", err)
	}
	if len(raw) == 0 {
		_ = b.replayStore.DeleteCodexDedicatedImageReplay(ctx, responseID)
		return codexDedicatedImageReplay{}, false, ErrCodexDedicatedImageReplayCorrupt
	}
	if err := json.Unmarshal(raw, &replay); err != nil {
		_ = b.replayStore.DeleteCodexDedicatedImageReplay(ctx, responseID)
		return codexDedicatedImageReplay{}, false, ErrCodexDedicatedImageReplayCorrupt
	}
	if !replay.ExpiresAt.IsZero() && time.Now().After(replay.ExpiresAt) {
		_ = b.replayStore.DeleteCodexDedicatedImageReplay(ctx, responseID)
		return codexDedicatedImageReplay{}, false, nil
	}
	b.replayMu.Lock()
	if b.replays == nil {
		b.replays = make(map[string]codexDedicatedImageReplay)
	}
	if len(b.replays) < codexDedicatedImageReplayMaxEntries {
		b.replays[responseID] = replay
	}
	b.replayMu.Unlock()
	return replay, true, nil
}

func (b *CodexDedicatedImageBridge) resolveDedicatedImageReplay(ctx context.Context, body []byte) ([]byte, error) {
	previousResponseID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	if previousResponseID == "" {
		return body, nil
	}
	replay, ok, err := b.dedicatedImageReplay(ctx, previousResponseID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return body, nil
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil || root == nil {
		return body, nil
	}
	delete(root, "previous_response_id")
	if replay.UpstreamResponseID != "" {
		root["previous_response_id"] = replay.UpstreamResponseID
	}
	var input []any
	if current, ok := root["input"]; ok {
		switch value := current.(type) {
		case []any:
			input = append(input, value...)
		case nil:
		default:
			input = append(input, value)
		}
	}
	var functionCallOutput any
	if len(replay.FunctionCallOutput) > 0 && json.Unmarshal(replay.FunctionCallOutput, &functionCallOutput) == nil {
		input = append([]any{functionCallOutput}, input...)
	}
	root["input"] = input
	resolved, err := json.Marshal(root)
	if err != nil {
		return body, nil
	}
	return resolved, nil
}

func hasCodexDedicatedImageReplayReference(body []byte) bool {
	responseID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	return strings.HasPrefix(responseID, codexDedicatedImageResponsePrefix)
}

func codexStringValue(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func writeJSONBytes(c *gin.Context, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.Status(http.StatusOK)
	_, err = c.Writer.Write(raw)
	return err
}

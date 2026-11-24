package service

import (
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
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		Model: plan.Model, Prompt: plan.Prompt, Size: codexDedicatedImagePlanSize(plan), N: 1,
		Quality: plan.Quality, ResponseFormat: "b64_json", Async: true,
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

func isBareCodexResponsesPath(path string) bool {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	return strings.HasSuffix(path, "/responses") && !strings.HasSuffix(path, "/responses/compact")
}

func dedicatedImageTierForModel(model string) string {
	tier, _ := CangyuanImageModelTier(normalizeDedicatedImageModel(model))
	return tier
}

func normalizeDedicatedImageModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	switch model {
	case "1k", "gpt-image-2-1k":
		return CangyuanImageModel1K
	case "2k", "gpt-image-2-2k":
		return CangyuanImageModel2K
	case "4k", "gpt-image-2-4k":
		return CangyuanImageModel4K
	}
	if IsCangyuanImageModel(model) {
		return model
	}
	return ""
}

func codexDedicatedImagePlanSize(plan *codexDedicatedImagePlan) string {
	if plan == nil {
		return ""
	}
	if size := strings.TrimSpace(plan.Size); size != "" {
		return size
	}
	return strings.TrimSpace(plan.AspectRatio)
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

package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

// channelMonitorAccountProbe is the production adapter for the injectable
// account probe seam. It deliberately owns no account-selection state: the
// repository supplies the persistent group pool and the existing forwarding
// services still own credential refresh, protocol conversion, proxy handling,
// and upstream request construction.
type channelMonitorAccountProbe struct {
	apiKeyRepo    APIKeyRepository
	accountRepo   AccountRepository
	settingRepo   SettingRepository
	gateway       *GatewayService
	openAIGateway *OpenAIGatewayService
	geminiGateway *GeminiMessagesCompatService
	slots         chan struct{}
	forwardFn     channelMonitorAccountForward
}

type channelMonitorAccountForward func(
	context.Context,
	*APIKey,
	*Account,
	ChannelMonitorAccountProbeRequest,
) (string, string, int, error)

// monitorAllGroupAccountRepository is intentionally optional: the account
// repository's public interface is shared by many test doubles and services.
// Production uses this seam to include disabled rows so they can be recorded
// as skipped instead of disappearing from the per-account coverage set.
type monitorAllGroupAccountRepository interface {
	ListAllByGroup(context.Context, int64) ([]Account, error)
}

func newChannelMonitorAccountProbe(
	apiKeyRepo APIKeyRepository,
	accountRepo AccountRepository,
	settingRepo SettingRepository,
	gateway *GatewayService,
	openAIGateway *OpenAIGatewayService,
	geminiGateway *GeminiMessagesCompatService,
) ChannelMonitorAccountProbe {
	return &channelMonitorAccountProbe{
		apiKeyRepo:    apiKeyRepo,
		accountRepo:   accountRepo,
		settingRepo:   settingRepo,
		gateway:       gateway,
		openAIGateway: openAIGateway,
		geminiGateway: geminiGateway,
		slots:         make(chan struct{}, monitorAccountProbeConcurrency),
	}
}

func (p *channelMonitorAccountProbe) Probe(
	ctx context.Context,
	request ChannelMonitorAccountProbeRequest,
) (*ChannelMonitorAccountProbeRun, bool, error) {
	groupID, apiKey, accounts, handled, err := p.resolveLocalCandidates(ctx, request)
	if !handled || err != nil {
		return nil, handled, err
	}

	request.GroupID = groupID
	roundCtx, cancel := context.WithTimeout(ctx, monitorAccountProbeRoundTimeoutForCount(len(accounts)))
	defer cancel()
	run := runChannelMonitorAccountProbes(roundCtx, request, accounts, p.attempt(apiKey))
	return run, true, nil
}

func (p *channelMonitorAccountProbe) resolveLocalCandidates(
	ctx context.Context,
	request ChannelMonitorAccountProbeRequest,
) (int64, *APIKey, []Account, bool, error) {
	if p == nil || p.apiKeyRepo == nil || p.accountRepo == nil || p.settingRepo == nil || request.Monitor == nil {
		return 0, nil, nil, false, nil
	}
	if !sameMonitorInstanceEndpoint(ctx, p.settingRepo, request.Monitor.Endpoint) {
		return 0, nil, nil, false, nil
	}

	apiKey, err := p.apiKeyRepo.GetByKey(ctx, request.Monitor.APIKey)
	if err != nil || apiKey == nil || !apiKey.IsActive() || apiKey.IsExpired() || apiKey.GroupID == nil || apiKey.Group == nil || !apiKey.Group.Hydrated || apiKey.Group.Status != StatusActive {
		// A key that cannot be authenticated and hydrated locally is not enough
		// evidence to enable account-directed behavior. Preserve old probing.
		return 0, nil, nil, false, nil
	}
	if apiKey.Group.ID != *apiKey.GroupID || !monitorProviderMatchesGroup(request.Monitor.Provider, apiKey.Group.Platform) {
		return 0, nil, nil, false, nil
	}

	var accounts []Account
	if allAccountsRepo, ok := p.accountRepo.(monitorAllGroupAccountRepository); ok {
		accounts, err = allAccountsRepo.ListAllByGroup(ctx, *apiKey.GroupID)
	} else {
		accounts, err = p.accountRepo.ListByGroup(ctx, *apiKey.GroupID)
	}
	if err != nil {
		// Candidate enumeration is part of the local-proof boundary. If it
		// cannot be completed, do not turn a transient repository failure into
		// a false account-level outage; let the original group-level checker
		// preserve external/legacy compatibility for this cycle.
		return 0, nil, nil, false, nil
	}
	platform := monitorPlatformForProvider(request.Monitor.Provider)
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		if accounts[i].Platform == platform {
			filtered = append(filtered, accounts[i])
		}
	}
	return *apiKey.GroupID, apiKey, filtered, true, nil
}

func sameMonitorInstanceEndpoint(ctx context.Context, settings SettingRepository, endpoint string) bool {
	if settings == nil {
		return false
	}
	configured, err := settings.GetValue(ctx, SettingKeyAPIBaseURL)
	if err != nil {
		return false
	}
	return sameEndpointOrigin(endpoint, configured)
}

func sameEndpointOrigin(left, right string) bool {
	leftURL, leftErr := url.Parse(strings.TrimSpace(left))
	rightURL, rightErr := url.Parse(strings.TrimSpace(right))
	if leftErr != nil || rightErr != nil || leftURL == nil || rightURL == nil {
		return false
	}
	if leftURL.Scheme == "" || rightURL.Scheme == "" || leftURL.Hostname() == "" || rightURL.Hostname() == "" {
		return false
	}
	if !strings.EqualFold(leftURL.Scheme, rightURL.Scheme) || !strings.EqualFold(leftURL.Hostname(), rightURL.Hostname()) {
		return false
	}
	return effectiveURLPort(leftURL) == effectiveURLPort(rightURL)
}

func effectiveURLPort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	if strings.EqualFold(value.Scheme, "https") {
		return "443"
	}
	return "80"
}

func monitorProviderMatchesGroup(provider, groupPlatform string) bool {
	return monitorPlatformForProvider(provider) != "" &&
		monitorPlatformForProvider(provider) == groupPlatform
}

func monitorPlatformForProvider(provider string) string {
	switch provider {
	case MonitorProviderOpenAI:
		return PlatformOpenAI
	case MonitorProviderAnthropic:
		return PlatformAnthropic
	case MonitorProviderGemini:
		return PlatformGemini
	case MonitorProviderGrok:
		return PlatformGrok
	case MonitorProviderKimi:
		return PlatformKimi
	case MonitorProviderZhipu:
		return PlatformZhipu
	case MonitorProviderDeepseek:
		return PlatformDeepseek
	default:
		return ""
	}
}

func (p *channelMonitorAccountProbe) attempt(apiKey *APIKey) channelMonitorAccountAttempt {
	return func(ctx context.Context, account *Account, request ChannelMonitorAccountProbeRequest) *CheckResult {
		result := newMonitorAttemptResult(request.Model)
		if p != nil && p.slots != nil {
			select {
			case p.slots <- struct{}{}:
				defer func() { <-p.slots }()
			case <-ctx.Done():
				result.Message = ctx.Err().Error()
				return result
			}
		}
		// Exclude local probe-slot queueing from the account's provider
		// latency. RoundDurationMs remains the separate end-to-end round metric.
		start := time.Now()
		forward := p.forwardAccount
		if p != nil && p.forwardFn != nil {
			forward = p.forwardFn
		}
		respText, rawBody, statusCode, err := forward(ctx, apiKey, account, request)
		latency := time.Since(start)
		latencyMs := int(latency / time.Millisecond)
		result.LatencyMs = &latencyMs
		adapter, apiMode, ok := providerAdapterFor(request.Monitor.Provider, checkAPIMode(request.Options))
		if !ok {
			err = fmt.Errorf("unsupported provider %q", request.Monitor.Provider)
		} else if err == nil && statusCode >= 200 && statusCode < 300 {
			respText = extractMonitorResponseText(adapter, []byte(rawBody))
			if request.Monitor.Provider == MonitorProviderOpenAI && apiMode == MonitorAPIModeResponses {
				respText = extractOpenAIResponsesText([]byte(rawBody))
			}
		}
		return classifyMonitorProviderResponse(
			result,
			respText,
			rawBody,
			statusCode,
			err,
			latency,
			request.Challenge,
			bodyOverrideMode(request.Options),
		)
	}
}

func newMonitorAttemptResult(model string) *CheckResult {
	return &CheckResult{Model: model, Status: MonitorStatusError, CheckedAt: time.Now()}
}

func (p *channelMonitorAccountProbe) forwardAccount(
	ctx context.Context,
	apiKey *APIKey,
	account *Account,
	request ChannelMonitorAccountProbeRequest,
) (string, string, int, error) {
	if p == nil || account == nil || request.Monitor == nil {
		return "", "", 0, fmt.Errorf("monitor account probe is not configured")
	}
	adapter, apiMode, ok := providerAdapterFor(request.Monitor.Provider, checkAPIMode(request.Options))
	if !ok {
		return "", "", 0, fmt.Errorf("unsupported provider %q", request.Monitor.Provider)
	}
	body, err := buildRequestBody(adapter, request.Monitor.Provider, apiMode, request.Model, request.Challenge.Prompt, request.Options)
	if err != nil {
		return "", "", 0, err
	}

	ctx = withChannelMonitorProbe(ctx)
	ctx = context.WithValue(ctx, ctxkey.Group, apiKey.Group)
	c, recorder := newChannelMonitorProbeContext(ctx, body, apiKey, request.Options)
	var forwardErr error
	switch account.Platform {
	case PlatformAnthropic:
		if p.gateway == nil {
			return "", "", 0, fmt.Errorf("anthropic gateway is not configured")
		}
		parsed, parseErr := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
		if parseErr != nil {
			return "", "", 0, parseErr
		}
		groupID := request.GroupID
		parsed.GroupID = &groupID
		_, forwardErr = p.gateway.Forward(ctx, c, account, parsed)
	case PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek:
		if p.openAIGateway == nil {
			return "", "", 0, fmt.Errorf("openai-compatible gateway is not configured")
		}
		if request.Monitor.Provider == MonitorProviderOpenAI && apiMode == MonitorAPIModeResponses {
			_, forwardErr = p.openAIGateway.Forward(ctx, c, account, body)
		} else {
			_, forwardErr = p.openAIGateway.ForwardAsChatCompletions(ctx, c, account, body, "", "")
		}
	case PlatformGemini:
		if p.geminiGateway == nil {
			return "", "", 0, fmt.Errorf("gemini gateway is not configured")
		}
		_, forwardErr = p.geminiGateway.ForwardNative(ctx, c, account, request.Model, "generateContent", false, body)
	default:
		return "", "", 0, fmt.Errorf("unsupported account platform %q", account.Platform)
	}

	statusCode := 0
	if recorder.Code > 0 && c.Writer.Written() {
		statusCode = recorder.Code
	}
	return "", recorder.Body.String(), statusCode, forwardErr
}

func newChannelMonitorProbeContext(ctx context.Context, body []byte, apiKey *APIKey, opts *CheckOptions) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "https://channel-monitor.invalid/v1/probe", bytes.NewReader(body))
	request = request.WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "sub2api-channel-monitor/1")
	for key, value := range mergeHeaders(nil, opts) {
		request.Header.Set(key, value)
	}
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	c.Set("api_key", apiKey)
	return c, recorder
}

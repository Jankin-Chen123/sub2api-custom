package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	pkgopenai "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func configureCodexNativeImageDelivery(request *service.CangyuanImageRequest) {
	if request == nil {
		return
	}
	// Codex receives a synchronous base64 response from Sub2API, while the
	// provider request remains an async task that the durable worker polls.
	// This path is independent from the workbench/API object-storage policy.
	request.ResponseFormat = "b64_json"
	request.Async = true
}

// normalizeCodexNativeImageRequest recognizes requests emitted by Codex's
// client-side image_gen extension. Older clients send gpt-image-2/size=auto;
// newer desktop clients send the selected dedicated tier directly. Both expect
// a synchronous data[].b64_json response. Restrict this behavior to official
// Codex clients (or the existing force_codex_cli compatibility switch) so
// ordinary Images API callers continue to follow the administrator's response
// format policy.
func (h *DedicatedImageHandler) normalizeCodexNativeImageRequest(c *gin.Context, parsed *service.OpenAIImagesRequest) bool {
	if h == nil || c == nil || parsed == nil {
		return false
	}
	if !h.forceCodexCLI && !pkgopenai.IsCodexOfficialClientByHeaders(c.GetHeader("User-Agent"), c.GetHeader("originator")) {
		return false
	}
	switch strings.TrimSpace(parsed.Model) {
	case "gpt-image-2":
		parsed.Model = service.CangyuanImageModel1K
	default:
		if !service.IsCangyuanImageModel(parsed.Model) {
			return false
		}
		// Keep the fixed tier explicitly selected by Codex.
	}
	if strings.EqualFold(strings.TrimSpace(parsed.Size), "auto") {
		parsed.Size = ""
		parsed.ExplicitSize = false
	}
	parsed.ResponseFormat = "b64_json"
	return true
}

// normalizeDedicatedImageAliasRequest routes the standard GPT image aliases to
// the smallest Cangyuan tier that can faithfully represent the request. It
// reports whether the model is a known alias separately from compatibility so
// callers can fail closed instead of accidentally selecting a general account.
func normalizeDedicatedImageAliasRequest(parsed *service.OpenAIImagesRequest) (bool, error) {
	if parsed == nil {
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(parsed.Model)) {
	case "gpt-image-1", "gpt-image-1.5", "gpt-image-2":
	default:
		return false, nil
	}
	if parsed.Stream {
		return true, dedicatedImageRequestError("image_stream_not_supported", "dedicated Cangyuan image models do not support streaming responses")
	}
	if parsed.N != 1 {
		return true, dedicatedImageRequestError("image_invalid_count", "dedicated Cangyuan image models require n=1")
	}
	if strings.TrimSpace(parsed.Background) != "" || strings.TrimSpace(parsed.OutputFormat) != "" ||
		strings.TrimSpace(parsed.Moderation) != "" || strings.TrimSpace(parsed.InputFidelity) != "" ||
		strings.TrimSpace(parsed.Style) != "" || parsed.OutputCompression != nil || parsed.PartialImages != nil {
		return true, dedicatedImageRequestError("image_option_not_supported", "the request contains image options that the dedicated Cangyuan provider cannot preserve")
	}

	candidate := *parsed
	hintedTier, err := normalizeDedicatedImageTierHints(&candidate)
	if err != nil {
		return true, err
	}
	operation := service.CangyuanImageOperationGeneration
	if candidate.IsEdits() {
		operation = service.CangyuanImageOperationEdit
	}
	models := []string{
		service.CangyuanImageModel1K,
		service.CangyuanImageModel2K,
		service.CangyuanImageModel4K,
	}
	if hintedTier != "" {
		models = []string{dedicatedImageTierModel(hintedTier)}
	}
	var validationErr error
	for _, model := range models {
		candidate.Model = model
		tier := dedicatedImageModelTier(model)
		candidate.SizeTier = tier
		request, requestErr := dedicatedCangyuanRequest(&candidate)
		if requestErr != nil {
			validationErr = requestErr
			continue
		}
		if validationErr = service.ValidateCangyuanImageRequest(operation, request); validationErr != nil {
			continue
		}
		*parsed = candidate
		return true, nil
	}
	if validationErr == nil {
		validationErr = dedicatedImageRequestError("image_invalid_size", "the requested image size does not match a supported Cangyuan tier")
	}
	return true, validationErr
}

func normalizeExplicitDedicatedImageRequest(parsed *service.OpenAIImagesRequest) error {
	if parsed == nil || !isDedicatedCangyuanModel(parsed.Model) {
		return nil
	}
	candidate := *parsed
	hintedTier, err := normalizeDedicatedImageTierHints(&candidate)
	if err != nil {
		return err
	}
	modelTier := dedicatedImageModelTier(candidate.Model)
	if hintedTier != "" && !strings.EqualFold(hintedTier, modelTier) {
		return dedicatedImageRequestError("image_invalid_size", "the requested image tier conflicts with the selected model")
	}
	candidate.SizeTier = modelTier
	*parsed = candidate
	return nil
}

func normalizeDedicatedImageTierHints(parsed *service.OpenAIImagesRequest) (string, error) {
	if parsed == nil {
		return "", nil
	}
	hintedTier := ""
	setTier := func(field, value string) error {
		tier, ok := dedicatedImageTierValue(value)
		if !ok {
			return dedicatedImageRequestError("image_invalid_size", field+" must be 1K, 2K, 4K, or an explicit WIDTHxHEIGHT size")
		}
		if hintedTier != "" && !strings.EqualFold(hintedTier, tier) {
			return dedicatedImageRequestError("image_invalid_size", "image tier fields conflict with each other")
		}
		hintedTier = tier
		return nil
	}
	setDimension := func(field, value string) error {
		value = strings.TrimSpace(value)
		if value == "" || strings.EqualFold(value, "auto") {
			return nil
		}
		if strings.TrimSpace(parsed.Size) != "" && !strings.EqualFold(strings.TrimSpace(parsed.Size), "auto") && !strings.EqualFold(strings.TrimSpace(parsed.Size), value) {
			return dedicatedImageRequestError("image_invalid_size", field+" conflicts with size")
		}
		parsed.Size = value
		parsed.ExplicitSize = true
		return nil
	}

	size := strings.TrimSpace(parsed.Size)
	switch {
	case size == "", strings.EqualFold(size, "auto"):
		parsed.Size = ""
		parsed.ExplicitSize = false
	case isDedicatedImageTierValue(size):
		if err := setTier("size", size); err != nil {
			return "", err
		}
		parsed.Size = ""
		parsed.ExplicitSize = false
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "image_size", value: parsed.ImageSize},
		{name: "output_resolution", value: parsed.OutputResolution},
	} {
		value := strings.TrimSpace(field.value)
		if value == "" || strings.EqualFold(value, "auto") {
			continue
		}
		if isDedicatedImageTierValue(value) {
			if err := setTier(field.name, value); err != nil {
				return "", err
			}
			continue
		}
		if err := setDimension(field.name, value); err != nil {
			return "", err
		}
	}
	parsed.ImageSize = ""
	parsed.OutputResolution = ""
	return hintedTier, nil
}

func dedicatedImageTierValue(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1k":
		return "1K", true
	case "2k":
		return "2K", true
	case "4k":
		return "4K", true
	}
	return service.CangyuanImageModelTier(value)
}

func isDedicatedImageTierValue(value string) bool {
	_, ok := dedicatedImageTierValue(value)
	return ok
}

func dedicatedImageTierModel(tier string) string {
	switch strings.ToUpper(strings.TrimSpace(tier)) {
	case "1K":
		return service.CangyuanImageModel1K
	case "2K":
		return service.CangyuanImageModel2K
	case "4K":
		return service.CangyuanImageModel4K
	default:
		return ""
	}
}

func dedicatedImageRequestError(code, message string) error {
	return &service.CangyuanAdapterError{Code: code, HTTPStatus: http.StatusBadRequest, Err: errors.New(message)}
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

func dedicatedCangyuanRequest(parsed *service.OpenAIImagesRequest) (service.CangyuanImageRequest, error) {
	if parsed == nil {
		return service.CangyuanImageRequest{}, fmt.Errorf("image request is missing")
	}
	size := strings.TrimSpace(parsed.Size)
	aspectRatio := strings.TrimSpace(parsed.AspectRatio)
	if size != "" && aspectRatio != "" {
		return service.CangyuanImageRequest{}, dedicatedImageRequestError("image_invalid_size", "size and aspect_ratio cannot both be set")
	}
	if size == "" {
		size = aspectRatio
	}
	request := service.CangyuanImageRequest{
		Model: parsed.Model, Prompt: parsed.Prompt, Size: size, N: parsed.N,
		Quality: parsed.Quality, ResponseFormat: parsed.ResponseFormat, Async: true,
		Multipart: parsed.Multipart,
		Images:    append([]string(nil), parsed.InputImageURLs...), Mask: parsed.MaskImageURL,
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

func isDedicatedCangyuanModel(model string) bool {
	return service.IsCangyuanImageModel(model)
}

func dedicatedImageModelTier(model string) string {
	tier, _ := service.CangyuanImageModelTier(model)
	return tier
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

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	CangyuanImageModel1K = "gpt-image-2-1k"
	CangyuanImageModel2K = "gpt-image-2-2k"
	CangyuanImageModel4K = "gpt-image-2-4k"

	cangyuanMinPixels              int64 = 655360
	cangyuanMaxEdge                      = 3840
	cangyuanMaxReferenceImages           = 9
	cangyuanMaxReferenceImageBytes int64 = 10 << 20
	cangyuanMaxPromptRunes               = 12000
	cangyuanMaxDecodedPixels       int64 = 16 << 20
	cangyuanDefaultResponseBytes   int64 = 96 << 20
)

type CangyuanImageOperation string

const (
	CangyuanImageOperationGeneration CangyuanImageOperation = "generation"
	CangyuanImageOperationEdit       CangyuanImageOperation = "edit"
)

type CangyuanImageRequest struct {
	Model            string   `json:"model"`
	Prompt           string   `json:"prompt"`
	Size             string   `json:"size,omitempty"`
	AspectRatio      string   `json:"aspect_ratio,omitempty"`
	N                int      `json:"n,omitempty"`
	Quality          string   `json:"quality,omitempty"`
	ResponseFormat   string   `json:"response_format,omitempty"`
	Async            bool     `json:"async,omitempty"`
	ImageSize        string   `json:"image_size,omitempty"`
	OutputResolution string   `json:"output_resolution,omitempty"`
	Images           []string `json:"images,omitempty"`
	Mask             string   `json:"mask,omitempty"`
	Multipart        bool     `json:"multipart,omitempty"`
}

type CangyuanImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type CangyuanImageResult struct {
	Created        int64
	Status         string
	UpstreamTaskID string
	Data           []CangyuanImageData
	Completed      bool
	Failed         bool
}

type CangyuanAdapterError struct {
	Code              string
	HTTPStatus        int
	Retryable         bool
	SubmissionUnknown bool
	Err               error
}

func (e *CangyuanAdapterError) Error() string {
	if e == nil {
		return "cangyuan image adapter error"
	}
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *CangyuanAdapterError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type CangyuanImageAdapter struct {
	baseURL          string
	apiKey           string
	httpClient       *http.Client
	maxResponseBytes int64
}

func NewCangyuanImageAdapter(baseURL, apiKey string, client *http.Client) (*CangyuanImageAdapter, error) {
	if _, err := buildCangyuanImageEndpoint(baseURL, "/v1/images/generations"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("cangyuan API key is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Minute}
	}
	return &CangyuanImageAdapter{
		baseURL:          strings.TrimSpace(baseURL),
		apiKey:           strings.TrimSpace(apiKey),
		httpClient:       client,
		maxResponseBytes: cangyuanDefaultResponseBytes,
	}, nil
}

func NewCangyuanImageAdapterFromAccount(account *Account, client *http.Client) (*CangyuanImageAdapter, error) {
	if account == nil || !account.IsImageOnly() {
		return nil, errors.New("a Cangyuan image_only account is required")
	}
	if _, err := NormalizeAccountPurposeExtra(account.Platform, account.Type, account.Credentials, account.Extra); err != nil {
		return nil, err
	}
	baseURL, _ := account.Credentials["base_url"].(string)
	apiKey, _ := account.Credentials["api_key"].(string)
	return NewCangyuanImageAdapter(baseURL, apiKey, client)
}

func (a *CangyuanImageAdapter) SubmitGeneration(ctx context.Context, request CangyuanImageRequest) (*CangyuanImageResult, error) {
	return a.submit(ctx, CangyuanImageOperationGeneration, request)
}

func (a *CangyuanImageAdapter) SubmitEdit(ctx context.Context, request CangyuanImageRequest) (*CangyuanImageResult, error) {
	return a.submit(ctx, CangyuanImageOperationEdit, request)
}

func (a *CangyuanImageAdapter) PollGeneration(ctx context.Context, upstreamTaskID string) (*CangyuanImageResult, error) {
	return a.poll(ctx, CangyuanImageOperationGeneration, upstreamTaskID)
}

func (a *CangyuanImageAdapter) PollEdit(ctx context.Context, upstreamTaskID string) (*CangyuanImageResult, error) {
	return a.poll(ctx, CangyuanImageOperationEdit, upstreamTaskID)
}

func (a *CangyuanImageAdapter) submit(ctx context.Context, operation CangyuanImageOperation, request CangyuanImageRequest) (*CangyuanImageResult, error) {
	request.Images = uniqueCangyuanImageReferences(request.Images)
	if err := ValidateCangyuanImageRequest(operation, request); err != nil {
		return nil, err
	}
	var body io.Reader
	contentType := "application/json"
	if operation == CangyuanImageOperationEdit && request.Multipart {
		encoded, formContentType, err := encodeCangyuanMultipartRequest(request)
		if err != nil {
			return nil, &CangyuanAdapterError{Code: "image_request_encode_failed", Err: err}
		}
		body, contentType = encoded, formContentType
	} else {
		encoded, err := json.Marshal(request)
		if err != nil {
			return nil, &CangyuanAdapterError{Code: "image_request_encode_failed", Err: err}
		}
		body = bytes.NewReader(encoded)
	}
	endpoint := "/v1/images/generations"
	if operation == CangyuanImageOperationEdit {
		endpoint = "/v1/images/edits"
	}
	result, err := a.do(ctx, http.MethodPost, endpoint, body, contentType)
	return result, err
}

func uniqueCangyuanImageReferences(images []string) []string {
	if len(images) == 0 {
		return nil
	}
	unique := make([]string, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, imageRef := range images {
		imageRef = strings.TrimSpace(imageRef)
		if imageRef == "" {
			unique = append(unique, imageRef)
			continue
		}
		if _, ok := seen[imageRef]; ok {
			continue
		}
		seen[imageRef] = struct{}{}
		unique = append(unique, imageRef)
	}
	return unique
}

func (a *CangyuanImageAdapter) poll(ctx context.Context, operation CangyuanImageOperation, upstreamTaskID string) (*CangyuanImageResult, error) {
	upstreamTaskID = strings.TrimSpace(upstreamTaskID)
	if upstreamTaskID == "" {
		return nil, &CangyuanAdapterError{Code: "image_task_id_required", Err: errors.New("upstream task ID is required")}
	}
	endpoint := "/v1/images/generations/" + url.PathEscape(upstreamTaskID)
	if operation == CangyuanImageOperationEdit {
		endpoint = "/v1/images/edits/" + url.PathEscape(upstreamTaskID)
	}
	return a.do(ctx, http.MethodGet, endpoint, nil, "")
}

func (a *CangyuanImageAdapter) do(ctx context.Context, method, endpoint string, body io.Reader, contentType string) (*CangyuanImageResult, error) {
	if a == nil || a.httpClient == nil {
		return nil, &CangyuanAdapterError{Code: "image_adapter_unavailable", Err: errors.New("adapter is not configured")}
	}
	target, err := buildCangyuanImageEndpoint(a.baseURL, endpoint)
	if err != nil {
		return nil, &CangyuanAdapterError{Code: "image_endpoint_invalid", Err: err}
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, &CangyuanAdapterError{Code: "image_request_invalid", Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	if body != nil {
		if strings.TrimSpace(contentType) == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, &CangyuanAdapterError{
			Code:              "image_upstream_timeout",
			Retryable:         method == http.MethodGet,
			HTTPStatus:        0,
			SubmissionUnknown: method == http.MethodPost,
			Err:               err,
		}
	}
	defer resp.Body.Close()

	maxBytes := a.maxResponseBytes
	if maxBytes <= 0 {
		maxBytes = cangyuanDefaultResponseBytes
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, &CangyuanAdapterError{Code: "image_upstream_read_failed", HTTPStatus: resp.StatusCode, Retryable: resp.StatusCode >= 500, Err: err}
	}
	if int64(len(raw)) > maxBytes {
		return nil, &CangyuanAdapterError{Code: "image_upstream_response_too_large", HTTPStatus: resp.StatusCode, Err: errors.New("upstream response exceeded the configured limit")}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		normalizedErr := normalizeCangyuanHTTPError(resp.StatusCode, raw)
		// A server/proxy error after a POST may be returned after the provider
		// accepted the generation. Treat that outcome as ambiguous rather than
		// allowing the Worker to submit the same image again. Explicit client
		// errors and rate limits remain safely retryable according to their
		// existing classification.
		if method == http.MethodPost && resp.StatusCode >= http.StatusInternalServerError {
			var adapterErr *CangyuanAdapterError
			if errors.As(normalizedErr, &adapterErr) && adapterErr != nil {
				adapterErr.SubmissionUnknown = true
			}
		}
		return nil, normalizedErr
	}
	return parseCangyuanImageResponse(raw)
}

func encodeCangyuanMultipartRequest(request CangyuanImageRequest) (io.Reader, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"model":             request.Model,
		"prompt":            request.Prompt,
		"size":              request.Size,
		"aspect_ratio":      request.AspectRatio,
		"quality":           request.Quality,
		"response_format":   request.ResponseFormat,
		"image_size":        request.ImageSize,
		"output_resolution": request.OutputResolution,
	}
	if request.N > 0 {
		fields["n"] = strconv.Itoa(request.N)
	}
	if request.Async {
		fields["async"] = "true"
	}
	for key, value := range fields {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", err
		}
	}
	for index, reference := range request.Images {
		if err := writeCangyuanMultipartImage(writer, "image", reference, index); err != nil {
			return nil, "", err
		}
	}
	if strings.TrimSpace(request.Mask) != "" {
		if err := writeCangyuanMultipartImage(writer, "mask", request.Mask, 0); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return bytes.NewReader(body.Bytes()), writer.FormDataContentType(), nil
}

func writeCangyuanMultipartImage(writer *multipart.Writer, field, reference string, index int) error {
	reference = strings.TrimSpace(reference)
	if strings.HasPrefix(strings.ToLower(reference), "data:") {
		data, contentType, err := decodeCangyuanImageDataURL(reference)
		if err != nil {
			return err
		}
		part, err := writer.CreateFormFile(field, cangyuanMultipartFilename(field, contentType, index))
		if err != nil {
			return err
		}
		_, err = part.Write(data)
		return err
	}
	if !isHTTPSURL(reference) {
		return errors.New("multipart image must be an HTTPS URL or image data URL")
	}
	return writer.WriteField(field, reference)
}

func cangyuanMultipartFilename(field, contentType string, index int) string {
	extension := ".bin"
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		extension = ".png"
	case "image/jpeg":
		extension = ".jpg"
	case "image/webp":
		extension = ".webp"
	}
	if field == "mask" {
		return "mask" + extension
	}
	return fmt.Sprintf("image-%d%s", index+1, extension)
}

func buildCangyuanImageEndpoint(baseURL, endpoint string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed == nil || !strings.EqualFold(parsed.Scheme, "https") || strings.TrimSpace(parsed.Host) == "" {
		return "", errors.New("a valid HTTPS Cangyuan base URL is required")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", errors.New("Cangyuan base URL must not contain credentials, query, or fragment")
	}
	basePath := strings.TrimRight(parsed.EscapedPath(), "/")
	endpoint = "/" + strings.TrimLeft(endpoint, "/")
	if strings.HasSuffix(strings.ToLower(basePath), "/v1") && strings.HasPrefix(strings.ToLower(endpoint), "/v1/") {
		endpoint = endpoint[len("/v1"):]
	}
	rawPath := basePath + endpoint
	decodedPath, err := url.PathUnescape(rawPath)
	if err != nil {
		return "", errors.New("Cangyuan endpoint contains invalid path escaping")
	}
	parsed.Path = decodedPath
	parsed.RawPath = rawPath
	return parsed.String(), nil
}

func ValidateCangyuanImageRequest(operation CangyuanImageOperation, request CangyuanImageRequest) error {
	tier, maxPixels, ok := cangyuanImageTier(request.Model)
	if !ok {
		return &CangyuanAdapterError{Code: "image_model_not_allowed", HTTPStatus: http.StatusBadRequest, Err: errors.New("unsupported Cangyuan image model")}
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return &CangyuanAdapterError{Code: "image_prompt_required", HTTPStatus: http.StatusBadRequest, Err: errors.New("prompt is required")}
	}
	if utf8.RuneCountInString(request.Prompt) > cangyuanMaxPromptRunes {
		return &CangyuanAdapterError{Code: "image_prompt_too_long", HTTPStatus: http.StatusBadRequest, Err: fmt.Errorf("prompt must not exceed %d characters", cangyuanMaxPromptRunes)}
	}
	if request.N != 0 && request.N != 1 {
		return &CangyuanAdapterError{Code: "image_invalid_count", HTTPStatus: http.StatusBadRequest, Err: errors.New("Cangyuan image tier models require n=1")}
	}
	if request.ResponseFormat != "" && request.ResponseFormat != "url" && request.ResponseFormat != "b64_json" {
		return &CangyuanAdapterError{Code: "image_invalid_response_format", HTTPStatus: http.StatusBadRequest, Err: errors.New("response_format must be url or b64_json")}
	}
	switch strings.ToLower(strings.TrimSpace(request.Quality)) {
	case "", "auto", "low", "medium", "high":
	default:
		return &CangyuanAdapterError{Code: "image_invalid_quality", HTTPStatus: http.StatusBadRequest, Err: errors.New("quality must be low, medium, high, or auto")}
	}
	if strings.TrimSpace(request.Size) != "" && strings.TrimSpace(request.AspectRatio) != "" {
		return &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: errors.New("size and aspect_ratio cannot both be set")}
	}
	for field, value := range map[string]string{
		"image_size":        request.ImageSize,
		"output_resolution": request.OutputResolution,
	} {
		if value != "" && !strings.EqualFold(strings.TrimSpace(value), tier) {
			return &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: fmt.Errorf("%s conflicts with the selected model tier", field)}
		}
	}
	if request.Size != "" {
		if strings.Contains(request.Size, ":") {
			if err := validateCangyuanAspectRatio(request.Size); err != nil {
				return err
			}
		} else {
			width, height, err := parseCangyuanImageSize(request.Size)
			if err != nil {
				return err
			}
			pixels := int64(width) * int64(height)
			if width%16 != 0 || height%16 != 0 || width > cangyuanMaxEdge || height > cangyuanMaxEdge || max(width, height) > 3*min(width, height) || pixels < cangyuanMinPixels || pixels > maxPixels {
				return &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: errors.New("size violates the selected Cangyuan model limits")}
			}
		}
	}
	if strings.TrimSpace(request.AspectRatio) != "" {
		if err := validateCangyuanAspectRatio(request.AspectRatio); err != nil {
			return err
		}
	}
	if len(request.Images) > 0 {
		if err := validateCangyuanReferenceValues(request.Images); err != nil {
			return err
		}
	}
	if operation == CangyuanImageOperationEdit {
		if len(request.Images) == 0 {
			return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("at least one reference image is required for edits")}
		}
		if request.Mask != "" && !isHTTPSURL(request.Mask) && !strings.HasPrefix(strings.ToLower(request.Mask), "data:image/png;base64,") {
			return &CangyuanAdapterError{Code: "image_invalid_mask", HTTPStatus: http.StatusBadRequest, Err: errors.New("mask must be an HTTPS URL or PNG data URL")}
		}
		if strings.HasPrefix(strings.ToLower(request.Mask), "data:") {
			data, contentType, err := decodeCangyuanImageDataURL(request.Mask)
			if err != nil || contentType != "image/png" {
				return &CangyuanAdapterError{Code: "image_invalid_mask", HTTPStatus: http.StatusBadRequest, Err: errors.New("mask data URL must contain a valid PNG no larger than 10 MB")}
			}
			decoded, format, decodeErr := image.Decode(bytes.NewReader(data))
			if decodeErr != nil || format != "png" || !colorModelHasAlpha(decoded.ColorModel()) {
				return &CangyuanAdapterError{Code: "image_invalid_mask", HTTPStatus: http.StatusBadRequest, Err: errors.New("mask PNG must have an alpha channel")}
			}
		}
	}
	return nil
}

func validateCangyuanAspectRatio(value string) error {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: errors.New("aspect_ratio must use WIDTH:HEIGHT")}
	}
	width, widthErr := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	height, heightErr := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 || width > 3*height || height > 3*width {
		return &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: errors.New("aspect_ratio must be positive and no wider than 3:1")}
	}
	return nil
}

func cangyuanImageTier(model string) (string, int64, bool) {
	switch strings.TrimSpace(model) {
	case CangyuanImageModel1K:
		return "1K", 1048576, true
	case CangyuanImageModel2K:
		return "2K", 4194304, true
	case CangyuanImageModel4K:
		return "4K", 8294400, true
	default:
		return "", 0, false
	}
}

func parseCangyuanImageSize(value string) (int, int, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), "x")
	if len(parts) != 2 {
		return 0, 0, &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: errors.New("size must use WIDTHxHEIGHT")}
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return 0, 0, &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: errors.New("size must contain positive integer dimensions")}
	}
	return width, height, nil
}

func validateCangyuanReferenceValues(images []string) error {
	unique := make(map[string]struct{}, len(images))
	for _, value := range images {
		value = strings.TrimSpace(value)
		if value == "" {
			return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image cannot be empty")}
		}
		if !isHTTPSURL(value) && !strings.HasPrefix(strings.ToLower(value), "data:image/") {
			return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference images must use HTTPS or data URLs")}
		}
		if strings.HasPrefix(strings.ToLower(value), "data:") {
			data, contentType, err := decodeCangyuanImageDataURL(value)
			if err != nil {
				return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: err}
			}
			_, format, decodeErr := image.DecodeConfig(bytes.NewReader(data))
			if decodeErr != nil {
				return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image data URL is not decodable")}
			}
			detectedType := "image/" + format
			if format == "jpeg" {
				detectedType = "image/jpeg"
			}
			if !strings.EqualFold(contentType, detectedType) {
				return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image MIME type does not match its content")}
			}
		}
		unique[value] = struct{}{}
	}
	if len(unique) > cangyuanMaxReferenceImages {
		return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("at most 9 unique reference images are allowed")}
	}
	return nil
}

func decodeCangyuanImageDataURL(value string) ([]byte, string, error) {
	header, payload, found := strings.Cut(strings.TrimSpace(value), ",")
	if !found || !strings.HasPrefix(strings.ToLower(header), "data:image/") || !strings.HasSuffix(strings.ToLower(header), ";base64") {
		return nil, "", errors.New("image data URL must use base64 encoding")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(header[len("data:"):], ";base64")))
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload))
	data, err := io.ReadAll(io.LimitReader(decoder, cangyuanMaxReferenceImageBytes+1))
	if err != nil {
		return nil, "", errors.New("image data URL contains invalid base64")
	}
	if int64(len(data)) > cangyuanMaxReferenceImageBytes {
		return nil, "", errors.New("reference image exceeds 10 MB")
	}
	return data, contentType, nil
}

func isHTTPSURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed != nil && strings.EqualFold(parsed.Scheme, "https") && parsed.Host != ""
}

type CangyuanResolvedImageAsset struct {
	ContentType string
	Data        []byte
}

func ValidateCangyuanResolvedEditAssets(images []CangyuanResolvedImageAsset, mask *CangyuanResolvedImageAsset) error {
	if len(images) == 0 || len(images) > cangyuanMaxReferenceImages {
		return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("edits require 1 to 9 resolved reference images")}
	}
	var firstFormat string
	var firstConfig image.Config
	for index, asset := range images {
		if int64(len(asset.Data)) > cangyuanMaxReferenceImageBytes {
			return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image exceeds 10 MB")}
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(asset.Data))
		if err != nil || validateCangyuanDecodedImageDimensions(config.Width, config.Height) != nil {
			return &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image is not decodable")}
		}
		if index == 0 {
			firstFormat = format
			firstConfig = config
		}
	}
	if mask == nil {
		return nil
	}
	if int64(len(mask.Data)) > cangyuanMaxReferenceImageBytes || !strings.EqualFold(strings.TrimSpace(mask.ContentType), "image/png") {
		return &CangyuanAdapterError{Code: "image_invalid_mask", HTTPStatus: http.StatusBadRequest, Err: errors.New("mask must be a PNG no larger than 10 MB")}
	}
	maskConfig, _, configErr := image.DecodeConfig(bytes.NewReader(mask.Data))
	if configErr != nil || validateCangyuanDecodedImageDimensions(maskConfig.Width, maskConfig.Height) != nil {
		return &CangyuanAdapterError{Code: "image_invalid_mask", HTTPStatus: http.StatusBadRequest, Err: errors.New("mask dimensions exceed the safe image limit")}
	}
	decoded, format, err := image.Decode(bytes.NewReader(mask.Data))
	if err != nil || format != "png" || firstFormat != "png" {
		return &CangyuanAdapterError{Code: "image_invalid_mask", HTTPStatus: http.StatusBadRequest, Err: errors.New("mask and first reference image must both be PNG")}
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != firstConfig.Width || bounds.Dy() != firstConfig.Height {
		return &CangyuanAdapterError{Code: "image_invalid_mask", HTTPStatus: http.StatusBadRequest, Err: errors.New("mask dimensions must match the first reference image")}
	}
	if !colorModelHasAlpha(decoded.ColorModel()) {
		return &CangyuanAdapterError{Code: "image_invalid_mask", HTTPStatus: http.StatusBadRequest, Err: errors.New("mask PNG must have an alpha channel")}
	}
	return nil
}

func colorModelHasAlpha(model color.Model) bool {
	return model == color.RGBAModel || model == color.RGBA64Model || model == color.NRGBAModel || model == color.NRGBA64Model || model == color.AlphaModel || model == color.Alpha16Model
}

func validateCangyuanDecodedImageDimensions(width, height int) error {
	if width <= 0 || height <= 0 {
		return errors.New("image dimensions must be positive")
	}
	if int64(width) > cangyuanMaxEdge*4 || int64(height) > cangyuanMaxEdge*4 {
		return errors.New("image dimensions exceed the safe limit")
	}
	if int64(width) > cangyuanMaxDecodedPixels/int64(height) {
		return errors.New("image pixel count exceeds the safe limit")
	}
	return nil
}

type cangyuanImageResponse struct {
	ID      string               `json:"id"`
	TaskID  string               `json:"task_id"`
	Created int64                `json:"created"`
	Status  string               `json:"status"`
	State   string               `json:"state"`
	Data    []CangyuanImageData  `json:"data"`
	Error   *cangyuanErrorObject `json:"error"`
}

type cangyuanErrorObject struct {
	Code string `json:"code"`
	Type string `json:"type"`
}

func parseCangyuanImageResponse(raw []byte) (*CangyuanImageResult, error) {
	var response cangyuanImageResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, &CangyuanAdapterError{Code: "image_upstream_invalid_response", Err: errors.New("upstream returned invalid JSON")}
	}
	if response.Error != nil {
		return nil, &CangyuanAdapterError{Code: normalizeCangyuanErrorCode(response.Error.Code, response.Error.Type), Err: errors.New("upstream returned an error")}
	}
	status := strings.ToLower(strings.TrimSpace(response.Status))
	if status == "" {
		status = strings.ToLower(strings.TrimSpace(response.State))
	}
	taskID := strings.TrimSpace(response.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(response.ID)
	}
	result := &CangyuanImageResult{
		Created:        response.Created,
		Status:         status,
		UpstreamTaskID: taskID,
		Data:           response.Data,
	}
	if len(response.Data) > 0 {
		if len(response.Data) != 1 {
			return nil, &CangyuanAdapterError{Code: "image_upstream_invalid_response", Err: errors.New("Cangyuan tier response contained more than one image")}
		}
		for _, item := range response.Data {
			if strings.TrimSpace(item.URL) == "" && strings.TrimSpace(item.B64JSON) == "" {
				return nil, &CangyuanAdapterError{Code: "image_upstream_invalid_response", Err: errors.New("completed image item has no URL or base64 data")}
			}
		}
		result.Completed = true
		return result, nil
	}
	switch status {
	case "queued", "pending", "submitted", "processing", "running", "in_progress":
		if taskID == "" {
			return nil, &CangyuanAdapterError{Code: "image_upstream_invalid_response", Err: errors.New("asynchronous response has no task ID")}
		}
		return result, nil
	case "failed", "error", "cancelled", "canceled":
		result.Failed = true
		return result, nil
	case "completed", "succeeded", "success":
		return nil, &CangyuanAdapterError{Code: "image_upstream_invalid_response", Err: errors.New("completed response has no image data")}
	default:
		return nil, &CangyuanAdapterError{Code: "image_upstream_invalid_response", Err: errors.New("upstream response has neither image data nor a recognized task status")}
	}
}

func normalizeCangyuanHTTPError(status int, raw []byte) error {
	var response cangyuanImageResponse
	_ = json.Unmarshal(raw, &response)
	upstreamCode := ""
	upstreamType := ""
	if response.Error != nil {
		upstreamCode = response.Error.Code
		upstreamType = response.Error.Type
	}
	code := normalizeCangyuanErrorCode(upstreamCode, upstreamType)
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		code = "image_upstream_auth_failed"
	} else if status == http.StatusTooManyRequests {
		code = "image_upstream_rate_limited"
	} else if status >= 500 {
		code = "image_upstream_unavailable"
	} else if code == "image_upstream_rejected" && status >= 400 && status < 500 {
		code = "image_upstream_rejected"
	}
	return &CangyuanAdapterError{
		Code:       code,
		HTTPStatus: status,
		Retryable:  status == http.StatusTooManyRequests || status >= 500,
		Err:        errors.New("Cangyuan rejected the image request"),
	}
}

func normalizeCangyuanErrorCode(code, errorType string) string {
	value := strings.ToLower(strings.TrimSpace(code + " " + errorType))
	switch {
	case strings.Contains(value, "auth"), strings.Contains(value, "api_key"), strings.Contains(value, "unauthorized"):
		return "image_upstream_auth_failed"
	case strings.Contains(value, "rate"), strings.Contains(value, "quota"):
		return "image_upstream_rate_limited"
	case strings.Contains(value, "size"), strings.Contains(value, "resolution"):
		return "image_invalid_size"
	case strings.Contains(value, "model"):
		return "image_model_not_allowed"
	default:
		return "image_upstream_rejected"
	}
}

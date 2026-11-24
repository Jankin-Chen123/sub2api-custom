package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const cangyuanDefaultResponseBytes int64 = 96 << 20

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
	var err error
	request, err = NormalizeCangyuanImageRequest(request)
	if err != nil {
		return nil, err
	}
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
	defer func() { _ = resp.Body.Close() }()

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
		"model":           request.Model,
		"prompt":          request.Prompt,
		"size":            request.Size,
		"quality":         request.Quality,
		"response_format": request.ResponseFormat,
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
		return "", errors.New("cangyuan base URL must not contain credentials, query, or fragment")
	}
	basePath := strings.TrimRight(parsed.EscapedPath(), "/")
	endpoint = "/" + strings.TrimLeft(endpoint, "/")
	if strings.HasSuffix(strings.ToLower(basePath), "/v1") && strings.HasPrefix(strings.ToLower(endpoint), "/v1/") {
		endpoint = endpoint[len("/v1"):]
	}
	rawPath := basePath + endpoint
	decodedPath, err := url.PathUnescape(rawPath)
	if err != nil {
		return "", errors.New("cangyuan endpoint contains invalid path escaping")
	}
	parsed.Path = decodedPath
	parsed.RawPath = rawPath
	return parsed.String(), nil
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
			return nil, &CangyuanAdapterError{Code: "image_upstream_invalid_response", Err: errors.New("cangyuan tier response contained more than one image")}
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
		Err:        errors.New("cangyuan rejected the image request"),
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

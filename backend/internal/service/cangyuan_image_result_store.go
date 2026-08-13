package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

const (
	cangyuanImageOutputMaxBytes int64 = 64 << 20
	cangyuanImageOutputTimeout        = 90 * time.Second
)

type ImageGenerationResultStore interface {
	Store(ctx context.Context, jobID string, data []CangyuanImageData) (objectRefs []string, actualSize string, err error)
}

type ImageGenerationResultReader interface {
	Open(ctx context.Context, objectRef string) (body io.ReadCloser, contentType string, contentLength int64, err error)
}

// ResolvingImageGenerationResultStore follows the existing runtime-editable
// image storage settings instead of capturing S3 credentials at process start.
type ResolvingImageGenerationResultStore struct {
	settings *ImageStorageSettingService
	prefix   string
	maxBytes int64
}

func NewResolvingImageGenerationResultStore(settings *ImageStorageSettingService, prefix string, maxBytes int64) *ResolvingImageGenerationResultStore {
	return &ResolvingImageGenerationResultStore{settings: settings, prefix: prefix, maxBytes: maxBytes}
}

func (s *ResolvingImageGenerationResultStore) Store(ctx context.Context, jobID string, data []CangyuanImageData) ([]string, string, error) {
	if s == nil || s.settings == nil {
		return nil, "", errors.New("image result storage settings are unavailable")
	}
	uploader, enabled := s.settings.resolve()
	if !enabled || uploader == nil || uploader.storage == nil {
		return nil, "", errors.New("image result storage is disabled or incomplete")
	}
	maxBytes := s.maxBytes
	if maxBytes <= 0 {
		maxBytes = uploader.maxDownloadBytes
	}
	return NewCangyuanImageResultStore(uploader.storage, s.prefix, maxBytes).Store(ctx, jobID, data)
}

func (s *ResolvingImageGenerationResultStore) Open(ctx context.Context, objectRef string) (io.ReadCloser, string, int64, error) {
	if s == nil || s.settings == nil {
		return nil, "", 0, errors.New("image result storage settings are unavailable")
	}
	uploader, enabled := s.settings.resolve()
	if !enabled || uploader == nil || uploader.storage == nil {
		return nil, "", 0, errors.New("image result storage is disabled or incomplete")
	}
	reader, ok := uploader.storage.(ImageStorageReader)
	if !ok {
		return nil, "", 0, errors.New("configured image storage does not support authenticated reads")
	}
	return reader.Open(ctx, strings.TrimSpace(objectRef))
}

func (s *ResolvingImageGenerationResultStore) Delete(ctx context.Context, objectRef string) error {
	if s == nil || s.settings == nil {
		return errors.New("image result storage settings are unavailable")
	}
	uploader, enabled := s.settings.resolve()
	if !enabled || uploader == nil || uploader.storage == nil {
		return errors.New("image result storage is disabled or incomplete")
	}
	deleter, ok := uploader.storage.(ImageStorageDeleter)
	if !ok {
		return errors.New("configured image storage does not support deletion")
	}
	return deleter.Delete(ctx, strings.TrimSpace(objectRef))
}

// CangyuanImageResultStore resolves a completed upstream URL/base64 result,
// validates the decoded image, and writes it under a deterministic object key.
// Retrying the storing phase overwrites the same key and never regenerates.
type CangyuanImageResultStore struct {
	storage       ImageStorage
	httpClient    *http.Client
	maxBytes      int64
	hostValidator cangyuanHostValidator
	prefix        string
}

func NewCangyuanImageResultStore(storage ImageStorage, prefix string, maxBytes int64) *CangyuanImageResultStore {
	client := newSSRFSafeHTTPClient(cangyuanImageOutputTimeout)
	client.CheckRedirect = cangyuanImageRedirectPolicy(isPrivateOrLoopbackHost)
	if maxBytes <= 0 || maxBytes > cangyuanImageOutputMaxBytes {
		maxBytes = cangyuanImageOutputMaxBytes
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix != "" {
		prefix += "/"
	}
	return &CangyuanImageResultStore{
		storage:       storage,
		httpClient:    client,
		maxBytes:      maxBytes,
		hostValidator: isPrivateOrLoopbackHost,
		prefix:        prefix,
	}
}

func (s *CangyuanImageResultStore) Store(ctx context.Context, jobID string, data []CangyuanImageData) ([]string, string, error) {
	if s == nil || s.storage == nil || s.httpClient == nil {
		return nil, "", errors.New("image result storage is not configured")
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || len(data) != 1 {
		return nil, "", errors.New("a single image result and job ID are required")
	}
	if streamWriter, ok := s.storage.(ImageStorageStreamWriter); ok {
		asset, err := s.resolveStream(ctx, data[0])
		if err != nil {
			return nil, "", err
		}
		defer func() {
			_ = asset.file.Close()
			_ = os.Remove(asset.file.Name())
		}()
		key := s.prefix + jobID + "/0" + extensionForContentType(asset.ContentType)
		if _, err := streamWriter.SaveStream(ctx, key, asset.ContentType, asset.file, asset.Size); err != nil {
			return nil, "", errors.New("image object storage write failed")
		}
		return []string{key}, strconv.Itoa(asset.Width) + "x" + strconv.Itoa(asset.Height), nil
	}
	asset, err := s.resolve(ctx, data[0])
	if err != nil {
		return nil, "", err
	}
	key := s.prefix + jobID + "/0" + extensionForContentType(asset.ContentType)
	if _, err := s.storage.Save(ctx, key, asset.ContentType, asset.Data); err != nil {
		return nil, "", errors.New("image object storage write failed")
	}
	return []string{key}, strconv.Itoa(asset.Width) + "x" + strconv.Itoa(asset.Height), nil
}

type cangyuanStreamAsset struct {
	file        *os.File
	Size        int64
	ContentType string
	Width       int
	Height      int
}

// resolveStream stages the upstream result in a bounded temporary file. This
// avoids holding a decoded 4K result in a second []byte while the object store
// upload consumes it. The file is deleted by Store on every exit path.
func (s *CangyuanImageResultStore) resolveStream(ctx context.Context, item CangyuanImageData) (*cangyuanStreamAsset, error) {
	file, err := os.CreateTemp("", "sub2api-cangyuan-result-*")
	if err != nil {
		return nil, errors.New("image result temporary storage is unavailable")
	}
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}

	declaredType, err := s.copyResultToFile(ctx, file, item)
	if err != nil {
		cleanup()
		return nil, err
	}
	stat, err := file.Stat()
	if err != nil || stat.Size() <= 0 {
		cleanup()
		return nil, errors.New("image result is empty")
	}
	if stat.Size() > s.maxBytes {
		cleanup()
		return nil, errors.New("image result exceeds the configured size limit")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, errors.New("image result temporary file could not be rewound")
	}
	config, format, err := image.DecodeConfig(file)
	if err != nil || validateCangyuanDecodedImageDimensions(config.Width, config.Height) != nil {
		cleanup()
		return nil, errors.New("image result is not decodable")
	}
	contentType := cangyuanOutputContentType(format)
	declaredType = strings.ToLower(strings.TrimSpace(declaredType))
	if declaredType != "" && declaredType != "application/octet-stream" && declaredType != contentType {
		cleanup()
		return nil, fmt.Errorf("image result MIME type does not match decoded content")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, errors.New("image result temporary file could not be rewound")
	}
	return &cangyuanStreamAsset{
		file: file, Size: stat.Size(), ContentType: contentType,
		Width: config.Width, Height: config.Height,
	}, nil
}

func (s *CangyuanImageResultStore) copyResultToFile(ctx context.Context, file *os.File, item CangyuanImageData) (string, error) {
	if raw := strings.TrimSpace(item.B64JSON); raw != "" {
		declaredType := ""
		payload := raw
		if strings.HasPrefix(strings.ToLower(raw), "data:") {
			var err error
			declaredType, payload, err = parseCangyuanOutputDataURL(raw)
			if err != nil {
				return "", err
			}
		}
		decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload))
		written, err := io.Copy(file, io.LimitReader(decoder, s.maxBytes+1))
		if err != nil {
			return "", errors.New("image result base64 is invalid")
		}
		if written > s.maxBytes {
			return "", errors.New("image result exceeds the configured size limit")
		}
		return declaredType, nil
	}

	rawURL := strings.TrimSpace(item.URL)
	if rawURL == "" {
		return "", errors.New("completed image result has no content")
	}
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		declaredType, payload, err := parseCangyuanOutputDataURL(rawURL)
		if err != nil {
			return "", err
		}
		decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload))
		written, err := io.Copy(file, io.LimitReader(decoder, s.maxBytes+1))
		if err != nil {
			return "", errors.New("image result base64 is invalid")
		}
		if written > s.maxBytes {
			return "", errors.New("image result exceeds the configured size limit")
		}
		return declaredType, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("image result URL failed the HTTPS safety policy")
	}
	blocked, err := s.hostValidator(ctx, parsed.Hostname())
	if err != nil || blocked {
		return "", errors.New("image result host failed the SSRF safety policy")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", errors.New("image result download request is invalid")
	}
	req.Header.Set("Accept", "image/*")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", errors.New("image result download failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", errors.New("image result download returned an unsuccessful status")
	}
	written, err := io.Copy(file, io.LimitReader(resp.Body, s.maxBytes+1))
	if err != nil {
		return "", errors.New("image result body could not be read")
	}
	if written > s.maxBytes {
		return "", errors.New("image result exceeds the configured size limit")
	}
	return strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])), nil
}

func (s *CangyuanImageResultStore) resolve(ctx context.Context, item CangyuanImageData) (*CangyuanResolvedAsset, error) {
	if raw := strings.TrimSpace(item.B64JSON); raw != "" {
		data, err := decodeCangyuanOutputBase64(raw, s.maxBytes)
		if err != nil {
			return nil, err
		}
		return inspectCangyuanOutputAsset(data, "")
	}
	rawURL := strings.TrimSpace(item.URL)
	if rawURL == "" {
		return nil, errors.New("completed image result has no content")
	}
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		data, declaredType, err := decodeCangyuanOutputDataURL(rawURL, s.maxBytes)
		if err != nil {
			return nil, err
		}
		return inspectCangyuanOutputAsset(data, declaredType)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("image result URL failed the HTTPS safety policy")
	}
	blocked, err := s.hostValidator(ctx, parsed.Hostname())
	if err != nil || blocked {
		return nil, errors.New("image result host failed the SSRF safety policy")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, errors.New("image result download request is invalid")
	}
	req.Header.Set("Accept", "image/*")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, errors.New("image result download failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("image result download returned an unsuccessful status")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBytes+1))
	if err != nil {
		return nil, errors.New("image result body could not be read")
	}
	if int64(len(raw)) > s.maxBytes {
		return nil, errors.New("image result exceeds the configured size limit")
	}
	declaredType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	return inspectCangyuanOutputAsset(raw, declaredType)
}

func decodeCangyuanOutputBase64(raw string, maxBytes int64) ([]byte, error) {
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		data, _, err := decodeCangyuanOutputDataURL(raw, maxBytes)
		return data, err
	}
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(raw))
	data, err := io.ReadAll(io.LimitReader(decoder, maxBytes+1))
	if err != nil {
		return nil, errors.New("image result base64 is invalid")
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("image result exceeds the configured size limit")
	}
	return data, nil
}

func decodeCangyuanOutputDataURL(raw string, maxBytes int64) ([]byte, string, error) {
	declaredType, payload, err := parseCangyuanOutputDataURL(raw)
	if err != nil {
		return nil, "", err
	}
	data, err := decodeCangyuanOutputBase64(payload, maxBytes)
	if err != nil {
		return nil, "", err
	}
	return data, strings.ToLower(declaredType), nil
}

func parseCangyuanOutputDataURL(raw string) (string, string, error) {
	header, payload, ok := strings.Cut(strings.TrimSpace(raw), ",")
	if !ok || !strings.HasPrefix(strings.ToLower(header), "data:image/") {
		return "", "", errors.New("image result data URL is invalid")
	}
	mediaTypeHeader := header[len("data:"):]
	separator := strings.LastIndex(mediaTypeHeader, ";")
	if separator < 0 || !strings.EqualFold(strings.TrimSpace(mediaTypeHeader[separator+1:]), "base64") {
		return "", "", errors.New("image result data URL is invalid")
	}
	mediaTypeHeader = strings.TrimSpace(mediaTypeHeader[:separator])
	declaredType, _, err := mime.ParseMediaType(mediaTypeHeader)
	if err != nil || !strings.HasPrefix(strings.ToLower(declaredType), "image/") {
		return "", "", errors.New("image result data URL media type is invalid")
	}
	return strings.ToLower(declaredType), payload, nil
}

func inspectCangyuanOutputAsset(data []byte, declaredType string) (*CangyuanResolvedAsset, error) {
	if len(data) == 0 {
		return nil, errors.New("image result is empty")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || validateCangyuanDecodedImageDimensions(config.Width, config.Height) != nil {
		return nil, errors.New("image result is not decodable")
	}
	contentType := cangyuanOutputContentType(format)
	declaredType = strings.ToLower(strings.TrimSpace(declaredType))
	if declaredType != "" && declaredType != "application/octet-stream" && declaredType != contentType {
		return nil, fmt.Errorf("image result MIME type does not match decoded content")
	}
	return &CangyuanResolvedAsset{ContentType: contentType, Data: data, Width: config.Width, Height: config.Height}, nil
}

func cangyuanOutputContentType(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "jpeg" {
		return "image/jpeg"
	}
	return "image/" + format
}

// normalizeCodexImageBase64 validates a provider b64_json result without
// materializing a second decoded image buffer. Codex's Responses contract
// wants the bare base64 payload, so a provider data-URL wrapper is removed,
// while the original encoded bytes are otherwise reused unchanged.
func normalizeCodexImageBase64(raw string, maxBytes int64) (*CodexImageResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("completed Codex image result has no base64 content")
	}
	declaredType := ""
	payload := raw
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		var err error
		declaredType, payload, err = parseCangyuanOutputDataURL(raw)
		if err != nil {
			return nil, err
		}
	}
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, errors.New("completed Codex image result has no base64 content")
	}
	if maxBytes <= 0 || maxBytes > cangyuanImageOutputMaxBytes {
		maxBytes = cangyuanImageOutputMaxBytes
	}

	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload))
	limited := &countingReader{reader: io.LimitReader(decoder, maxBytes+1)}
	config, format, err := image.DecodeConfig(limited)
	if err != nil || validateCangyuanDecodedImageDimensions(config.Width, config.Height) != nil {
		return nil, errors.New("image result is not decodable")
	}
	if _, err := io.Copy(io.Discard, limited); err != nil {
		return nil, errors.New("image result base64 is invalid")
	}
	if limited.read > maxBytes {
		return nil, errors.New("image result exceeds the configured size limit")
	}
	contentType := cangyuanOutputContentType(format)
	declaredType = strings.ToLower(strings.TrimSpace(declaredType))
	if declaredType != "" && declaredType != "application/octet-stream" && declaredType != contentType {
		return nil, errors.New("image result MIME type does not match decoded content")
	}
	return &CodexImageResult{
		Base64:       payload,
		OutputFormat: imageOutputFormat(contentType),
		ActualSize:   strconv.Itoa(config.Width) + "x" + strconv.Itoa(config.Height),
	}, nil
}

// resolveCodexImageResult keeps the provider's base64 payload unchanged when
// possible. Some compatible providers ignore response_format=b64_json and
// return an HTTPS URL instead; in that case only, download and validate the
// image through the same bounded SSRF-safe path used by the workbench, then
// encode it for Codex's image_generation_call result. Nothing is persisted to
// object storage on this path.
func resolveCodexImageResult(ctx context.Context, item CangyuanImageData, maxBytes int64) (*CodexImageResult, error) {
	return resolveCodexImageResultWithStore(ctx, item, maxBytes, nil)
}

func resolveCodexImageResultWithStore(ctx context.Context, item CangyuanImageData, maxBytes int64, resolver *CangyuanImageResultStore) (*CodexImageResult, error) {
	if strings.TrimSpace(item.B64JSON) != "" {
		return normalizeCodexImageBase64(item.B64JSON, maxBytes)
	}
	if strings.TrimSpace(item.URL) == "" {
		return nil, errors.New("completed Codex image result has no content")
	}
	if resolver == nil {
		resolver = NewCangyuanImageResultStore(nil, "", maxBytes)
	}
	asset, err := resolver.resolve(ctx, CangyuanImageData{URL: item.URL})
	if err != nil {
		return nil, err
	}
	return &CodexImageResult{
		Base64:       base64.StdEncoding.EncodeToString(asset.Data),
		OutputFormat: imageOutputFormat(asset.ContentType),
		ActualSize:   strconv.Itoa(asset.Width) + "x" + strconv.Itoa(asset.Height),
	}, nil
}

type countingReader struct {
	reader io.Reader
	read   int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += int64(n)
	return n, err
}

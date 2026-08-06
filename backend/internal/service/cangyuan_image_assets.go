package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const cangyuanAssetDownloadTimeout = 45 * time.Second

type CangyuanResolvedAsset struct {
	ContentType string
	Data        []byte
	Width       int
	Height      int
	SHA256      string
}

type cangyuanHostValidator func(context.Context, string) (bool, error)

type CangyuanImageAssetResolver struct {
	httpClient    *http.Client
	maxBytes      int64
	hostValidator cangyuanHostValidator
}

func NewCangyuanImageAssetResolver(maxBytes int64) *CangyuanImageAssetResolver {
	client := newSSRFSafeHTTPClient(cangyuanAssetDownloadTimeout)
	client.CheckRedirect = cangyuanImageRedirectPolicy(isPrivateOrLoopbackHost)
	return newCangyuanImageAssetResolver(client, maxBytes, isPrivateOrLoopbackHost)
}

func newCangyuanImageAssetResolver(client *http.Client, maxBytes int64, validator cangyuanHostValidator) *CangyuanImageAssetResolver {
	if maxBytes <= 0 || maxBytes > cangyuanMaxReferenceImageBytes {
		maxBytes = cangyuanMaxReferenceImageBytes
	}
	if client == nil {
		client = newSSRFSafeHTTPClient(cangyuanAssetDownloadTimeout)
	}
	if validator == nil {
		validator = isPrivateOrLoopbackHost
	}
	client.CheckRedirect = cangyuanImageRedirectPolicy(validator)
	return &CangyuanImageAssetResolver{httpClient: client, maxBytes: maxBytes, hostValidator: validator}
}

func cangyuanImageRedirectPolicy(validator cangyuanHostValidator) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many image download redirects")
		}
		if req.URL == nil || !strings.EqualFold(req.URL.Scheme, "https") {
			return errors.New("image redirect must use HTTPS")
		}
		blocked, err := validator(req.Context(), req.URL.Hostname())
		if err != nil {
			return errors.New("image redirect host could not be verified")
		}
		if blocked {
			return errors.New("image redirect blocked by SSRF policy")
		}
		return nil
	}
}

func (r *CangyuanImageAssetResolver) Resolve(ctx context.Context, rawValue string) (*CangyuanResolvedAsset, error) {
	rawValue = strings.TrimSpace(rawValue)
	if strings.HasPrefix(strings.ToLower(rawValue), "data:") {
		data, contentType, err := decodeCangyuanImageDataURL(rawValue)
		if err != nil {
			return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: err}
		}
		return inspectCangyuanResolvedAsset(data, contentType)
	}
	parsed, err := url.Parse(rawValue)
	if err != nil || parsed == nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.User != nil {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image must use an HTTPS URL without embedded credentials")}
	}
	if parsed.Fragment != "" {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image URL must not contain a fragment")}
	}
	blocked, err := r.hostValidator(ctx, parsed.Hostname())
	if err != nil {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image host could not be verified")}
	}
	if blocked {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image host is blocked by SSRF policy")}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image request is invalid")}
	}
	req.Header.Set("Accept", "image/*")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, &CangyuanAdapterError{Code: "image_reference_download_failed", Retryable: true, Err: errors.New("reference image download failed")}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &CangyuanAdapterError{Code: "image_reference_download_failed", HTTPStatus: resp.StatusCode, Retryable: resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500, Err: errors.New("reference image download returned an unsuccessful status")}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, r.maxBytes+1))
	if err != nil {
		return nil, &CangyuanAdapterError{Code: "image_reference_download_failed", Retryable: true, Err: errors.New("reference image body could not be read")}
	}
	if int64(len(data)) > r.maxBytes {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image exceeds 10 MB")}
	}
	declaredType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	return inspectCangyuanResolvedAsset(data, declaredType)
}

func (r *CangyuanImageAssetResolver) ResolveUnique(ctx context.Context, values []string) ([]CangyuanResolvedAsset, error) {
	if len(values) == 0 {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("at least one reference image is required")}
	}
	results := make([]CangyuanResolvedAsset, 0, min(len(values), cangyuanMaxReferenceImages))
	seen := make(map[string]struct{})
	for _, value := range values {
		asset, err := r.Resolve(ctx, value)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[asset.SHA256]; exists {
			continue
		}
		seen[asset.SHA256] = struct{}{}
		results = append(results, *asset)
		if len(results) > cangyuanMaxReferenceImages {
			return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("at most 9 unique reference images are allowed")}
		}
	}
	return results, nil
}

func (r *CangyuanImageAssetResolver) ResolveEditAssets(ctx context.Context, images []string, mask string) ([]CangyuanResolvedAsset, *CangyuanResolvedAsset, error) {
	resolved, err := r.ResolveUnique(ctx, images)
	if err != nil {
		return nil, nil, err
	}
	var resolvedMask *CangyuanResolvedAsset
	if strings.TrimSpace(mask) != "" {
		resolvedMask, err = r.Resolve(ctx, mask)
		if err != nil {
			return nil, nil, err
		}
	}
	validationImages := make([]CangyuanResolvedImageAsset, 0, len(resolved))
	for _, asset := range resolved {
		validationImages = append(validationImages, CangyuanResolvedImageAsset{ContentType: asset.ContentType, Data: asset.Data})
	}
	var validationMask *CangyuanResolvedImageAsset
	if resolvedMask != nil {
		validationMask = &CangyuanResolvedImageAsset{ContentType: resolvedMask.ContentType, Data: resolvedMask.Data}
	}
	if err := ValidateCangyuanResolvedEditAssets(validationImages, validationMask); err != nil {
		return nil, nil, err
	}
	return resolved, resolvedMask, nil
}

func inspectCangyuanResolvedAsset(data []byte, declaredType string) (*CangyuanResolvedAsset, error) {
	if len(data) == 0 || int64(len(data)) > cangyuanMaxReferenceImageBytes {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image is empty or exceeds 10 MB")}
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || validateCangyuanDecodedImageDimensions(config.Width, config.Height) != nil {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image is not decodable")}
	}
	detectedType := "image/" + format
	if format == "jpeg" {
		detectedType = "image/jpeg"
	}
	if strings.HasPrefix(declaredType, "image/") && declaredType != detectedType {
		return nil, &CangyuanAdapterError{Code: "image_invalid_reference", HTTPStatus: http.StatusBadRequest, Err: errors.New("reference image MIME type does not match its content")}
	}
	hash := sha256.Sum256(data)
	return &CangyuanResolvedAsset{
		ContentType: detectedType,
		Data:        data,
		Width:       config.Width,
		Height:      config.Height,
		SHA256:      hex.EncodeToString(hash[:]),
	}, nil
}

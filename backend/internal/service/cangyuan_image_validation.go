package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	CangyuanImageModel1K           = "gpt-image-2-1k"
	CangyuanImageModel2K           = "gpt-image-2-2k"
	CangyuanImageModel4K           = "gpt-image-2-4k"
	CangyuanImage25FlareModel1K    = "gpt-image-2.5-flare-1k"
	CangyuanImage25FlareModel2K    = "gpt-image-2.5-flare-2k"
	CangyuanImage25FlareModel4K    = "gpt-image-2.5-flare-4k"
	CangyuanImage25SunburstModel1K = "gpt-image-2.5-sunburst-1k"
	CangyuanImage25SunburstModel2K = "gpt-image-2.5-sunburst-2k"
	CangyuanImage25SunburstModel4K = "gpt-image-2.5-sunburst-4k"

	cangyuanMinPixels              int64 = 655360
	cangyuanMaxEdge                      = 3840
	cangyuanMaxReferenceImages           = 9
	cangyuanMaxReferenceImageBytes int64 = 10 << 20
	cangyuanMaxPromptRunes               = 12000
	cangyuanMaxDecodedPixels       int64 = 16 << 20
)

type CangyuanImageOperation string

type cangyuanImageModelCapabilities struct {
	Tier         string
	MaxPixels    int64
	Qualities    map[string]struct{}
	AspectRatios map[string]struct{}
}

var (
	cangyuanFixedImageQualities = stringSet("low", "medium", "high", "xhigh", "max")
	cangyuanFixedImageRatios    = stringSet("1:1", "16:9", "9:16", "3:2", "2:3", "4:3", "3:4", "21:9")
	cangyuanImageModels         = map[string]cangyuanImageModelCapabilities{
		CangyuanImageModel1K:           fixedCangyuanImageCapabilities("1K", 1048576),
		CangyuanImageModel2K:           fixedCangyuanImageCapabilities("2K", 4194304),
		CangyuanImageModel4K:           fixedCangyuanImageCapabilities("4K", 8294400),
		CangyuanImage25FlareModel1K:    fixedCangyuanImageCapabilities("1K", 1048576),
		CangyuanImage25FlareModel2K:    fixedCangyuanImageCapabilities("2K", 4194304),
		CangyuanImage25FlareModel4K:    fixedCangyuanImageCapabilities("4K", 8294400),
		CangyuanImage25SunburstModel1K: fixedCangyuanImageCapabilities("1K", 1048576),
		CangyuanImage25SunburstModel2K: fixedCangyuanImageCapabilities("2K", 4194304),
		CangyuanImage25SunburstModel4K: fixedCangyuanImageCapabilities("4K", 8294400),
	}
)

const (
	CangyuanImageOperationGeneration CangyuanImageOperation = "generation"
	CangyuanImageOperationEdit       CangyuanImageOperation = "edit"
)

type CangyuanImageRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Size   string `json:"size,omitempty"`
	// AspectRatio, ImageSize, and OutputResolution are compatibility-only
	// inputs accepted from older Sub2API callers. Cangyuan's documented Images
	// API exposes geometry through size; canonicalization folds the ratio into
	// Size and strips all three legacy fields before persistence or transport.
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

func ValidateCangyuanImageRequest(operation CangyuanImageOperation, request CangyuanImageRequest) error {
	var err error
	request, err = NormalizeCangyuanImageRequest(request)
	if err != nil {
		return err
	}
	capabilities, ok := cangyuanImageCapabilities(request.Model)
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
		return &CangyuanAdapterError{Code: "image_invalid_count", HTTPStatus: http.StatusBadRequest, Err: errors.New("cangyuan image tier models require n=1")}
	}
	if request.ResponseFormat != "" && request.ResponseFormat != "url" && request.ResponseFormat != "b64_json" {
		return &CangyuanAdapterError{Code: "image_invalid_response_format", HTTPStatus: http.StatusBadRequest, Err: errors.New("response_format must be url or b64_json")}
	}
	quality := strings.ToLower(strings.TrimSpace(request.Quality))
	if quality != "" {
		if _, allowed := capabilities.Qualities[quality]; !allowed {
			return &CangyuanAdapterError{Code: "image_invalid_quality", HTTPStatus: http.StatusBadRequest, Err: errors.New("quality must be low, medium, high, xhigh, or max")}
		}
	}
	if request.Size != "" {
		if strings.Contains(request.Size, ":") {
			if err := validateCangyuanAspectRatio(request.Size, capabilities); err != nil {
				return err
			}
		} else {
			width, height, err := parseCangyuanImageSize(request.Size)
			if err != nil {
				return err
			}
			pixels := int64(width) * int64(height)
			if width%16 != 0 || height%16 != 0 || width > cangyuanMaxEdge || height > cangyuanMaxEdge || max(width, height) > 3*min(width, height) || pixels < cangyuanMinPixels || pixels > capabilities.MaxPixels {
				return &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: errors.New("size violates the selected Cangyuan model limits")}
			}
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

// NormalizeCangyuanImageRequest produces the only representation that may be
// persisted or sent upstream. It keeps old clients working while preventing
// undocumented aspect_ratio/image_size/output_resolution fields from leaking
// into Cangyuan requests. Dedicated jobs always use the provider's async task
// contract; synchronous public responses are implemented by waiting locally.
func NormalizeCangyuanImageRequest(request CangyuanImageRequest) (CangyuanImageRequest, error) {
	tier, _, ok := cangyuanImageTier(request.Model)
	if !ok {
		return request, &CangyuanAdapterError{Code: "image_model_not_allowed", HTTPStatus: http.StatusBadRequest, Err: errors.New("unsupported Cangyuan image model")}
	}
	request.Model = strings.TrimSpace(request.Model)
	request.Size = strings.TrimSpace(request.Size)
	request.AspectRatio = strings.TrimSpace(request.AspectRatio)
	if request.Size != "" && request.AspectRatio != "" {
		return request, &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: errors.New("size and aspect_ratio cannot both be set")}
	}
	for field, value := range map[string]string{
		"image_size":        request.ImageSize,
		"output_resolution": request.OutputResolution,
	} {
		if value != "" && !strings.EqualFold(strings.TrimSpace(value), tier) {
			return request, &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: fmt.Errorf("%s conflicts with the selected model tier", field)}
		}
	}
	if request.Size == "" {
		request.Size = request.AspectRatio
	}
	request.AspectRatio = ""
	request.ImageSize = ""
	request.OutputResolution = ""
	request.Quality = strings.ToLower(strings.TrimSpace(request.Quality))
	if request.Quality == "auto" {
		request.Quality = ""
	}
	request.Async = true
	return request, nil
}

func validateCangyuanAspectRatio(value string, capabilities cangyuanImageModelCapabilities) error {
	if _, ok := capabilities.AspectRatios[strings.TrimSpace(value)]; ok {
		return nil
	}
	return &CangyuanAdapterError{Code: "image_invalid_size", HTTPStatus: http.StatusBadRequest, Err: errors.New("size ratio is not supported by the selected Cangyuan model")}
}

func cangyuanImageTier(model string) (string, int64, bool) {
	capabilities, ok := cangyuanImageCapabilities(model)
	if !ok {
		return "", 0, false
	}
	return capabilities.Tier, capabilities.MaxPixels, true
}

func cangyuanImageCapabilities(model string) (cangyuanImageModelCapabilities, bool) {
	capabilities, ok := cangyuanImageModels[strings.TrimSpace(model)]
	return capabilities, ok
}

func fixedCangyuanImageCapabilities(tier string, maxPixels int64) cangyuanImageModelCapabilities {
	return cangyuanImageModelCapabilities{
		Tier:         tier,
		MaxPixels:    maxPixels,
		Qualities:    cangyuanFixedImageQualities,
		AspectRatios: cangyuanFixedImageRatios,
	}
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func IsCangyuanImageModel(model string) bool {
	_, _, ok := cangyuanImageTier(model)
	return ok
}

func CangyuanImageModelTier(model string) (string, bool) {
	tier, _, ok := cangyuanImageTier(model)
	return tier, ok
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

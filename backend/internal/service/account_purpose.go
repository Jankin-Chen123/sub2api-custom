package service

import (
	"maps"
	"net/url"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	AccountPurposeExtraKey = "account_purpose"

	AccountPurposeGeneral   = "general"
	AccountPurposeImageOnly = "image_only"
)

var cangyuanImageModels = map[string]struct{}{
	"gpt-image-2-1k": {},
	"gpt-image-2-2k": {},
	"gpt-image-2-4k": {},
}

// AccountPurpose returns the persisted account purpose. Missing, null, empty,
// and legacy values are treated as general so pre-feature accounts keep their
// existing behavior. Writes are validated separately and reject unknown values.
func (a *Account) AccountPurpose() string {
	if a == nil || a.Extra == nil {
		return AccountPurposeGeneral
	}
	value, _ := a.Extra[AccountPurposeExtraKey].(string)
	if strings.TrimSpace(value) == AccountPurposeImageOnly {
		return AccountPurposeImageOnly
	}
	return AccountPurposeGeneral
}

func (a *Account) IsImageOnly() bool {
	return a.AccountPurpose() == AccountPurposeImageOnly
}

// SupportsCangyuanImageFallback reports whether a general OpenAI API-key
// account has explicitly opted into being used by the Cangyuan adapter. A
// normal OpenAI Images-compatible account is not sufficient: the fallback
// adapter sends Cangyuan's generation/edit protocol and model names.
func (a *Account) SupportsCangyuanImageFallback() bool {
	if a == nil || a.IsImageOnly() || !a.IsOpenAIApiKey() || a.Credentials == nil {
		return false
	}
	baseURL, _ := a.Credentials["base_url"].(string)
	if !isValidCangyuanBaseURL(baseURL) {
		return false
	}
	apiKey, _ := a.Credentials["api_key"].(string)
	if strings.TrimSpace(apiKey) == "" {
		return false
	}
	for _, upstreamModel := range a.GetModelMapping() {
		if _, supported := cangyuanImageModels[strings.TrimSpace(upstreamModel)]; supported {
			return true
		}
	}
	return false
}

// NormalizeAccountPurposeExtra validates the administrator-controlled account
// purpose and returns a cloned map. image_only is intentionally narrow in the
// first release: it is an OpenAI API-key account configured for Cangyuan's
// dedicated GPT Image 2 tiers.
func NormalizeAccountPurposeExtra(platform, accountType string, credentials, extra map[string]any) (map[string]any, error) {
	normalized := maps.Clone(extra)
	if normalized == nil {
		normalized = make(map[string]any)
	}

	raw, provided := normalized[AccountPurposeExtraKey]
	if !provided || raw == nil {
		delete(normalized, AccountPurposeExtraKey)
		return nilIfEmptyMap(normalized), nil
	}

	purpose, ok := raw.(string)
	if !ok {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_PURPOSE", "account_purpose must be general or image_only")
	}
	purpose = strings.TrimSpace(purpose)
	switch purpose {
	case "", AccountPurposeGeneral:
		// General is the compatibility default. Keeping the map sparse prevents
		// mass rewrites of legacy accounts while still accepting an explicit value.
		delete(normalized, AccountPurposeExtraKey)
		return nilIfEmptyMap(normalized), nil
	case AccountPurposeImageOnly:
		if platform != PlatformOpenAI || accountType != AccountTypeAPIKey {
			return nil, infraerrors.BadRequest(
				"IMAGE_ONLY_ACCOUNT_TYPE_INVALID",
				"image_only accounts must use the OpenAI platform and API-key authentication",
			)
		}
		if err := validateCangyuanImageOnlyCredentials(credentials); err != nil {
			return nil, err
		}
		normalized[AccountPurposeExtraKey] = AccountPurposeImageOnly
		return normalized, nil
	default:
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_PURPOSE", "account_purpose must be general or image_only")
	}
}

func validateCangyuanImageOnlyCredentials(credentials map[string]any) error {
	baseURL, _ := credentials["base_url"].(string)
	if !isValidCangyuanBaseURL(baseURL) {
		return infraerrors.BadRequest(
			"IMAGE_ONLY_BASE_URL_INVALID",
			"image_only accounts require a valid HTTPS base_url without credentials, query, or fragment",
		)
	}

	apiKey, _ := credentials["api_key"].(string)
	if strings.TrimSpace(apiKey) == "" {
		return infraerrors.BadRequest(
			"IMAGE_ONLY_API_KEY_REQUIRED",
			"image_only accounts require an API key",
		)
	}

	rawMapping, ok := credentials["model_mapping"]
	if !ok {
		return infraerrors.BadRequest(
			"IMAGE_ONLY_MODEL_MAPPING_REQUIRED",
			"image_only accounts require a non-empty model_mapping",
		)
	}
	mapping := normalizedStringMap(rawMapping)
	if len(mapping) == 0 {
		return infraerrors.BadRequest(
			"IMAGE_ONLY_MODEL_MAPPING_REQUIRED",
			"image_only accounts require a non-empty model_mapping",
		)
	}
	for _, upstreamModel := range mapping {
		upstreamModel = strings.TrimSpace(upstreamModel)
		if _, supported := cangyuanImageModels[upstreamModel]; !supported {
			return infraerrors.BadRequest(
				"IMAGE_ONLY_MODEL_MAPPING_INVALID",
				"image_only model_mapping targets must be gpt-image-2-1k, gpt-image-2-2k, or gpt-image-2-4k",
			)
		}
	}
	return nil
}

// isValidCangyuanBaseURL keeps administrator configuration failures at the
// write boundary. The adapter applies the same restrictions when it builds
// request URLs, but accepting a URL here and failing only after a paid job is
// queued would leave an image-only account apparently healthy in the UI.
func isValidCangyuanBaseURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed != nil &&
		strings.EqualFold(parsed.Scheme, "https") &&
		strings.TrimSpace(parsed.Host) != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func normalizedStringMap(raw any) map[string]string {
	result := make(map[string]string)
	switch values := raw.(type) {
	case map[string]string:
		for key, value := range values {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key != "" && value != "" {
				result[key] = value
			}
		}
	case map[string]any:
		for key, rawValue := range values {
			value, ok := rawValue.(string)
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if ok && key != "" && value != "" {
				result[key] = value
			}
		}
	}
	return result
}

func nilIfEmptyMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	return value
}

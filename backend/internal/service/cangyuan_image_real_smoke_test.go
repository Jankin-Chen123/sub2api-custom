package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRealCangyuanGenerationSmoke is an opt-in, billable provider check. It
// is skipped during normal CI and never contains a credential in source or
// test output. The key may be supplied through CANGYUAN_API_KEY or a local
// file named by CANGYUAN_API_KEY_FILE.
func TestRealCangyuanGenerationSmoke(t *testing.T) {
	if strings.TrimSpace(os.Getenv("CANGYUAN_REAL_SMOKE")) != "1" {
		t.Skip("set CANGYUAN_REAL_SMOKE=1 to run a billable Cangyuan smoke test")
	}

	apiKey := strings.TrimSpace(os.Getenv("CANGYUAN_API_KEY"))
	if apiKey == "" {
		keyFile := strings.TrimSpace(os.Getenv("CANGYUAN_API_KEY_FILE"))
		if keyFile == "" {
			t.Fatal("CANGYUAN_API_KEY or CANGYUAN_API_KEY_FILE is required")
		}
		contents, err := os.ReadFile(keyFile)
		require.NoError(t, err)
		apiKey = strings.TrimSpace(string(contents))
	}
	require.NotEmpty(t, apiKey)

	baseURL := strings.TrimSpace(os.Getenv("CANGYUAN_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://ai.cangyuansuanli.cn/v1"
	}
	model := strings.TrimSpace(os.Getenv("CANGYUAN_REAL_MODEL"))
	if model == "" {
		model = CangyuanImageModel1K
	}
	tier, _, ok := cangyuanImageTier(model)
	require.Truef(t, ok, "unsupported Cangyuan model %q", model)
	size := strings.TrimSpace(os.Getenv("CANGYUAN_REAL_SIZE"))
	if size == "" {
		switch model {
		case CangyuanImageModel1K:
			size = "1024x1024"
		case CangyuanImageModel2K:
			size = "2048x2048"
		case CangyuanImageModel4K:
			size = "3840x2160"
		}
	}

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly},
		Credentials: map[string]any{
			"api_key":  apiKey,
			"base_url": baseURL,
			"model_mapping": map[string]any{
				CangyuanImageModel1K: CangyuanImageModel1K,
				CangyuanImageModel2K: CangyuanImageModel2K,
				CangyuanImageModel4K: CangyuanImageModel4K,
			},
		},
	}
	adapter, err := NewCangyuanImageAdapterFromAccount(account, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	asyncMode := strings.TrimSpace(os.Getenv("CANGYUAN_REAL_ASYNC")) == "1"
	result, err := adapter.SubmitGeneration(ctx, CangyuanImageRequest{
		Model:            model,
		Prompt:           "A simple friendly blue circle on a clean white background",
		Size:             size,
		N:                1,
		ResponseFormat:   "url",
		Async:            asyncMode,
		ImageSize:        tier,
		OutputResolution: tier,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Failed)
	if asyncMode {
		require.NotEmpty(t, result.UpstreamTaskID, "Cangyuan async response did not include a task ID")
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for !result.Completed && !result.Failed {
			select {
			case <-ctx.Done():
				t.Fatal("Cangyuan async smoke timed out while polling")
			case <-ticker.C:
				result, err = adapter.PollGeneration(ctx, result.UpstreamTaskID)
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		}
	}
	require.False(t, result.Failed)
	require.NotEmpty(t, result.Data, "Cangyuan returned no image data")

	// Keep the log safe for CI output: no URL, base64 payload, provider task
	// ID, account binding, or raw response is printed.
	t.Logf("Cangyuan smoke succeeded: model=%s async=%t status=%s completed=%t data_count=%d", model, asyncMode, result.Status, result.Completed, len(result.Data))
}

func TestRealCangyuanBase64GenerationSmoke(t *testing.T) {
	if strings.TrimSpace(os.Getenv("CANGYUAN_REAL_B64_SMOKE")) != "1" {
		t.Skip("set CANGYUAN_REAL_B64_SMOKE=1 to run a billable Cangyuan base64 smoke test")
	}
	apiKey, baseURL := realCangyuanSmokeCredentials(t)
	adapter, err := NewCangyuanImageAdapter(baseURL, apiKey, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	result, err := adapter.SubmitGeneration(ctx, CangyuanImageRequest{
		Model: CangyuanImageModel1K, Prompt: "A simple friendly blue circle on a clean white background",
		Size: "1024x1024", N: 1, ResponseFormat: "b64_json", ImageSize: "1K", OutputResolution: "1K",
	})
	requireRealCangyuanSmokeResult(t, result, err)
	require.NotNil(t, result)
	require.False(t, result.Failed)
	require.NotEmpty(t, result.Data)
	require.NotEmpty(t, result.Data[0].B64JSON)
	t.Logf("Cangyuan base64 smoke succeeded: status=%s completed=%t data_count=%d", result.Status, result.Completed, len(result.Data))
}

// TestRealCangyuanJSONEditSmoke is an opt-in, billable provider check for the
// JSON image-to-image path. The fixture uses a valid 1024x1024 source and an
// alpha PNG mask; the earlier 1x1 fixture was intentionally too weak to prove
// the provider's normal edit behavior.
func TestRealCangyuanJSONEditSmoke(t *testing.T) {
	if strings.TrimSpace(os.Getenv("CANGYUAN_REAL_EDIT_SMOKE")) != "1" {
		t.Skip("set CANGYUAN_REAL_EDIT_SMOKE=1 to run a billable Cangyuan JSON edit smoke test")
	}
	apiKey, baseURL := realCangyuanSmokeCredentials(t)
	adapter, err := NewCangyuanImageAdapter(baseURL, apiKey, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	mask := ""
	if strings.TrimSpace(os.Getenv("CANGYUAN_REAL_EDIT_WITH_MASK")) == "1" {
		mask = realCangyuanSmokePNGDataURL(true)
	}
	result, err := adapter.SubmitEdit(ctx, CangyuanImageRequest{
		Model: CangyuanImageModel1K, Prompt: "Change the background to a calm sunset while preserving the subject",
		N: 1, ResponseFormat: "url", ImageSize: "1K", OutputResolution: "1K",
		Images: []string{realCangyuanSmokePNGDataURL(false)},
		Mask:   mask,
	})
	requireRealCangyuanSmokeResult(t, result, err)
	require.NotNil(t, result)
	require.False(t, result.Failed)
	require.NotEmpty(t, result.Data)
	t.Logf("Cangyuan JSON edit smoke succeeded: status=%s completed=%t data_count=%d", result.Status, result.Completed, len(result.Data))
}

// TestRealCangyuanMultipartEditSmoke is an opt-in, billable provider check
// for repeated multipart image fields and a multipart mask.
func TestRealCangyuanMultipartEditSmoke(t *testing.T) {
	if strings.TrimSpace(os.Getenv("CANGYUAN_REAL_MULTIPART_SMOKE")) != "1" {
		t.Skip("set CANGYUAN_REAL_MULTIPART_SMOKE=1 to run a billable Cangyuan multipart edit smoke test")
	}
	apiKey, baseURL := realCangyuanSmokeCredentials(t)
	adapter, err := NewCangyuanImageAdapter(baseURL, apiKey, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	mask := ""
	if strings.TrimSpace(os.Getenv("CANGYUAN_REAL_MULTIPART_WITH_MASK")) == "1" {
		mask = realCangyuanSmokePNGDataURL(true)
	}
	result, err := adapter.SubmitEdit(ctx, CangyuanImageRequest{
		Model: CangyuanImageModel1K, Prompt: "Make the background a calm sunset while preserving the subject",
		N: 1, ResponseFormat: "url", ImageSize: "1K", OutputResolution: "1K", Multipart: true,
		Images: []string{realCangyuanSmokePNGDataURL(false), realCangyuanSmokePNGDataURL(false)},
		Mask:   mask,
	})
	requireRealCangyuanSmokeResult(t, result, err)
	require.NotNil(t, result)
	require.False(t, result.Failed)
	require.NotEmpty(t, result.Data)
	t.Logf("Cangyuan multipart edit smoke succeeded: status=%s completed=%t data_count=%d", result.Status, result.Completed, len(result.Data))
}

func realCangyuanSmokeCredentials(t *testing.T) (string, string) {
	t.Helper()
	apiKey := strings.TrimSpace(os.Getenv("CANGYUAN_API_KEY"))
	if apiKey == "" {
		keyFile := strings.TrimSpace(os.Getenv("CANGYUAN_API_KEY_FILE"))
		require.NotEmpty(t, keyFile, "CANGYUAN_API_KEY or CANGYUAN_API_KEY_FILE is required")
		contents, err := os.ReadFile(keyFile)
		require.NoError(t, err)
		apiKey = strings.TrimSpace(string(contents))
	}
	require.NotEmpty(t, apiKey)
	baseURL := strings.TrimSpace(os.Getenv("CANGYUAN_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://ai.cangyuansuanli.cn/v1"
	}
	return apiKey, baseURL
}

func requireRealCangyuanSmokeResult(t *testing.T, result *CangyuanImageResult, err error) {
	t.Helper()
	if err == nil {
		return
	}
	var adapterErr *CangyuanAdapterError
	if errors.As(err, &adapterErr) && adapterErr != nil {
		t.Fatalf("Cangyuan smoke failed: code=%s http_status=%d retryable=%t submission_unknown=%t", adapterErr.Code, adapterErr.HTTPStatus, adapterErr.Retryable, adapterErr.SubmissionUnknown)
	}
	t.Fatalf("Cangyuan smoke failed: error_type=%T", err)
}

func realCangyuanSmokePNGDataURL(mask bool) string {
	const side = 1024
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	fill := color.NRGBA{R: 40, G: 120, B: 220, A: 255}
	if mask {
		fill = color.NRGBA{R: 255, G: 255, B: 255, A: 128}
	}
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var encoded bytes.Buffer
	_ = png.Encode(&encoded, img)
	return imageAssetDataURL("image/png", encoded.Bytes())
}

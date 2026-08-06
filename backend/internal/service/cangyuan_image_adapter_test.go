package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildCangyuanImageEndpointAvoidsDuplicateV1(t *testing.T) {
	tests := map[string]string{
		"https://ai.cangyuansuanli.cn":    "https://ai.cangyuansuanli.cn/v1/images/generations",
		"https://ai.cangyuansuanli.cn/":   "https://ai.cangyuansuanli.cn/v1/images/generations",
		"https://ai.cangyuansuanli.cn/v1": "https://ai.cangyuansuanli.cn/v1/images/generations",
	}
	for baseURL, expected := range tests {
		actual, err := buildCangyuanImageEndpoint(baseURL, "/v1/images/generations")
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
}

func TestValidateCangyuanImageRequestSizes(t *testing.T) {
	valid := []CangyuanImageRequest{
		{Model: CangyuanImageModel1K, Prompt: "test", Size: "1024x1024", N: 1, OutputResolution: "1K"},
		{Model: CangyuanImageModel2K, Prompt: "test", Size: "2048x2048", N: 1, ImageSize: "2k"},
		{Model: CangyuanImageModel4K, Prompt: "test", Size: "3840x2160", N: 1, OutputResolution: "4K"},
		{Model: CangyuanImageModel4K, Prompt: "test", Size: "16:9", N: 1},
		{Model: CangyuanImageModel2K, Prompt: "test", AspectRatio: "7:6", N: 1},
	}
	for _, request := range valid {
		require.NoError(t, ValidateCangyuanImageRequest(CangyuanImageOperationGeneration, request))
	}

	invalid := []CangyuanImageRequest{
		{Model: CangyuanImageModel1K, Prompt: "test", Size: "1030x1024", N: 1},
		{Model: CangyuanImageModel1K, Prompt: "test", Size: "2048x2048", N: 1},
		{Model: CangyuanImageModel4K, Prompt: "test", Size: "4096x4096", N: 1},
		{Model: CangyuanImageModel4K, Prompt: "test", Size: "3840x1024", N: 1},
		{Model: CangyuanImageModel2K, Prompt: "test", Size: "2048x2048", N: 2},
		{Model: CangyuanImageModel2K, Prompt: "test", Size: "2048x2048", N: 1, OutputResolution: "4K"},
		{Model: CangyuanImageModel4K, Prompt: "test", AspectRatio: "4:1", N: 1},
		{Model: CangyuanImageModel4K, Prompt: "test", Size: "16:9", AspectRatio: "16:9", N: 1},
		{Model: CangyuanImageModel2K, Prompt: "test", Size: "2048x2048", Quality: "ultra", N: 1},
	}
	for _, request := range invalid {
		err := ValidateCangyuanImageRequest(CangyuanImageOperationGeneration, request)
		require.Error(t, err)
	}
}

func TestValidateCangyuanImageRequestRejectsOversizedPrompt(t *testing.T) {
	request := CangyuanImageRequest{
		Model:  CangyuanImageModel1K,
		Prompt: strings.Repeat("你", cangyuanMaxPromptRunes+1),
		Size:   "1024x1024",
		N:      1,
	}

	err := ValidateCangyuanImageRequest(CangyuanImageOperationGeneration, request)
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_prompt_too_long", adapterErr.Code)
	require.Equal(t, http.StatusBadRequest, adapterErr.HTTPStatus)
}

func TestValidateCangyuanDecodedImageDimensionsBoundsDecodeBombs(t *testing.T) {
	require.NoError(t, validateCangyuanDecodedImageDimensions(3840, 2160))
	require.Error(t, validateCangyuanDecodedImageDimensions(16384, 1))
	require.Error(t, validateCangyuanDecodedImageDimensions(8192, 4096))
}

func TestCangyuanImageAdapterDeduplicatesReferenceAliasesBeforeSubmit(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request CangyuanImageRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, []string{"https://example.com/a.png", "https://example.com/b.png"}, request.Images)
		require.Equal(t, "16:9", request.AspectRatio)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://images.example/result.png"}]}`))
	}))
	defer server.Close()

	adapter, err := NewCangyuanImageAdapter(server.URL, "test-secret", server.Client())
	require.NoError(t, err)
	_, err = adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel4K, Prompt: "poster", AspectRatio: "16:9", N: 1,
		Images: []string{"https://example.com/a.png", "https://example.com/a.png", "https://example.com/b.png"},
	})
	require.NoError(t, err)
}

func TestCangyuanImageAdapterSubmitGenerationSync(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var request CangyuanImageRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, CangyuanImageModel1K, request.Model)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1785800000,"data":[{"url":"https://images.example/result.png"}]}`))
	}))
	defer server.Close()

	adapter, err := NewCangyuanImageAdapter(server.URL+"/v1", "test-secret", server.Client())
	require.NoError(t, err)
	result, err := adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel1K, Prompt: "test", Size: "1024x1024", N: 1, ResponseFormat: "url",
	})
	require.NoError(t, err)
	require.True(t, result.Completed)
	require.Len(t, result.Data, 1)
	require.NotEmpty(t, result.Data[0].URL)
}

func TestCangyuanImageAdapterEditMultipartUsesRepeatedImageFiles(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/edits", r.URL.Path)
		require.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
		require.NoError(t, r.ParseMultipartForm(2<<20))
		require.Equal(t, []string{"gpt-image-2-2k"}, r.MultipartForm.Value["model"])
		require.Len(t, r.MultipartForm.File["image"], 2)
		require.Equal(t, "image-1.png", r.MultipartForm.File["image"][0].Filename)
		require.Equal(t, "image-2.png", r.MultipartForm.File["image"][1].Filename)
		require.Len(t, r.MultipartForm.File["mask"], 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"ZmFrZQ=="}]}`))
	}))
	defer server.Close()

	adapter, err := NewCangyuanImageAdapter(server.URL, "test-secret", server.Client())
	require.NoError(t, err)
	input := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	secondImage := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	secondImage.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	secondInput := encodePNG(t, secondImage)
	mask := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	result, err := adapter.SubmitEdit(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel2K, Prompt: "edit", Size: "2048x2048", N: 1,
		ResponseFormat: "b64_json", Multipart: true,
		Images: []string{imageAssetDataURL("image/png", input), imageAssetDataURL("image/png", secondInput)},
		Mask:   imageAssetDataURL("image/png", mask),
	})
	require.NoError(t, err)
	require.True(t, result.Completed)
}

func TestCangyuanImageAdapterAsyncSubmissionAndStickyPollPath(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/images/generations":
			_, _ = w.Write([]byte(`{"task_id":"private/task","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/v1/images/generations/private%2Ftask":
			_, _ = w.Write([]byte(`{"task_id":"private/task","status":"completed","data":[{"b64_json":"ZmFrZQ=="}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter, err := NewCangyuanImageAdapter(server.URL, "test-secret", server.Client())
	require.NoError(t, err)
	submitted, err := adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel1K, Prompt: "test", Size: "1024x1024", N: 1, Async: true,
	})
	require.NoError(t, err)
	require.False(t, submitted.Completed)
	require.Equal(t, "private/task", submitted.UpstreamTaskID)

	completed, err := adapter.PollGeneration(context.Background(), submitted.UpstreamTaskID)
	require.NoError(t, err)
	require.True(t, completed.Completed)
	require.NotEmpty(t, completed.Data[0].B64JSON)
}

func TestCangyuanImageAdapterNormalizesErrorsWithoutBodyLeak(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"new_api_error","code":"","message":"secret upstream detail"}}`))
	}))
	defer server.Close()

	adapter, err := NewCangyuanImageAdapter(server.URL, "test-secret", server.Client())
	require.NoError(t, err)
	_, err = adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel1K, Prompt: "test", Size: "1024x1024", N: 1,
	})
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_upstream_auth_failed", adapterErr.Code)
	require.NotContains(t, err.Error(), "secret upstream detail")
}

func TestCangyuanImageAdapterRejectsAmbiguousSuccessfulResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"created":1785800000}`))
	}))
	defer server.Close()
	adapter, err := NewCangyuanImageAdapter(server.URL, "test-secret", server.Client())
	require.NoError(t, err)
	_, err = adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel1K, Prompt: "test", Size: "1024x1024", N: 1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "image_upstream_invalid_response")
}

func TestCangyuanImageAdapterRejectsMultipleResultsForTierRequest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://images.example/one.png"},{"url":"https://images.example/two.png"}]}`))
	}))
	defer server.Close()
	adapter, err := NewCangyuanImageAdapter(server.URL, "test-secret", server.Client())
	require.NoError(t, err)
	_, err = adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel1K, Prompt: "one image", Size: "1024x1024", N: 1,
	})
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_upstream_invalid_response", adapterErr.Code)
}

func TestCangyuanImageAdapterContractErrors(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		wantCode      string
		wantRetryable bool
		wantUnknown   bool
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"error":{"type":"rate_limit_exceeded","message":"provider detail"}}`, wantCode: "image_upstream_rate_limited", wantRetryable: true, wantUnknown: false},
		{name: "new api error without provider code", status: http.StatusBadRequest, body: `{"error":{"type":"new_api_error","message":"provider detail"}}`, wantCode: "image_upstream_rejected", wantRetryable: false, wantUnknown: false},
		{name: "bad gateway", status: http.StatusBadGateway, body: `{"error":{"type":"upstream_error","message":"provider detail"}}`, wantCode: "image_upstream_unavailable", wantRetryable: true, wantUnknown: true},
		{name: "server error", status: http.StatusInternalServerError, body: `{"error":{"type":"internal_error","message":"provider detail"}}`, wantCode: "image_upstream_unavailable", wantRetryable: true, wantUnknown: true},
		{name: "invalid JSON", status: http.StatusOK, body: `{not-json`, wantCode: "image_upstream_invalid_response", wantRetryable: false, wantUnknown: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			adapter, err := NewCangyuanImageAdapter(server.URL, "test-secret", server.Client())
			require.NoError(t, err)
			_, err = adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
				Model: CangyuanImageModel1K, Prompt: "test", Size: "1024x1024", N: 1,
			})
			var adapterErr *CangyuanAdapterError
			require.ErrorAs(t, err, &adapterErr)
			require.Equal(t, test.wantCode, adapterErr.Code)
			require.Equal(t, test.wantRetryable, adapterErr.Retryable)
			require.Equal(t, test.wantUnknown, adapterErr.SubmissionUnknown)
			require.NotContains(t, err.Error(), "provider detail")
		})
	}
}

func TestCangyuanImageAdapterTimeoutIsSubmissionUnknown(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	adapter, err := NewCangyuanImageAdapter("https://cangyuan.example", "test-secret", client)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = adapter.SubmitGeneration(ctx, CangyuanImageRequest{
		Model: CangyuanImageModel1K, Prompt: "test", Size: "1024x1024", N: 1,
	})
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_upstream_timeout", adapterErr.Code)
	require.True(t, adapterErr.SubmissionUnknown)
	require.False(t, errors.Is(err, context.Canceled), "adapter must expose a stable code even when the request context is canceled")
}

func TestValidateCangyuanResolvedEditAssets(t *testing.T) {
	input := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	maskImage := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	maskImage.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	mask := encodePNG(t, maskImage)
	require.NoError(t, ValidateCangyuanResolvedEditAssets(
		[]CangyuanResolvedImageAsset{{ContentType: "image/png", Data: input}},
		&CangyuanResolvedImageAsset{ContentType: "image/png", Data: mask},
	))

	wrongSize := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 16, 16)))
	err := ValidateCangyuanResolvedEditAssets(
		[]CangyuanResolvedImageAsset{{ContentType: "image/png", Data: input}},
		&CangyuanResolvedImageAsset{ContentType: "image/png", Data: wrongSize},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "image_invalid_mask")
}

func TestValidateCangyuanEditReferenceCountAfterDedup(t *testing.T) {
	request := CangyuanImageRequest{
		Model:  CangyuanImageModel1K,
		Prompt: "edit",
		Size:   "1024x1024",
		N:      1,
		Images: []string{"https://example.com/a.png", "https://example.com/a.png"},
	}
	require.NoError(t, ValidateCangyuanImageRequest(CangyuanImageOperationEdit, request))

	request.Images = nil
	for index := 0; index < 10; index++ {
		request.Images = append(request.Images, "https://example.com/"+strings.Repeat("x", index+1)+".png")
	}
	require.Error(t, ValidateCangyuanImageRequest(CangyuanImageOperationEdit, request))
}

func TestValidateCangyuanImageRequestChecksDataURLAndMask(t *testing.T) {
	input := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	maskImage := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	maskImage.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	mask := encodePNG(t, maskImage)
	request := CangyuanImageRequest{
		Model:  CangyuanImageModel1K,
		Prompt: "edit",
		Size:   "1024x1024",
		N:      1,
		Images: []string{"data:image/png;base64," + base64.StdEncoding.EncodeToString(input)},
		Mask:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(mask),
	}
	require.NoError(t, ValidateCangyuanImageRequest(CangyuanImageOperationEdit, request))

	request.Mask = "data:image/png;base64,not-base64"
	err := ValidateCangyuanImageRequest(CangyuanImageOperationEdit, request)
	require.Error(t, err)
	require.Contains(t, err.Error(), "image_invalid_mask")
}

func TestValidateCangyuanImageRequestRejectsDataURLMIMEConfusion(t *testing.T) {
	input := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	request := CangyuanImageRequest{
		Model:  CangyuanImageModel1K,
		Prompt: "edit",
		Size:   "1024x1024",
		N:      1,
		Images: []string{"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(input)},
	}
	err := ValidateCangyuanImageRequest(CangyuanImageOperationEdit, request)
	require.Error(t, err)
	require.Contains(t, err.Error(), "image_invalid_reference")
}

func encodePNG(t *testing.T, value image.Image) []byte {
	t.Helper()
	var output bytes.Buffer
	require.NoError(t, png.Encode(&output, value))
	return output.Bytes()
}

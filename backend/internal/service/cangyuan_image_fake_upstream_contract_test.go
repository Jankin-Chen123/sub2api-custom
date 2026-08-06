package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeCangyuanUpstream is deliberately small, but it speaks the same
// generation/edit + asynchronous polling contract as the adapter. Keeping it
// in-process makes the contract test independent of a live provider and lets
// it assert the exact path, authorization, JSON, and multipart shapes.
type fakeCangyuanUpstream struct {
	server     *httptest.Server
	mu         sync.Mutex
	pollCounts map[string]int
	calls      []fakeCangyuanCall
	output     []byte
	outputPath string
}

type fakeCangyuanCall struct {
	method      string
	path        string
	contentType string
	model       string
	prompt      string
	size        string
	imageCount  int
	hasMask     bool
	async       bool
}

func newFakeCangyuanUpstream(t *testing.T) *fakeCangyuanUpstream {
	t.Helper()
	fake := &fakeCangyuanUpstream{
		pollCounts: make(map[string]int),
		output:     encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 16))),
		outputPath: "/fake-output.png",
	}
	fake.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Provider result URLs are downloaded as ordinary HTTPS resources;
		// they may be signed URLs and must not require forwarding the provider
		// API key. Keep this separate from the API endpoint auth assertion.
		if r.Method == http.MethodGet && r.URL.Path == fake.outputPath {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(fake.output)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fake-cangyuan-key" {
			writeFakeCangyuanJSON(w, http.StatusUnauthorized, `{"error":{"type":"authentication_error"}}`)
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/images/generations":
			var request CangyuanImageRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				writeFakeCangyuanJSON(w, http.StatusBadRequest, `{"error":{"type":"invalid_request_error"}}`)
				return
			}
			fake.mu.Lock()
			fake.calls = append(fake.calls, fakeCangyuanCall{
				method: r.Method, path: r.URL.Path, contentType: r.Header.Get("Content-Type"),
				model: request.Model, prompt: request.Prompt, size: request.Size,
				imageCount: len(request.Images), hasMask: strings.TrimSpace(request.Mask) != "", async: request.Async,
			})
			fake.mu.Unlock()
			writeFakeCangyuanJSON(w, http.StatusOK, `{"task_id":"generation/task-1","status":"queued"}`)
			return

		case r.Method == http.MethodPost && r.URL.Path == "/v1/images/edits":
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				writeFakeCangyuanJSON(w, http.StatusBadRequest, `{"error":{"type":"invalid_request_error"}}`)
				return
			}
			files := 0
			if r.MultipartForm != nil {
				files = len(r.MultipartForm.File["image"])
			}
			fake.mu.Lock()
			fake.calls = append(fake.calls, fakeCangyuanCall{
				method: r.Method, path: r.URL.Path, contentType: r.Header.Get("Content-Type"),
				model: r.FormValue("model"), prompt: r.FormValue("prompt"), size: r.FormValue("size"),
				imageCount: files, hasMask: r.MultipartForm != nil && len(r.MultipartForm.File["mask"]) == 1,
				async: r.FormValue("async") == "true",
			})
			fake.mu.Unlock()
			writeFakeCangyuanJSON(w, http.StatusOK, `{"task_id":"edit/task-2","status":"queued"}`)
			return
		}

		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.EscapedPath(), "/v1/images/") {
			taskID, operation, ok := fakeTaskFromPath(r.URL.EscapedPath())
			if !ok {
				writeFakeCangyuanJSON(w, http.StatusNotFound, `{"error":{"type":"not_found"}}`)
				return
			}
			fake.mu.Lock()
			fake.pollCounts[taskID]++
			pollNumber := fake.pollCounts[taskID]
			fake.mu.Unlock()

			// A transient upstream failure must be surfaced as retryable; the
			// caller can then poll the same task without resubmitting it.
			if operation == "generation" && pollNumber == 1 {
				writeFakeCangyuanJSON(w, http.StatusBadGateway, `{"error":{"type":"upstream_error"}}`)
				return
			}
			if operation == "edit" && pollNumber == 1 {
				writeFakeCangyuanJSON(w, http.StatusOK, `{"task_id":"edit/task-2","status":"processing"}`)
				return
			}
			if operation == "generation" {
				encoded := base64.StdEncoding.EncodeToString(fake.output)
				writeFakeCangyuanJSON(w, http.StatusOK, `{"task_id":"generation/task-1","status":"completed","data":[{"b64_json":"`+encoded+`"}]}`)
				return
			}
			writeFakeCangyuanJSON(w, http.StatusOK, `{"task_id":"edit/task-2","status":"completed","data":[{"url":"`+fake.server.URL+fake.outputPath+`"}]}`)
			return
		}

		http.NotFound(w, r)
	}))
	return fake
}

func writeFakeCangyuanJSON(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func fakeTaskFromPath(escapedPath string) (taskID, operation string, ok bool) {
	for _, candidate := range []string{"generations", "edits"} {
		prefix := "/v1/images/" + candidate + "/"
		if !strings.HasPrefix(escapedPath, prefix) {
			continue
		}
		taskID, err := url.PathUnescape(strings.TrimPrefix(escapedPath, prefix))
		if err != nil || taskID == "" {
			return "", "", false
		}
		return taskID, strings.TrimSuffix(candidate, "s"), true
	}
	return "", "", false
}

func (f *fakeCangyuanUpstream) Calls() []fakeCangyuanCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCangyuanCall(nil), f.calls...)
}

func TestCangyuanFakeUpstreamGenerationEditPollingAndResultContract(t *testing.T) {
	fake := newFakeCangyuanUpstream(t)
	defer fake.server.Close()

	adapter, err := NewCangyuanImageAdapter(fake.server.URL+"/v1", "fake-cangyuan-key", fake.server.Client())
	require.NoError(t, err)

	generation, err := adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel4K, Prompt: "a four-k dog poster", Size: "3840x2160", N: 1,
		Async: true, ImageSize: "4K", OutputResolution: "4K", ResponseFormat: "b64_json",
	})
	require.NoError(t, err)
	require.Equal(t, "generation/task-1", generation.UpstreamTaskID)

	_, err = adapter.PollGeneration(context.Background(), generation.UpstreamTaskID)
	var adapterErr *CangyuanAdapterError
	require.ErrorAs(t, err, &adapterErr)
	require.Equal(t, "image_upstream_unavailable", adapterErr.Code)
	require.True(t, adapterErr.Retryable)

	generation, err = adapter.PollGeneration(context.Background(), generation.UpstreamTaskID)
	require.NoError(t, err)
	require.True(t, generation.Completed)
	require.Len(t, generation.Data, 1)

	input := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 16)))
	secondImage := image.NewNRGBA(image.Rect(0, 0, 32, 16))
	secondImage.SetNRGBA(1, 1, color.NRGBA{B: 255, A: 255})
	secondInput := encodePNG(t, secondImage)
	maskImage := image.NewNRGBA(image.Rect(0, 0, 32, 16))
	maskImage.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	mask := encodePNG(t, maskImage)
	edit, err := adapter.SubmitEdit(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel2K, Prompt: "replace the background", Size: "2048x2048", N: 1,
		Async: true, Multipart: true, ResponseFormat: "url",
		Images: []string{imageAssetDataURL("image/png", input), imageAssetDataURL("image/png", secondInput)},
		Mask:   imageAssetDataURL("image/png", mask),
	})
	require.NoError(t, err)
	require.Equal(t, "edit/task-2", edit.UpstreamTaskID)

	edit, err = adapter.PollEdit(context.Background(), edit.UpstreamTaskID)
	require.NoError(t, err)
	require.False(t, edit.Completed)
	require.Equal(t, "processing", edit.Status)
	edit, err = adapter.PollEdit(context.Background(), edit.UpstreamTaskID)
	require.NoError(t, err)
	require.True(t, edit.Completed)
	require.Len(t, edit.Data, 1)
	require.Contains(t, edit.Data[0].URL, fake.outputPath)

	calls := fake.Calls()
	require.Len(t, calls, 2)
	require.Equal(t, "/v1/images/generations", calls[0].path)
	require.Equal(t, CangyuanImageModel4K, calls[0].model)
	require.Equal(t, "/v1/images/edits", calls[1].path)
	require.Contains(t, calls[1].contentType, "multipart/form-data")
	require.Equal(t, 2, calls[1].imageCount, "multipart image fields must be repeated, not collapsed")
	require.True(t, calls[1].hasMask)

	// The result store is the boundary between a provider result and a
	// user-visible object. Exercise both provider result forms from this one
	// fake upstream and prove that the upstream URL is not returned as storage
	// metadata.
	storage := &recordingImageStorage{}
	store := &CangyuanImageResultStore{
		storage: storage, httpClient: fake.server.Client(), maxBytes: cangyuanImageOutputMaxBytes,
		hostValidator: func(context.Context, string) (bool, error) { return false, nil },
		prefix:        "fake-results/",
	}
	refs, actualSize, err := store.Store(context.Background(), "generation-job", generation.Data)
	require.NoError(t, err)
	require.Equal(t, "32x16", actualSize)
	require.Equal(t, []string{"fake-results/generation-job/0.png"}, refs)
	refs, actualSize, err = store.Store(context.Background(), "edit-job", edit.Data)
	require.NoError(t, err)
	require.Equal(t, "32x16", actualSize)
	require.Equal(t, []string{"fake-results/edit-job/0.png"}, refs)
	require.Len(t, storage.data, 2)
}

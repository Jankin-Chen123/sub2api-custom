package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This is a local protocol smoke test. It deliberately uses a TLS httptest
// server and synthetic image data, so it exercises the real adapter and
// Worker without contacting a paid provider or storing an upstream secret.
func TestLocalCangyuanGenerationAndWorkerSmoke(t *testing.T) {
	var mu sync.Mutex
	seenModels := make(map[string]int)
	seenPaths := make([]string, 0, 8)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer local-smoke-key" {
			http.Error(w, `{"error":{"type":"invalid_request_error"}}`, http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost {
			if r.URL.Path != "/v1/images/generations" && r.URL.Path != "/v1/images/edits" {
				http.Error(w, `{"error":{"type":"invalid_request_error"}}`, http.StatusNotFound)
				return
			}
			var request CangyuanImageRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode local smoke request: %v", err)
				http.Error(w, `{"error":{"type":"invalid_request_error"}}`, http.StatusBadRequest)
				return
			}
			mu.Lock()
			seenModels[request.Model]++
			seenPaths = append(seenPaths, r.URL.Path)
			mu.Unlock()
			if request.Async {
				taskID := "local-smoke-generation-task"
				if r.URL.Path == "/v1/images/edits" {
					taskID = "local-smoke-edit-task"
				}
				_, _ = w.Write([]byte(`{"task_id":"` + taskID + `","status":"queued"}`))
				return
			}
			_, _ = w.Write([]byte(`{"created":1785800000,"data":[{"b64_json":"bG9jYWwtaW1hZ2U="}]}`))
			return
		}
		if r.Method == http.MethodGet {
			if strings.HasPrefix(r.URL.Path, "/v1/images/generations/") {
				_, _ = w.Write([]byte(`{"task_id":"local-smoke-generation-task","status":"completed","data":[{"b64_json":"bG9jYWwtaW1hZ2U="}]}`))
				return
			}
			if strings.HasPrefix(r.URL.Path, "/v1/images/edits/") {
				_, _ = w.Write([]byte(`{"task_id":"local-smoke-edit-task","status":"completed","data":[{"b64_json":"bG9jYWwtaW1hZ2U="}]}`))
				return
			}
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	adapter, err := NewCangyuanImageAdapter(server.URL+"/v1", "local-smoke-key", server.Client())
	require.NoError(t, err)
	for _, test := range []struct {
		model string
		size  string
		tier  string
	}{
		{CangyuanImageModel1K, "1024x1024", "1K"},
		{CangyuanImageModel2K, "2048x2048", "2K"},
		{CangyuanImageModel4K, "3840x2160", "4K"},
	} {
		result, submitErr := adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
			Model: test.model, Prompt: "local smoke", Size: test.size, N: 1,
			OutputResolution: test.tier,
		})
		require.NoError(t, submitErr)
		require.True(t, result.Completed)
		require.Len(t, result.Data, 1)
	}

	asyncResult, err := adapter.SubmitGeneration(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel4K, Prompt: "local async smoke", Size: "3840x2160", N: 1,
		OutputResolution: "4K", Async: true,
	})
	require.NoError(t, err)
	require.False(t, asyncResult.Completed)
	require.Equal(t, "local-smoke-generation-task", asyncResult.UpstreamTaskID)
	polled, err := adapter.PollGeneration(context.Background(), asyncResult.UpstreamTaskID)
	require.NoError(t, err)
	require.True(t, polled.Completed)

	editResult, err := adapter.SubmitEdit(context.Background(), CangyuanImageRequest{
		Model: CangyuanImageModel2K, Prompt: "local edit smoke", Size: "2048x2048", N: 1,
		Images: []string{"https://example.test/source.png"}, Mask: "https://example.test/mask.png", Async: true,
	})
	require.NoError(t, err)
	require.Equal(t, "local-smoke-edit-task", editResult.UpstreamTaskID)
	editPolled, err := adapter.PollEdit(context.Background(), editResult.UpstreamTaskID)
	require.NoError(t, err)
	require.True(t, editPolled.Completed)

	worker, repo, payloads, results, _, _, _ := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	worker.providers = &DefaultImageGenerationProviderFactory{HTTPClient: server.Client()}
	worker.opts.RetryDelay = time.Millisecond
	workerJob := repo.job
	workerJob.PublicModel = CangyuanImageModel4K
	workerJob.Status = ImageGenerationJobStatusQueued
	workerJob.AccountID = nil
	workerJob.UpstreamTaskID = nil
	workerJob.UpstreamModel = nil
	workerJob.ResultObjectRefs = nil
	workerJob.ActualSize = nil
	account := worker.accounts.(*imageWorkerAccountSelector).account
	account.Credentials["base_url"] = server.URL
	account.Credentials["api_key"] = "local-smoke-key"
	account.Credentials["model_mapping"] = map[string]any{CangyuanImageModel4K: CangyuanImageModel4K}
	payloads.payload.Request = CangyuanImageRequest{
		Model: CangyuanImageModel4K, Prompt: "local worker smoke", Size: "3840x2160", N: 1,
		OutputResolution: "4K", Async: true,
	}

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusSubmitted, repo.job.Status)
	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, 1, results.calls)

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, seenModels[CangyuanImageModel1K], 1)
	require.GreaterOrEqual(t, seenModels[CangyuanImageModel2K], 1)
	require.GreaterOrEqual(t, seenModels[CangyuanImageModel4K], 2)
	require.Contains(t, seenPaths, "/v1/images/generations")
	require.Contains(t, seenPaths, "/v1/images/edits")
}

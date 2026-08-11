package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type imageWorkbenchRepo struct {
	service.ImageGenerationJobRepository
	jobs       []*service.ImageGenerationJob
	lastUserID int64
	lastName   string
	deletedID  string
	lastSource string
	lastFilter service.ImageGenerationJobFilter
}

type imageWorkbenchCreateRepo struct {
	*imageWorkbenchRepo
}

func (r *imageWorkbenchCreateRepo) CreateImageGenerationJob(_ context.Context, params service.CreateImageGenerationJobParams) (*service.ImageGenerationJob, bool, error) {
	now := time.Now()
	job := &service.ImageGenerationJob{
		JobID: params.JobID, UserID: params.UserID, APIKeyID: params.APIKeyID,
		GroupID: params.GroupID, SubscriptionID: params.SubscriptionID,
		BillingType: params.BillingType, Source: params.Source, Operation: params.Operation,
		Status: params.Status, PublicModel: params.PublicModel, DisplayName: params.DisplayName, RequestedSize: params.RequestedSize,
		Quality: params.Quality, ResponseFormat: params.ResponseFormat,
		IdempotencyKey: params.IdempotencyKey, RequestHash: params.RequestHash,
		PromptHash: params.PromptHash, PayloadObjectRef: params.PayloadObjectRef,
		ResultObjectRefs: []string{}, BaseCost: params.BaseCost, RateMultiplier: params.RateMultiplier,
		EstimatedCost: params.EstimatedCost, HeldCost: params.HeldCost,
		CreatedAt: now, UpdatedAt: now,
	}
	r.jobs = append(r.jobs, job)
	return job, false, nil
}

type imageWorkbenchPayloadStore struct {
	saved map[string]*service.ImageGenerationPayload
}

func (s *imageWorkbenchPayloadStore) Save(_ context.Context, ref string, payload *service.ImageGenerationPayload, _ time.Duration) error {
	if s.saved == nil {
		s.saved = make(map[string]*service.ImageGenerationPayload)
	}
	copyValue := *payload
	s.saved[ref] = &copyValue
	return nil
}

func (s *imageWorkbenchPayloadStore) Get(_ context.Context, ref string) (*service.ImageGenerationPayload, error) {
	payload := s.saved[ref]
	if payload == nil {
		return nil, service.ErrImageGenerationPayloadNotFound
	}
	copyValue := *payload
	return &copyValue, nil
}

func (s *imageWorkbenchPayloadStore) Delete(_ context.Context, ref string) error {
	delete(s.saved, ref)
	return nil
}

type imageWorkbenchAPIKeyRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r *imageWorkbenchAPIKeyRepo) GetByID(context.Context, int64) (*service.APIKey, error) {
	return r.key, nil
}

func (r *imageWorkbenchRepo) GetImageGenerationJobForUser(_ context.Context, userID int64, jobID string) (*service.ImageGenerationJob, error) {
	r.lastUserID = userID
	for _, job := range r.jobs {
		if job != nil && job.JobID == jobID && job.UserID != nil && *job.UserID == userID {
			return job, nil
		}
	}
	return nil, service.ErrImageGenerationJobNotFound
}

func (r *imageWorkbenchRepo) ListImageGenerationJobsForOwner(_ context.Context, userID int64, filter service.ImageGenerationJobFilter) ([]*service.ImageGenerationJob, error) {
	r.lastUserID = userID
	r.lastFilter = filter
	result := make([]*service.ImageGenerationJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		if job == nil || job.UserID == nil || *job.UserID != userID {
			continue
		}
		if filter.Source != "" && job.Source != filter.Source {
			continue
		}
		if filter.Status != "" && job.Status != filter.Status {
			continue
		}
		if filter.Operation != "" && job.Operation != filter.Operation {
			continue
		}
		result = append(result, job)
	}
	return result, nil
}

func (r *imageWorkbenchRepo) RenameImageGenerationJobForUser(_ context.Context, userID int64, jobID, displayName string) (*service.ImageGenerationJob, error) {
	r.lastUserID = userID
	r.lastName = displayName
	for _, job := range r.jobs {
		if job != nil && job.JobID == jobID && job.UserID != nil && *job.UserID == userID && job.Source == service.ImageGenerationJobSourceWorkbench {
			job.DisplayName = &displayName
			return job, nil
		}
	}
	return nil, service.ErrImageGenerationJobNotFound
}

func (r *imageWorkbenchRepo) DeleteImageGenerationJobForUser(_ context.Context, userID int64, jobID, source string) error {
	r.lastUserID = userID
	r.deletedID = jobID
	r.lastSource = source
	for index, job := range r.jobs {
		if job != nil && job.JobID == jobID && job.UserID != nil && *job.UserID == userID && job.Source == source &&
			(job.Status == service.ImageGenerationJobStatusCompleted || job.Status == service.ImageGenerationJobStatusFailed) {
			r.jobs = append(r.jobs[:index], r.jobs[index+1:]...)
			return nil
		}
	}
	return service.ErrImageGenerationJobNotFound
}

type imageWorkbenchResultStore struct {
	dedicatedImageReader
	deletedRefs []string
}

func (s *imageWorkbenchResultStore) Delete(_ context.Context, ref string) error {
	s.deletedRefs = append(s.deletedRefs, ref)
	return nil
}

func setWorkbenchSubject(c *gin.Context, userID int64) {
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID, Concurrency: 1})
}

func TestWorkbenchJobIsScopedToJWTUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(11)
	job := &service.ImageGenerationJob{
		JobID: "imgjob_owned", UserID: &ownerID, Source: service.ImageGenerationJobSourceWorkbench,
		Status: service.ImageGenerationJobStatusPolling, Operation: service.ImageGenerationJobOperationGeneration,
		PublicModel: service.CangyuanImageModel2K, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	repo := &imageWorkbenchRepo{jobs: []*service.ImageGenerationJob{job}}
	h := &DedicatedImageHandler{repo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/image-workbench/jobs/imgjob_owned", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: job.JobID}}
	setWorkbenchSubject(c, 12)

	h.GetWorkbenchJob(c)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, int64(12), repo.lastUserID)
}

func TestWorkbenchRejectsNonWorkbenchSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(11)
	job := &service.ImageGenerationJob{
		JobID: "imgjob_api", UserID: &ownerID, Source: service.ImageGenerationJobSourceAPI,
		Status: service.ImageGenerationJobStatusCompleted, ResultObjectRefs: []string{"private/result.png"},
	}
	h := &DedicatedImageHandler{repo: &imageWorkbenchRepo{jobs: []*service.ImageGenerationJob{job}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/image-workbench/jobs/imgjob_api", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: job.JobID}}
	setWorkbenchSubject(c, ownerID)

	h.GetWorkbenchJob(c)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWorkbenchContentReturnsConflictUntilCompleted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(11)
	job := &service.ImageGenerationJob{
		JobID: "imgjob_pending", UserID: &ownerID, Source: service.ImageGenerationJobSourceWorkbench,
		Status: service.ImageGenerationJobStatusPolling,
	}
	h := &DedicatedImageHandler{repo: &imageWorkbenchRepo{jobs: []*service.ImageGenerationJob{job}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/image-workbench/jobs/imgjob_pending/content", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: job.JobID}}
	setWorkbenchSubject(c, ownerID)

	h.WorkbenchContent(c)

	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestWorkbenchContentHidesAnotherUsersCompletedResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(11)
	job := &service.ImageGenerationJob{
		JobID: "imgjob_private_content", UserID: &ownerID, Source: service.ImageGenerationJobSourceWorkbench,
		Status: service.ImageGenerationJobStatusCompleted, ResultObjectRefs: []string{"private/result.png"},
	}
	repo := &imageWorkbenchRepo{jobs: []*service.ImageGenerationJob{job}}
	reader := &dedicatedImageReader{raw: []byte("must-not-be-read")}
	h := &DedicatedImageHandler{repo: repo, results: reader}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/image-workbench/jobs/imgjob_private_content/content", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: job.JobID}}
	setWorkbenchSubject(c, 12)

	h.WorkbenchContent(c)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Empty(t, reader.ref)
}

func TestWorkbenchResponseDoesNotExposePromptOrUpstreamBinding(t *testing.T) {
	accountID := int64(91)
	upstreamTaskID := "private-upstream-task"
	requestHash := "private-request-hash"
	job := &service.ImageGenerationJob{
		JobID: "imgjob_public", Source: service.ImageGenerationJobSourceWorkbench,
		Status: service.ImageGenerationJobStatusPolling, PublicModel: service.CangyuanImageModel4K,
		AccountID: &accountID, UpstreamTaskID: &upstreamTaskID, RequestHash: &requestHash,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	raw, err := json.Marshal(workbenchImageJobResponse(job))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "prompt")
	require.NotContains(t, string(raw), "account_id")
	require.NotContains(t, string(raw), "upstream_task")
	require.NotContains(t, string(raw), requestHash)
}

func TestWorkbenchRenamePersistsOwnerScopedArtworkName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(11)
	job := &service.ImageGenerationJob{
		JobID: "imgjob_rename", UserID: &ownerID, Source: service.ImageGenerationJobSourceWorkbench,
		Status: service.ImageGenerationJobStatusCompleted, Operation: service.ImageGenerationJobOperationGeneration,
		PublicModel: service.CangyuanImageModel1K, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	repo := &imageWorkbenchRepo{jobs: []*service.ImageGenerationJob{job}}
	h := &DedicatedImageHandler{repo: repo}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/user/image-workbench/jobs/imgjob_rename", bytes.NewReader([]byte(`{"name":"  蓝色知更鸟  "}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: job.JobID}}
	setWorkbenchSubject(c, ownerID)

	h.RenameWorkbenchJob(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, ownerID, repo.lastUserID)
	require.Equal(t, "蓝色知更鸟", repo.lastName)
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "蓝色知更鸟", response["name"])
}

func TestWorkbenchRenameHidesAnotherUsersJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(11)
	job := &service.ImageGenerationJob{JobID: "imgjob_private_rename", UserID: &ownerID, Source: service.ImageGenerationJobSourceWorkbench}
	repo := &imageWorkbenchRepo{jobs: []*service.ImageGenerationJob{job}}
	h := &DedicatedImageHandler{repo: repo}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/user/image-workbench/jobs/imgjob_private_rename", bytes.NewReader([]byte(`{"name":"not allowed"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: job.JobID}}
	setWorkbenchSubject(c, 12)

	h.RenameWorkbenchJob(c)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Nil(t, job.DisplayName)
}

func TestWorkbenchDeleteRemovesOwnedCompletedArtworkAndStoredResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(11)
	job := &service.ImageGenerationJob{
		JobID: "imgjob_delete", UserID: &ownerID, Source: service.ImageGenerationJobSourceWorkbench,
		Status: service.ImageGenerationJobStatusCompleted, ResultObjectRefs: []string{"private/result.png"},
	}
	repo := &imageWorkbenchRepo{jobs: []*service.ImageGenerationJob{job}}
	results := &imageWorkbenchResultStore{}
	h := &DedicatedImageHandler{repo: repo, results: results}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/user/image-workbench/jobs/imgjob_delete", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: job.JobID}}
	setWorkbenchSubject(c, ownerID)

	h.DeleteWorkbenchJob(c)

	require.Equal(t, http.StatusNoContent, c.Writer.Status())
	c.Writer.WriteHeaderNow()
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, ownerID, repo.lastUserID)
	require.Equal(t, job.JobID, repo.deletedID)
	require.Equal(t, service.ImageGenerationJobSourceWorkbench, repo.lastSource)
	require.Equal(t, []string{"private/result.png"}, results.deletedRefs)
	require.Empty(t, repo.jobs)
}

func TestWorkbenchDeleteRejectsRunningTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(11)
	job := &service.ImageGenerationJob{
		JobID: "imgjob_running", UserID: &ownerID, Source: service.ImageGenerationJobSourceWorkbench,
		Status: service.ImageGenerationJobStatusPolling,
	}
	repo := &imageWorkbenchRepo{jobs: []*service.ImageGenerationJob{job}}
	h := &DedicatedImageHandler{repo: repo}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/user/image-workbench/jobs/imgjob_running", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: job.JobID}}
	setWorkbenchSubject(c, ownerID)

	h.DeleteWorkbenchJob(c)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.Empty(t, repo.deletedID)
	require.Len(t, repo.jobs, 1)
}

func TestWorkbenchAPIKeyMustBelongToJWTUser(t *testing.T) {
	apiKey := &service.APIKey{
		ID: 21, UserID: 11, Status: service.StatusActive,
		Group: &service.Group{Platform: service.PlatformOpenAI, AllowImageGeneration: true},
	}

	require.True(t, workbenchAPIKeyEligible(11, apiKey))
	require.False(t, workbenchAPIKeyEligible(12, apiKey))
}

func TestWorkbenchListAppliesPaginationAndStatusFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(11)
	otherID := int64(12)
	now := time.Now()
	repo := &imageWorkbenchRepo{jobs: []*service.ImageGenerationJob{
		{JobID: "imgjob_match", UserID: &ownerID, Source: service.ImageGenerationJobSourceWorkbench, Status: service.ImageGenerationJobStatusCompleted, Operation: service.ImageGenerationJobOperationGeneration, PublicModel: service.CangyuanImageModel1K, CreatedAt: now, UpdatedAt: now},
		{JobID: "imgjob_other", UserID: &otherID, Source: service.ImageGenerationJobSourceWorkbench, Status: service.ImageGenerationJobStatusCompleted, Operation: service.ImageGenerationJobOperationGeneration, PublicModel: service.CangyuanImageModel1K, CreatedAt: now, UpdatedAt: now},
	}}
	h := &DedicatedImageHandler{repo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/image-workbench/jobs?status=completed&operation=generation&limit=7&offset=3", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	setWorkbenchSubject(c, ownerID)

	h.ListWorkbenchJobs(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(11), repo.lastUserID)
	require.Equal(t, 7, repo.lastFilter.Limit)
	require.Equal(t, 3, repo.lastFilter.Offset)
	require.Equal(t, service.ImageGenerationJobSourceWorkbench, repo.lastFilter.Source)
	require.Equal(t, service.ImageGenerationJobStatusCompleted, repo.lastFilter.Status)
	require.Equal(t, service.ImageGenerationJobOperationGeneration, repo.lastFilter.Operation)
	var body struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	require.Equal(t, "imgjob_match", body.Data[0]["id"])
}

func TestWorkbenchCreateHTTPHandlerPersistsDedicatedJobAndPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID, apiKeyID, groupID := int64(11), int64(22), int64(33)
	price := 0.1
	apiKey := &service.APIKey{
		ID: apiKeyID, UserID: userID, Key: "sk-local-workbench",
		Status: service.StatusActive, User: &service.User{ID: userID},
		GroupID: &groupID,
		Group: &service.Group{
			ID: groupID, Platform: service.PlatformOpenAI,
			AllowImageGeneration: true, RateMultiplier: 1, ImagePrice1K: &price,
		},
	}

	cfg := &config.Config{
		RunMode:        config.RunModeSimple,
		DedicatedImage: config.DedicatedImageConfig{Enabled: true, WorkerEnabled: true},
	}
	apiKeyService := service.NewAPIKeyService(
		&imageWorkbenchAPIKeyRepo{key: apiKey}, nil, nil, nil, nil, nil, cfg,
	)
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	defer billingCache.Stop()
	concurrency := service.NewConcurrencyService(nil)
	billing := service.NewBillingService(cfg, nil)
	openAIGatewayService := service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil, nil, cfg, nil, concurrency, billing, nil,
		billingCache, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	openAI := NewOpenAIGatewayHandler(openAIGatewayService, concurrency, billingCache, apiKeyService, nil, nil, nil, nil, cfg)

	repo := &imageWorkbenchCreateRepo{imageWorkbenchRepo: &imageWorkbenchRepo{}}
	payloads := &imageWorkbenchPayloadStore{}
	orchestrator := service.NewImageGenerationOrchestrator(repo, payloads, time.Hour)
	// The handler only needs the runtime gate for this admission test. The
	// actual worker lifecycle is covered by service-level flow tests.
	runtime := service.NewImageGenerationWorkerRuntime(&service.ImageGenerationWorker{})
	runtime.Start()
	defer runtime.Stop()
	h := NewDedicatedImageHandler(orchestrator, repo, nil, billing, runtime, openAI, nil, cfg)

	estimateReq := httptest.NewRequest(http.MethodPost, "/api/v1/user/image-workbench/estimate", bytes.NewReader([]byte(`{"api_key_id":22,"model":"gpt-image-2-1k"}`)))
	estimateReq.Header.Set("Content-Type", "application/json")
	estimateRec := httptest.NewRecorder()
	estimateCtx, _ := gin.CreateTestContext(estimateRec)
	estimateCtx.Request = estimateReq
	setWorkbenchSubject(estimateCtx, userID)
	h.EstimateWorkbenchCost(estimateCtx)
	require.Equal(t, http.StatusOK, estimateRec.Code)
	var estimateResponse map[string]any
	require.NoError(t, json.Unmarshal(estimateRec.Body.Bytes(), &estimateResponse))
	require.Equal(t, service.CangyuanImageModel1K, estimateResponse["model"])
	require.Equal(t, 0.1, estimateResponse["estimated_cost"])

	body := []byte(`{"api_key_id":22,"model":"gpt-image-2-1k","prompt":"draw a small orange puppy","size":"1024x1024"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/image-workbench/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	setWorkbenchSubject(c, userID)

	h.CreateWorkbenchJob(c)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Len(t, repo.jobs, 1)
	job := repo.jobs[0]
	require.Equal(t, service.ImageGenerationJobSourceWorkbench, job.Source)
	require.Equal(t, service.ImageGenerationJobOperationGeneration, job.Operation)
	require.Equal(t, service.CangyuanImageModel1K, job.PublicModel)
	require.NotNil(t, job.DisplayName)
	require.Equal(t, "draw a small orange puppy", *job.DisplayName)
	require.NotNil(t, job.PayloadObjectRef)
	stored := payloads.saved[*job.PayloadObjectRef]
	require.NotNil(t, stored)
	require.Equal(t, "draw a small orange puppy", stored.Request.Prompt)

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, job.JobID, response["id"])
	require.Equal(t, "draw a small orange puppy", response["name"])
	require.Equal(t, "auto", response["quality"])
	require.Equal(t, "queued", response["status"])
}

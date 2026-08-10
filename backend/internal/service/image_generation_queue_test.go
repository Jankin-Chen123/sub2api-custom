package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type imageGenerationQueueSettingsRepoStub struct {
	values map[string]string
	err    error
}

func (r *imageGenerationQueueSettingsRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, errors.New("not implemented")
}
func (r *imageGenerationQueueSettingsRepoStub) GetValue(context.Context, string) (string, error) {
	return "", errors.New("not implemented")
}
func (r *imageGenerationQueueSettingsRepoStub) Set(context.Context, string, string) error {
	return nil
}
func (r *imageGenerationQueueSettingsRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}
func (r *imageGenerationQueueSettingsRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (r *imageGenerationQueueSettingsRepoStub) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}
func (r *imageGenerationQueueSettingsRepoStub) Delete(context.Context, string) error { return nil }

type imageGenerationQueueCounterRepoStub struct {
	ImageGenerationJobRepository
	count int
	err   error
}

func (r *imageGenerationQueueCounterRepoStub) CountQueuedImageGenerationJobs(context.Context) (int, error) {
	return r.count, r.err
}

func TestImageGenerationQueueSettingsUseSafeDefaultsAndRuntimeValues(t *testing.T) {
	defaults, err := (&SettingService{}).GetImageGenerationQueueSettings(context.Background())
	require.NoError(t, err)
	require.True(t, defaults.Enabled)
	require.Equal(t, 1, defaults.MaxActive)
	require.Equal(t, 100, defaults.MaxQueued)

	repo := &imageGenerationQueueSettingsRepoStub{values: map[string]string{
		SettingKeyImageGenerationQueueEnabled:  "false",
		SettingKeyImageGenerationMaxActiveJobs: "4",
		SettingKeyImageGenerationMaxQueuedJobs: "250",
	}}
	settings, err := (&SettingService{settingRepo: repo}).GetImageGenerationQueueSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, 4, settings.MaxActive)
	require.Equal(t, 250, settings.MaxQueued)
}

func TestImageGenerationQueueControllerChecksPersistentWaitingLimit(t *testing.T) {
	settingsRepo := &imageGenerationQueueSettingsRepoStub{values: map[string]string{
		SettingKeyImageGenerationQueueEnabled:  "true",
		SettingKeyImageGenerationMaxActiveJobs: "1",
		SettingKeyImageGenerationMaxQueuedJobs: "2",
	}}
	queue := NewImageGenerationQueueController(
		&imageGenerationQueueCounterRepoStub{count: 2},
		&SettingService{settingRepo: settingsRepo},
		nil,
	)
	allowed, err := queue.CanEnqueue(context.Background())
	require.NoError(t, err)
	require.False(t, allowed)

	queue.repo = &imageGenerationQueueCounterRepoStub{count: 1}
	allowed, err = queue.CanEnqueue(context.Background())
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestImageGenerationQueueLeaseIsRenewedUntilAsyncTaskCompletes(t *testing.T) {
	worker, repo, _, _, _, _, client := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	client.submitResults = []*CangyuanImageResult{{Status: "queued", UpstreamTaskID: "upstream-task"}}
	client.pollResults = []*CangyuanImageResult{{Completed: true, Data: []CangyuanImageData{{URL: "https://temporary.example/image"}}}}

	cache := &imageAdmissionCacheStub{acquire: []bool{true, true}}
	worker.SetQueueController(NewImageGenerationQueueController(nil, nil, NewConcurrencyService(cache)))

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusSubmitted, repo.job.Status)
	require.Zero(t, cache.released, "an accepted upstream task must keep its server slot")

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, 1, cache.released)
}

func TestImageGenerationQueueReleaseRunsForTerminalWorkerPaths(t *testing.T) {
	worker, repo, _, _, _, _, client := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	client.submitResults = []*CangyuanImageResult{{Completed: true, Data: []CangyuanImageData{{B64JSON: "result"}}}}
	cache := &imageAdmissionCacheStub{acquire: []bool{true}}
	worker.SetQueueController(NewImageGenerationQueueController(nil, nil, NewConcurrencyService(cache)))

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusCompleted, repo.job.Status)
	require.Equal(t, 1, cache.released)
}

func TestImageGenerationQueueReleaseRunsForSubmissionUnknown(t *testing.T) {
	worker, repo, _, _, billing, _, client := newImageWorkerFixture(ImageGenerationJobStatusQueued)
	client.submitErrors = []error{&CangyuanAdapterError{
		Code:              "image_upstream_timeout",
		SubmissionUnknown: true,
		Err:               errors.New("network outcome unknown"),
	}}
	cache := &imageAdmissionCacheStub{acquire: []bool{true}}
	worker.SetQueueController(NewImageGenerationQueueController(nil, nil, NewConcurrencyService(cache)))

	require.NoError(t, worker.RunOnce(context.Background()))
	require.Equal(t, ImageGenerationJobStatusSubmissionUnknown, repo.job.Status)
	require.Equal(t, 1, cache.released)
	require.Zero(t, billing.releaseCalls, "an unknown submission must keep its billing hold for reconciliation")
}

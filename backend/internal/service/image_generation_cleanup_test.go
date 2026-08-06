package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageGenerationCleanupRepoStub struct {
	ImageGenerationJobRepository
	jobs    []*ImageGenerationJob
	deleted []string
}

func (r *imageGenerationCleanupRepoStub) ListImageGenerationJobsForCleanup(_ context.Context, _ time.Time, _ int) ([]*ImageGenerationJob, error) {
	return r.jobs, nil
}

func (r *imageGenerationCleanupRepoStub) DeleteImageGenerationJob(_ context.Context, jobID string) error {
	r.deleted = append(r.deleted, jobID)
	return nil
}

type imageGenerationCleanupPayloadStub struct {
	deleted []string
	err     error
}

func (s *imageGenerationCleanupPayloadStub) Save(context.Context, string, *ImageGenerationPayload, time.Duration) error {
	return nil
}

func (s *imageGenerationCleanupPayloadStub) Get(context.Context, string) (*ImageGenerationPayload, error) {
	return nil, ErrImageGenerationPayloadNotFound
}

func (s *imageGenerationCleanupPayloadStub) Delete(_ context.Context, ref string) error {
	if s.err != nil {
		return s.err
	}
	s.deleted = append(s.deleted, ref)
	return nil
}

type imageGenerationCleanupDeleterStub struct {
	deleted []string
	err     error
}

func (s *imageGenerationCleanupDeleterStub) Delete(_ context.Context, ref string) error {
	if s.err != nil {
		return s.err
	}
	s.deleted = append(s.deleted, ref)
	return nil
}

func TestImageGenerationCleanupDeletesObjectsBeforeJob(t *testing.T) {
	job := &ImageGenerationJob{
		JobID:            "imgjob_cleanup",
		Status:           ImageGenerationJobStatusCompleted,
		PayloadObjectRef: stringPointer("image-generation/imgjob_cleanup"),
		ResultObjectRefs: []string{"images/cangyuan/imgjob_cleanup/0.png"},
	}
	repo := &imageGenerationCleanupRepoStub{jobs: []*ImageGenerationJob{job}}
	payloads := &imageGenerationCleanupPayloadStub{}
	deleter := &imageGenerationCleanupDeleterStub{}
	cleanup := NewImageGenerationCleanupService(repo, payloads, nil, deleter, time.Hour, time.Hour, 10)

	deleted, err := cleanup.RunOnce(context.Background(), time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, []string{"images/cangyuan/imgjob_cleanup/0.png"}, deleter.deleted)
	require.Equal(t, []string{"image-generation/imgjob_cleanup"}, payloads.deleted)
	require.Equal(t, []string{"imgjob_cleanup"}, repo.deleted)
}

func TestImageGenerationCleanupRetainsRowsWhenObjectsCannotBeDeleted(t *testing.T) {
	job := &ImageGenerationJob{
		JobID:            "imgjob_no_deleter",
		Status:           ImageGenerationJobStatusFailed,
		ResultObjectRefs: []string{"images/cangyuan/imgjob_no_deleter/0.png"},
	}
	repo := &imageGenerationCleanupRepoStub{jobs: []*ImageGenerationJob{job}}
	cleanup := NewImageGenerationCleanupService(repo, nil, nil, nil, time.Hour, time.Hour, 10)

	deleted, err := cleanup.RunOnce(context.Background(), time.Now())
	require.NoError(t, err)
	require.Zero(t, deleted)
	require.Empty(t, repo.deleted)
}

func TestImageGenerationCleanupRetriesObjectDeletionBeforeRemovingRow(t *testing.T) {
	job := &ImageGenerationJob{JobID: "imgjob_retry", ResultObjectRefs: []string{"object"}}
	repo := &imageGenerationCleanupRepoStub{jobs: []*ImageGenerationJob{job}}
	deleter := &imageGenerationCleanupDeleterStub{err: errors.New("storage unavailable")}
	cleanup := NewImageGenerationCleanupService(repo, nil, nil, deleter, time.Hour, time.Hour, 10)

	deleted, err := cleanup.RunOnce(context.Background(), time.Now())
	require.NoError(t, err)
	require.Zero(t, deleted)
	require.Empty(t, repo.deleted)
}

func TestImageGenerationCleanupRetainsRowsWhenPayloadDeletionFails(t *testing.T) {
	job := &ImageGenerationJob{
		JobID:            "imgjob_payload_retry",
		PayloadObjectRef: stringPointer("image-generation/imgjob_payload_retry"),
	}
	repo := &imageGenerationCleanupRepoStub{jobs: []*ImageGenerationJob{job}}
	payloads := &imageGenerationCleanupPayloadStub{err: errors.New("redis unavailable")}
	cleanup := NewImageGenerationCleanupService(repo, payloads, nil, nil, time.Hour, time.Hour, 10)

	deleted, err := cleanup.RunOnce(context.Background(), time.Now())
	require.NoError(t, err)
	require.Zero(t, deleted)
	require.Empty(t, repo.deleted)
}

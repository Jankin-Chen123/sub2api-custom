package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

var imageGenerationJobTestColumns = []string{
	"id", "job_id", "user_id", "api_key_id", "group_id", "subscription_id", "account_id", "billing_type",
	"source", "operation", "status", "public_model", "upstream_model",
	"requested_size", "actual_size", "quality", "response_format",
	"upstream_task_id", "idempotency_key", "request_hash", "prompt_hash",
	"payload_object_ref", "result_object_refs",
	"base_cost", "rate_multiplier", "estimated_cost", "held_cost", "settled_cost",
	"error_code", "error_message", "attempt_count", "claim_version",
	"lease_expires_at", "next_attempt_at",
	"created_at", "updated_at", "submitted_at", "completed_at", "settled_at",
}

func TestImageGenerationJobRepositoryCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	now := time.Now().UTC()
	userID := int64(11)
	apiKeyID := int64(22)
	groupID := int64(33)
	size := "1024x1024"

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO image_generation_jobs")).
		WithArgs(
			"imgjob_test", &userID, &apiKeyID, &groupID, nil, int8(0),
			service.ImageGenerationJobSourceAPI, service.ImageGenerationJobOperationGeneration, service.ImageGenerationJobStatusQueued,
			service.CangyuanImageModel1K, &size, nil, nil,
			nil, nil, "prompt-hash", nil, float64(0), float64(0), float64(1), float64(1),
		).
		WillReturnRows(newImageGenerationJobRow(now,
			"imgjob_test", service.ImageGenerationJobStatusQueued,
			&userID, &apiKeyID, &groupID, nil, nil, nil, 0, 0,
		))

	repository := &imageGenerationJobRepository{db: db, sql: db}
	job, replayed, err := repository.CreateImageGenerationJob(context.Background(), service.CreateImageGenerationJobParams{
		JobID: "imgjob_test", UserID: &userID, APIKeyID: &apiKeyID, GroupID: &groupID,
		Source: service.ImageGenerationJobSourceAPI, Operation: service.ImageGenerationJobOperationGeneration,
		Status: service.ImageGenerationJobStatusQueued, PublicModel: service.CangyuanImageModel1K,
		RequestedSize: &size, PromptHash: "prompt-hash", EstimatedCost: 1, HeldCost: 1,
	})
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, "imgjob_test", job.JobID)
	require.Equal(t, []string{}, job.ResultObjectRefs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGenerationJobRepositoryAtomicQueueAdmissionRejectsFullQueue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended('image_generation_queue_admission', 0))")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM image_generation_jobs`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	repository := &imageGenerationJobRepository{db: db, sql: db}
	userID, apiKeyID := int64(11), int64(22)
	_, _, err = repository.CreateImageGenerationJobWithQueueLimit(context.Background(), service.CreateImageGenerationJobParams{
		JobID: "imgjob_queue_full", UserID: &userID, APIKeyID: &apiKeyID,
		Source: service.ImageGenerationJobSourceAPI, Operation: service.ImageGenerationJobOperationGeneration,
		Status: service.ImageGenerationJobStatusQueued, PublicModel: service.CangyuanImageModel1K,
		PromptHash: "prompt-hash",
	}, 0)
	require.ErrorIs(t, err, service.ErrImageGenerationQueueFull)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGenerationJobRepositoryIdempotentReplayAndConflict(t *testing.T) {
	t.Run("replay", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()
		now := time.Now().UTC()
		userID := int64(11)
		apiKeyID := int64(22)
		key := "same-key"
		hash := "same-hash"

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))")).
			WithArgs("11:22:workbench:same-key").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`(?s)SELECT .* FROM image_generation_jobs.*idempotency_key = \$4`).
			WithArgs(int64(11), int64(22), service.ImageGenerationJobSourceWorkbench, key).
			WillReturnRows(newImageGenerationJobRow(now,
				"imgjob_existing", service.ImageGenerationJobStatusQueued,
				&userID, &apiKeyID, nil, nil, &key, &hash, 0, 0,
			))
		mock.ExpectCommit()

		repository := &imageGenerationJobRepository{db: db, sql: db}
		job, replayed, err := repository.CreateImageGenerationJob(context.Background(), service.CreateImageGenerationJobParams{
			JobID: "imgjob_new", UserID: &userID, APIKeyID: &apiKeyID,
			Source: service.ImageGenerationJobSourceWorkbench, Operation: service.ImageGenerationJobOperationGeneration,
			Status: service.ImageGenerationJobStatusQueued, PublicModel: service.CangyuanImageModel1K,
			IdempotencyKey: &key, RequestHash: &hash, PromptHash: "prompt-hash",
		})
		require.NoError(t, err)
		require.True(t, replayed)
		require.Equal(t, "imgjob_existing", job.JobID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("conflict", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()
		now := time.Now().UTC()
		userID := int64(11)
		apiKeyID := int64(22)
		key := "same-key"
		oldHash := "old-hash"
		newHash := "new-hash"

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))")).
			WithArgs("11:22:workbench:same-key").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`(?s)SELECT .* FROM image_generation_jobs.*idempotency_key = \$4`).
			WithArgs(int64(11), int64(22), service.ImageGenerationJobSourceWorkbench, key).
			WillReturnRows(newImageGenerationJobRow(now,
				"imgjob_existing", service.ImageGenerationJobStatusQueued,
				&userID, &apiKeyID, nil, nil, &key, &oldHash, 0, 0,
			))
		mock.ExpectRollback()

		repository := &imageGenerationJobRepository{db: db, sql: db}
		_, _, err = repository.CreateImageGenerationJob(context.Background(), service.CreateImageGenerationJobParams{
			JobID: "imgjob_new", UserID: &userID, APIKeyID: &apiKeyID,
			Source: service.ImageGenerationJobSourceWorkbench, Operation: service.ImageGenerationJobOperationGeneration,
			Status: service.ImageGenerationJobStatusQueued, PublicModel: service.CangyuanImageModel1K,
			IdempotencyKey: &key, RequestHash: &newHash, PromptHash: "prompt-hash",
		})
		require.ErrorIs(t, err, service.ErrImageGenerationIdempotency)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestImageGenerationJobRepositoryClaimAndFence(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	now := time.Now().UTC().Truncate(time.Second)
	leaseUntil := now.Add(time.Minute)

	mock.ExpectQuery(`(?s)WITH candidate AS .*FOR UPDATE SKIP LOCKED.*UPDATE image_generation_jobs`).
		WithArgs(now, leaseUntil).
		WillReturnRows(newImageGenerationJobRow(now,
			"imgjob_claim", service.ImageGenerationJobStatusSubmitting,
			nil, nil, nil, nil, nil, nil, 1, 7,
		))

	repository := &imageGenerationJobRepository{db: db, sql: db}
	job, err := repository.ClaimNextImageGenerationJob(context.Background(), now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, service.ImageGenerationJobStatusSubmitting, job.Status)
	require.Equal(t, int64(7), job.ClaimVersion)
	require.Equal(t, 1, job.AttemptCount)

	mock.ExpectExec(`(?s)UPDATE image_generation_jobs.*claim_version = \$2`).
		WithArgs("imgjob_claim", int64(7), leaseUntil.Add(time.Minute)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	err = repository.RenewImageGenerationJobLease(context.Background(), "imgjob_claim", 7, leaseUntil.Add(time.Minute))
	require.ErrorIs(t, err, service.ErrImageGenerationClaimLost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGenerationJobRepositorySubmittedStickyAndTerminalFencing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repository := &imageGenerationJobRepository{db: db, sql: db}
	now := time.Now().UTC()

	mock.ExpectExec(`(?s)UPDATE image_generation_jobs.*status = 'submitted'.*account_id = \$3.*upstream_task_id = \$5`).
		WithArgs("imgjob_test", int64(3), int64(42), service.CangyuanImageModel2K, "private-upstream-task", now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repository.MarkImageGenerationJobSubmitted(
		context.Background(), "imgjob_test", 3, 42, service.CangyuanImageModel2K, "private-upstream-task", now,
	))

	mock.ExpectExec(`(?s)UPDATE image_generation_jobs.*status = 'completed'.*claim_version = \$2`).
		WithArgs("imgjob_test", int64(2), float64(1.5), now).
		WillReturnResult(sqlmock.NewResult(0, 0))
	err = repository.MarkImageGenerationJobCompleted(context.Background(), "imgjob_test", 2, 1.5, now)
	require.ErrorIs(t, err, service.ErrImageGenerationClaimLost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGenerationJobRepositoryQueuesHeldCreatedJobAndStoresSyncResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repository := &imageGenerationJobRepository{db: db, sql: db}
	now := time.Now().UTC()

	mock.ExpectExec(`(?s)UPDATE image_generation_jobs.*status = 'queued'.*held_cost = \$3.*status = 'created'`).
		WithArgs("imgjob_created", int64(4), float64(0.25), now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repository.QueueImageGenerationJob(context.Background(), "imgjob_created", 4, 0.25, now))

	mock.ExpectExec(`(?s)UPDATE image_generation_jobs.*status = 'storing'.*account_id = \$3.*upstream_model = \$4.*status = 'submitting'`).
		WithArgs("imgjob_sync", int64(5), int64(42), service.CangyuanImageModel4K, "3840x2160", now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repository.MarkImageGenerationJobStoringFromSubmission(
		context.Background(), "imgjob_sync", 5, 42, service.CangyuanImageModel4K, "3840x2160", now,
	))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGenerationJobRepositoryRecoversSubmissionAsUnknownWithoutResubmit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repository := &imageGenerationJobRepository{db: db, sql: db}
	now := time.Now().UTC()

	mock.ExpectQuery(`(?s)WITH expired AS .*FOR UPDATE SKIP LOCKED.*status = 'submitting' THEN 'submission_unknown'.*RETURNING job\.job_id, job\.status`).
		WithArgs(now, 100).
		WillReturnRows(sqlmock.NewRows([]string{"job_id", "status"}).
			AddRow("imgjob_unknown_1", service.ImageGenerationJobStatusSubmissionUnknown).
			AddRow("imgjob_unknown_2", service.ImageGenerationJobStatusSubmissionUnknown))
	recovered, err := repository.RecoverExpiredImageGenerationJobLeases(context.Background(), now, 100)
	require.NoError(t, err)
	require.Len(t, recovered, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newImageGenerationJobRow(
	now time.Time,
	jobID string,
	status string,
	userID, apiKeyID, groupID, accountID *int64,
	idempotencyKey, requestHash *string,
	attemptCount int,
	claimVersion int64,
) *sqlmock.Rows {
	return sqlmock.NewRows(imageGenerationJobTestColumns).AddRow(
		int64(1), jobID, nullableInt64(userID), nullableInt64(apiKeyID), nullableInt64(groupID), nil, nullableInt64(accountID), int8(0),
		service.ImageGenerationJobSourceWorkbench, service.ImageGenerationJobOperationGeneration, status, service.CangyuanImageModel1K, nil,
		"1024x1024", nil, nil, "url",
		nil, nullableString(idempotencyKey), nullableString(requestHash), "prompt-hash",
		nil, []byte(`[]`),
		float64(0), float64(0), float64(0), float64(0), float64(0),
		nil, nil, attemptCount, claimVersion,
		nil, nil,
		now, now, nil, nil, nil,
	)
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

var _ imageGenerationJobSQLExecutor = (*sql.Tx)(nil)

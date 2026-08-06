package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type imageGenerationJobSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type imageGenerationJobRepository struct {
	db  *sql.DB
	sql imageGenerationJobSQLExecutor
}

func NewImageGenerationJobRepository(db *sql.DB) service.ImageGenerationJobRepository {
	return &imageGenerationJobRepository{db: db, sql: db}
}

func (r *imageGenerationJobRepository) CreateImageGenerationJob(ctx context.Context, params service.CreateImageGenerationJobParams) (*service.ImageGenerationJob, bool, error) {
	if params.JobID == "" {
		jobID, err := service.NewImageGenerationJobID()
		if err != nil {
			return nil, false, err
		}
		params.JobID = jobID
	}
	if params.Status == "" {
		params.Status = service.ImageGenerationJobStatusCreated
	}
	if params.IdempotencyKey == nil || strings.TrimSpace(*params.IdempotencyKey) == "" || r.db == nil {
		job, err := insertImageGenerationJob(ctx, r.sql, params)
		if err != nil {
			return nil, false, translatePersistenceError(err, nil, service.ErrImageGenerationJobExists)
		}
		return job, false, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	scope := imageGenerationIdempotencyScope(params)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, scope); err != nil {
		return nil, false, err
	}
	existing, findErr := getImageGenerationJobByIdempotency(ctx, tx, params)
	if findErr == nil {
		if !nullableStringsEqual(existing.RequestHash, params.RequestHash) {
			return nil, false, service.ErrImageGenerationIdempotency
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return existing, true, nil
	}
	if !errors.Is(findErr, sql.ErrNoRows) {
		return nil, false, findErr
	}
	job, err := insertImageGenerationJob(ctx, tx, params)
	if err != nil {
		return nil, false, translatePersistenceError(err, nil, service.ErrImageGenerationJobExists)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return job, false, nil
}

func (r *imageGenerationJobRepository) GetImageGenerationJob(ctx context.Context, jobID string) (*service.ImageGenerationJob, error) {
	job, err := scanImageGenerationJob(r.sql.QueryRowContext(ctx, imageGenerationJobSelectSQL+` WHERE job_id = $1`, jobID))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrImageGenerationJobNotFound, nil)
	}
	return job, nil
}

func (r *imageGenerationJobRepository) GetImageGenerationJobForUser(ctx context.Context, userID int64, jobID string) (*service.ImageGenerationJob, error) {
	job, err := scanImageGenerationJob(r.sql.QueryRowContext(ctx, imageGenerationJobSelectSQL+`
 WHERE job_id = $1 AND user_id = $2`, jobID, userID))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrImageGenerationJobNotFound, nil)
	}
	return job, nil
}

func (r *imageGenerationJobRepository) GetImageGenerationJobForOwner(ctx context.Context, userID, apiKeyID int64, jobID string) (*service.ImageGenerationJob, error) {
	job, err := scanImageGenerationJob(r.sql.QueryRowContext(ctx, imageGenerationJobSelectSQL+`
 WHERE job_id = $1 AND user_id = $2 AND api_key_id = $3`, jobID, userID, apiKeyID))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrImageGenerationJobNotFound, nil)
	}
	return job, nil
}

func (r *imageGenerationJobRepository) ListImageGenerationJobsForOwner(ctx context.Context, userID int64, filter service.ImageGenerationJobFilter) ([]*service.ImageGenerationJob, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	query := imageGenerationJobSelectSQL + ` WHERE user_id = $1`
	args := []any{userID}
	if filter.Source != "" {
		query += " AND source = $" + strconv.Itoa(len(args)+1)
		args = append(args, filter.Source)
	}
	if filter.Status != "" {
		switch filter.Status {
		case "queued":
			query += " AND status IN ('created', 'queued')"
		case "in_progress":
			query += " AND status IN ('planning', 'submitting', 'submitted', 'polling', 'storing', 'settling')"
		default:
			query += " AND status = $" + strconv.Itoa(len(args)+1)
			args = append(args, filter.Status)
		}
	}
	if filter.Operation != "" {
		query += " AND operation = $" + strconv.Itoa(len(args)+1)
		args = append(args, filter.Operation)
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, limit, filter.Offset)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	jobs := make([]*service.ImageGenerationJob, 0)
	for rows.Next() {
		job, scanErr := scanImageGenerationJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *imageGenerationJobRepository) ClaimNextImageGenerationJob(ctx context.Context, now time.Time, leaseDuration time.Duration) (*service.ImageGenerationJob, error) {
	if leaseDuration <= 0 {
		leaseDuration = time.Minute
	}
	leaseUntil := now.Add(leaseDuration)
	job, err := scanImageGenerationJob(r.sql.QueryRowContext(ctx, `
WITH candidate AS (
    SELECT id AS candidate_id
    FROM image_generation_jobs
    WHERE status IN ('created', 'queued', 'submitted', 'polling', 'storing', 'settling')
      AND (next_attempt_at IS NULL OR next_attempt_at <= $1)
      AND (lease_expires_at IS NULL OR lease_expires_at <= $1)
    ORDER BY
      CASE status
        WHEN 'storing' THEN 1
        WHEN 'settling' THEN 2
        WHEN 'submitted' THEN 3
        WHEN 'polling' THEN 4
        WHEN 'created' THEN 5
        ELSE 6
      END,
      COALESCE(next_attempt_at, created_at),
      id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE image_generation_jobs AS job
SET status = CASE job.status
        WHEN 'queued' THEN 'submitting'
        WHEN 'submitted' THEN 'polling'
        ELSE job.status
    END,
    attempt_count = CASE WHEN job.status = 'queued' THEN job.attempt_count + 1 ELSE job.attempt_count END,
    claim_version = job.claim_version + 1,
    lease_expires_at = $2,
    updated_at = $1
 FROM candidate
 WHERE job.id = candidate.candidate_id
   AND job.status IN ('created', 'queued', 'submitted', 'polling', 'storing', 'settling')
   AND (job.lease_expires_at IS NULL OR job.lease_expires_at <= $1)
RETURNING `+imageGenerationJobReturningColumns, now, leaseUntil))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrImageGenerationJobNotFound, nil)
	}
	return job, nil
}

func (r *imageGenerationJobRepository) RenewImageGenerationJobLease(ctx context.Context, jobID string, claimVersion int64, leaseUntil time.Time) error {
	result, err := r.sql.ExecContext(ctx, `
UPDATE image_generation_jobs
SET lease_expires_at = $3, updated_at = NOW()
WHERE job_id = $1
  AND claim_version = $2
  AND status IN ('created', 'submitting', 'submitted', 'polling', 'storing', 'settling')`, jobID, claimVersion, leaseUntil)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrImageGenerationClaimLost
	}
	return nil
}

func (r *imageGenerationJobRepository) QueueImageGenerationJob(ctx context.Context, jobID string, claimVersion int64, heldCost float64, queuedAt time.Time) error {
	if heldCost < 0 {
		heldCost = 0
	}
	return r.execFencedImageGenerationUpdate(ctx, `
UPDATE image_generation_jobs
SET status = 'queued',
    held_cost = $3,
    next_attempt_at = $4,
    lease_expires_at = NULL,
    error_code = NULL,
    error_message = NULL,
    updated_at = $4
WHERE job_id = $1
  AND claim_version = $2
  AND status = 'created'`, jobID, claimVersion, heldCost, queuedAt)
}

func (r *imageGenerationJobRepository) MarkImageGenerationJobSubmitted(ctx context.Context, jobID string, claimVersion, accountID int64, upstreamModel, upstreamTaskID string, submittedAt time.Time) error {
	if accountID <= 0 || strings.TrimSpace(upstreamTaskID) == "" {
		return service.ErrImageGenerationClaimLost
	}
	return r.execFencedImageGenerationUpdate(ctx, `
UPDATE image_generation_jobs
SET status = 'submitted',
    account_id = $3,
    upstream_model = $4,
    upstream_task_id = $5,
    submitted_at = COALESCE(submitted_at, $6),
    next_attempt_at = $6,
    lease_expires_at = NULL,
    error_code = NULL,
    error_message = NULL,
    updated_at = $6
WHERE job_id = $1
  AND claim_version = $2
  AND status = 'submitting'`, jobID, claimVersion, accountID, upstreamModel, strings.TrimSpace(upstreamTaskID), submittedAt)
}

func (r *imageGenerationJobRepository) MarkImageGenerationJobStoringFromSubmission(ctx context.Context, jobID string, claimVersion, accountID int64, upstreamModel, actualSize string, submittedAt time.Time) error {
	if accountID <= 0 || strings.TrimSpace(upstreamModel) == "" {
		return service.ErrImageGenerationClaimLost
	}
	return r.execFencedImageGenerationUpdate(ctx, `
UPDATE image_generation_jobs
SET status = 'storing',
    account_id = $3,
    upstream_model = $4,
    actual_size = NULLIF($5, ''),
    submitted_at = COALESCE(submitted_at, $6),
    next_attempt_at = NULL,
    error_code = NULL,
    error_message = NULL,
    updated_at = $6
WHERE job_id = $1
  AND claim_version = $2
  AND status = 'submitting'`, jobID, claimVersion, accountID, strings.TrimSpace(upstreamModel), strings.TrimSpace(actualSize), submittedAt)
}

func (r *imageGenerationJobRepository) ScheduleImageGenerationJobPoll(ctx context.Context, jobID string, claimVersion int64, nextAttemptAt time.Time) error {
	return r.execFencedImageGenerationUpdate(ctx, `
UPDATE image_generation_jobs
SET status = 'polling',
    next_attempt_at = $3,
    lease_expires_at = NULL,
    updated_at = NOW()
WHERE job_id = $1
  AND claim_version = $2
  AND status IN ('submitted', 'polling')
  AND account_id IS NOT NULL
  AND upstream_task_id IS NOT NULL`, jobID, claimVersion, nextAttemptAt)
}

func (r *imageGenerationJobRepository) MarkImageGenerationJobStoring(ctx context.Context, jobID string, claimVersion int64, actualSize string) error {
	return r.execFencedImageGenerationUpdate(ctx, `
UPDATE image_generation_jobs
SET status = 'storing',
    actual_size = NULLIF($3, ''),
    next_attempt_at = NULL,
    updated_at = NOW()
WHERE job_id = $1
  AND claim_version = $2
  AND status IN ('submitted', 'polling')`, jobID, claimVersion, strings.TrimSpace(actualSize))
}

func (r *imageGenerationJobRepository) MarkImageGenerationJobSettling(ctx context.Context, jobID string, claimVersion int64, resultObjectRefs []string, actualSize string, settledAt time.Time) error {
	if resultObjectRefs == nil {
		resultObjectRefs = []string{}
	}
	encoded, err := json.Marshal(resultObjectRefs)
	if err != nil {
		return err
	}
	return r.execFencedImageGenerationUpdate(ctx, `
UPDATE image_generation_jobs
SET status = 'settling',
    result_object_refs = $3::jsonb,
    actual_size = COALESCE(NULLIF($4, ''), actual_size),
    next_attempt_at = NULL,
    updated_at = $5
WHERE job_id = $1
  AND claim_version = $2
  AND status = 'storing'`, jobID, claimVersion, encoded, strings.TrimSpace(actualSize), settledAt)
}

func (r *imageGenerationJobRepository) MarkImageGenerationJobCompleted(ctx context.Context, jobID string, claimVersion int64, settledCost float64, completedAt time.Time) error {
	return r.execFencedImageGenerationUpdate(ctx, `
UPDATE image_generation_jobs
SET status = 'completed',
    settled_cost = $3,
    completed_at = COALESCE(completed_at, $4),
    settled_at = COALESCE(settled_at, $4),
    lease_expires_at = NULL,
    next_attempt_at = NULL,
    error_code = NULL,
    error_message = NULL,
    updated_at = $4
WHERE job_id = $1
  AND claim_version = $2
  AND status = 'settling'`, jobID, claimVersion, settledCost, completedAt)
}

func (r *imageGenerationJobRepository) MarkImageGenerationJobFailed(ctx context.Context, jobID string, claimVersion int64, code, message string, completedAt time.Time) error {
	return r.markImageGenerationJobTerminal(ctx, jobID, claimVersion, service.ImageGenerationJobStatusFailed, code, message, completedAt)
}

func (r *imageGenerationJobRepository) MarkImageGenerationJobSubmissionUnknown(ctx context.Context, jobID string, claimVersion int64, code, message string, completedAt time.Time) error {
	return r.markImageGenerationJobTerminal(ctx, jobID, claimVersion, service.ImageGenerationJobStatusSubmissionUnknown, code, message, completedAt)
}

func (r *imageGenerationJobRepository) markImageGenerationJobTerminal(ctx context.Context, jobID string, claimVersion int64, status, code, message string, completedAt time.Time) error {
	if status != service.ImageGenerationJobStatusFailed && status != service.ImageGenerationJobStatusSubmissionUnknown {
		return service.ErrImageGenerationClaimLost
	}
	return r.execFencedImageGenerationUpdate(ctx, `
UPDATE image_generation_jobs
SET status = $3,
    error_code = NULLIF($4, ''),
    error_message = NULLIF($5, ''),
    completed_at = COALESCE(completed_at, $6),
    lease_expires_at = NULL,
    next_attempt_at = NULL,
    updated_at = $6
WHERE job_id = $1
  AND claim_version = $2
  AND status NOT IN ('completed', 'failed', 'submission_unknown')`, jobID, claimVersion, status, service.RedactImageGenerationErrorMessage(code, 128), service.RedactImageGenerationErrorMessage(message, 1024), completedAt)
}

func (r *imageGenerationJobRepository) ReleaseImageGenerationJobForRetry(ctx context.Context, jobID string, claimVersion int64, status, code, message string, nextAttemptAt time.Time) error {
	switch status {
	case service.ImageGenerationJobStatusCreated,
		service.ImageGenerationJobStatusQueued,
		service.ImageGenerationJobStatusPolling,
		service.ImageGenerationJobStatusStoring,
		service.ImageGenerationJobStatusSettling:
	default:
		return service.ErrImageGenerationClaimLost
	}
	return r.execFencedImageGenerationUpdate(ctx, `
UPDATE image_generation_jobs
SET status = $3,
    error_code = NULLIF($4, ''),
    error_message = NULLIF($5, ''),
    next_attempt_at = $6,
    lease_expires_at = NULL,
    updated_at = NOW()
WHERE job_id = $1
  AND claim_version = $2
  AND status NOT IN ('completed', 'failed', 'submission_unknown')`, jobID, claimVersion, status, service.RedactImageGenerationErrorMessage(code, 128), service.RedactImageGenerationErrorMessage(message, 1024), nextAttemptAt)
}

func (r *imageGenerationJobRepository) RecoverExpiredImageGenerationJobLeases(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	result, err := r.sql.ExecContext(ctx, `
WITH expired AS (
    SELECT id
    FROM image_generation_jobs
    WHERE lease_expires_at IS NOT NULL
      AND lease_expires_at <= $1
      AND status IN ('created', 'submitting', 'submitted', 'polling', 'storing', 'settling')
    ORDER BY lease_expires_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT $2
)
UPDATE image_generation_jobs AS job
SET status = CASE
        WHEN job.status = 'submitting' THEN 'submission_unknown'
        WHEN job.status = 'submitted' THEN 'polling'
        ELSE job.status
    END,
    error_code = CASE
        WHEN job.status = 'submitting' THEN 'image_submission_unknown'
        ELSE job.error_code
    END,
    error_message = CASE
        WHEN job.status = 'submitting' THEN 'worker lease expired while upstream submission outcome was unknown'
        ELSE job.error_message
    END,
    completed_at = CASE
        WHEN job.status = 'submitting' THEN COALESCE(job.completed_at, $1)
        ELSE job.completed_at
    END,
    next_attempt_at = CASE
        WHEN job.status = 'submitting' THEN NULL
        ELSE $1
    END,
    lease_expires_at = NULL,
    updated_at = $1
FROM expired
WHERE job.id = expired.id`, now, limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *imageGenerationJobRepository) ListImageGenerationJobsForCleanup(ctx context.Context, before time.Time, limit int) ([]*service.ImageGenerationJob, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.sql.QueryContext(ctx, imageGenerationJobSelectSQL+`
 WHERE status IN ('completed', 'failed', 'submission_unknown')
   AND COALESCE(completed_at, updated_at) < $1
 ORDER BY COALESCE(completed_at, updated_at), id
 LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	jobs := make([]*service.ImageGenerationJob, 0, limit)
	for rows.Next() {
		job, scanErr := scanImageGenerationJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *imageGenerationJobRepository) DeleteImageGenerationJob(ctx context.Context, jobID string) error {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return service.ErrImageGenerationJobNotFound
	}
	result, err := r.sql.ExecContext(ctx, `
DELETE FROM image_generation_jobs
WHERE job_id = $1
  AND status IN ('completed', 'failed', 'submission_unknown')`, jobID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrImageGenerationJobNotFound
	}
	return nil
}

func (r *imageGenerationJobRepository) execFencedImageGenerationUpdate(ctx context.Context, query string, args ...any) error {
	result, err := r.sql.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrImageGenerationClaimLost
	}
	return nil
}

func imageGenerationIdempotencyScope(params service.CreateImageGenerationJobParams) string {
	return fmt.Sprintf("%d:%d:%s:%s", pointerInt64Value(params.UserID), pointerInt64Value(params.APIKeyID), params.Source, strings.TrimSpace(*params.IdempotencyKey))
}

func pointerInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func nullableStringsEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func getImageGenerationJobByIdempotency(ctx context.Context, executor imageGenerationJobSQLExecutor, params service.CreateImageGenerationJobParams) (*service.ImageGenerationJob, error) {
	return scanImageGenerationJob(executor.QueryRowContext(ctx, imageGenerationJobSelectSQL+`
 WHERE COALESCE(user_id, 0) = $1
   AND COALESCE(api_key_id, 0) = $2
   AND source = $3
   AND idempotency_key = $4
 ORDER BY id DESC
 LIMIT 1`, pointerInt64Value(params.UserID), pointerInt64Value(params.APIKeyID), params.Source, strings.TrimSpace(*params.IdempotencyKey)))
}

func insertImageGenerationJob(ctx context.Context, executor imageGenerationJobSQLExecutor, params service.CreateImageGenerationJobParams) (*service.ImageGenerationJob, error) {
	return scanImageGenerationJob(executor.QueryRowContext(ctx, `
INSERT INTO image_generation_jobs (
    job_id, user_id, api_key_id, group_id, subscription_id, billing_type, source, operation, status,
    public_model, requested_size, quality, response_format,
    idempotency_key, request_hash, prompt_hash, payload_object_ref,
    base_cost, rate_multiplier, estimated_cost, held_cost
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9,
    $10, $11, $12, $13,
    $14, $15, $16, $17,
    $18, $19, $20, $21
)
RETURNING `+imageGenerationJobReturningColumns,
		params.JobID, params.UserID, params.APIKeyID, params.GroupID, params.SubscriptionID, params.BillingType,
		params.Source, params.Operation, params.Status, params.PublicModel,
		params.RequestedSize, params.Quality, params.ResponseFormat,
		params.IdempotencyKey, params.RequestHash, params.PromptHash,
		params.PayloadObjectRef, params.BaseCost, params.RateMultiplier, params.EstimatedCost, params.HeldCost,
	))
}

const imageGenerationJobReturningColumns = `
    id, job_id, user_id, api_key_id, group_id, subscription_id, account_id, billing_type,
    source, operation, status, public_model, upstream_model,
    requested_size, actual_size, quality, response_format,
    upstream_task_id, idempotency_key, request_hash, prompt_hash,
    payload_object_ref, result_object_refs,
    base_cost, rate_multiplier, estimated_cost, held_cost, settled_cost,
    error_code, error_message, attempt_count, claim_version,
    lease_expires_at, next_attempt_at,
    created_at, updated_at, submitted_at, completed_at, settled_at`

const imageGenerationJobSelectSQL = `SELECT ` + imageGenerationJobReturningColumns + ` FROM image_generation_jobs`

type imageGenerationJobScanner interface {
	Scan(dest ...any) error
}

func scanImageGenerationJob(scanner imageGenerationJobScanner) (*service.ImageGenerationJob, error) {
	job := &service.ImageGenerationJob{}
	var resultRefs []byte
	err := scanner.Scan(
		&job.ID, &job.JobID, &job.UserID, &job.APIKeyID, &job.GroupID, &job.SubscriptionID, &job.AccountID, &job.BillingType,
		&job.Source, &job.Operation, &job.Status, &job.PublicModel, &job.UpstreamModel,
		&job.RequestedSize, &job.ActualSize, &job.Quality, &job.ResponseFormat,
		&job.UpstreamTaskID, &job.IdempotencyKey, &job.RequestHash, &job.PromptHash,
		&job.PayloadObjectRef, &resultRefs,
		&job.BaseCost, &job.RateMultiplier, &job.EstimatedCost, &job.HeldCost, &job.SettledCost,
		&job.ErrorCode, &job.ErrorMessage, &job.AttemptCount, &job.ClaimVersion,
		&job.LeaseExpiresAt, &job.NextAttemptAt,
		&job.CreatedAt, &job.UpdatedAt, &job.SubmittedAt, &job.CompletedAt, &job.SettledAt,
	)
	if err != nil {
		return nil, err
	}
	if len(resultRefs) > 0 {
		if err := json.Unmarshal(resultRefs, &job.ResultObjectRefs); err != nil {
			return nil, err
		}
	}
	if job.ResultObjectRefs == nil {
		job.ResultObjectRefs = []string{}
	}
	return job, nil
}

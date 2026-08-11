package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	imageGenerationBearerPattern   = regexp.MustCompile(`(?i)bearer\s+[^\s,;]+`)
	imageGenerationAPIKeyPattern   = regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{8,}\b`)
	imageGenerationURLQueryPattern = regexp.MustCompile(`(https?://[^\s?]+)\?[^\s]+`)
)

const (
	ImageGenerationJobSourceAPI       = "api"
	ImageGenerationJobSourceCodex     = "codex"
	ImageGenerationJobSourceWorkbench = "workbench"
	ImageGenerationJobSourceAdminTest = "admin_test"
)

const (
	ImageGenerationJobOperationGeneration = "generation"
	ImageGenerationJobOperationEdit       = "edit"
)

const (
	ImageGenerationJobStatusCreated           = "created"
	ImageGenerationJobStatusPlanning          = "planning"
	ImageGenerationJobStatusQueued            = "queued"
	ImageGenerationJobStatusSubmitting        = "submitting"
	ImageGenerationJobStatusSubmitted         = "submitted"
	ImageGenerationJobStatusPolling           = "polling"
	ImageGenerationJobStatusStoring           = "storing"
	ImageGenerationJobStatusSettling          = "settling"
	ImageGenerationJobStatusCompleted         = "completed"
	ImageGenerationJobStatusFailed            = "failed"
	ImageGenerationJobStatusSubmissionUnknown = "submission_unknown"
)

var (
	ErrImageGenerationJobNotFound = infraerrors.New(http.StatusNotFound, "IMAGE_TASK_NOT_FOUND", "image task not found")
	ErrImageGenerationJobExists   = infraerrors.New(http.StatusConflict, "IMAGE_TASK_EXISTS", "image task already exists")
	ErrImageGenerationIdempotency = infraerrors.New(http.StatusConflict, "IDEMPOTENCY_CONFLICT", "idempotency key was reused with different image parameters")
	ErrImageGenerationClaimLost   = infraerrors.New(http.StatusConflict, "IMAGE_TASK_CLAIM_LOST", "image task worker claim is no longer current")
)

type ImageGenerationJob struct {
	ID               int64
	JobID            string
	UserID           *int64
	APIKeyID         *int64
	GroupID          *int64
	SubscriptionID   *int64
	AccountID        *int64
	BillingType      int8
	Source           string
	Operation        string
	Status           string
	PublicModel      string
	DisplayName      *string
	UpstreamModel    *string
	RequestedSize    *string
	ActualSize       *string
	Quality          *string
	ResponseFormat   *string
	UpstreamTaskID   *string
	IdempotencyKey   *string
	RequestHash      *string
	PromptHash       string
	PayloadObjectRef *string
	ResultObjectRefs []string
	BaseCost         float64
	RateMultiplier   float64
	EstimatedCost    float64
	HeldCost         float64
	SettledCost      float64
	ErrorCode        *string
	ErrorMessage     *string
	AttemptCount     int
	ClaimVersion     int64
	LeaseExpiresAt   *time.Time
	NextAttemptAt    *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	SubmittedAt      *time.Time
	CompletedAt      *time.Time
	SettledAt        *time.Time
}

type CreateImageGenerationJobParams struct {
	JobID            string
	UserID           *int64
	APIKeyID         *int64
	GroupID          *int64
	SubscriptionID   *int64
	BillingType      int8
	Source           string
	Operation        string
	Status           string
	PublicModel      string
	DisplayName      *string
	RequestedSize    *string
	Quality          *string
	ResponseFormat   *string
	IdempotencyKey   *string
	RequestHash      *string
	PromptHash       string
	PayloadObjectRef *string
	BaseCost         float64
	RateMultiplier   float64
	EstimatedCost    float64
	HeldCost         float64
}

type ImageGenerationJobFilter struct {
	Source    string
	Status    string
	Operation string
	Limit     int
	Offset    int
}

// ImageGenerationJobRecovery describes a durable lease recovery transition.
// The worker runtime uses the job ID to release any process-external
// execution slot when recovery makes an unknown submission terminal.
type ImageGenerationJobRecovery struct {
	JobID  string
	Status string
}

type ImageGenerationJobRepository interface {
	CreateImageGenerationJob(ctx context.Context, params CreateImageGenerationJobParams) (job *ImageGenerationJob, replayed bool, err error)
	GetImageGenerationJob(ctx context.Context, jobID string) (*ImageGenerationJob, error)
	GetImageGenerationJobForUser(ctx context.Context, userID int64, jobID string) (*ImageGenerationJob, error)
	GetImageGenerationJobForOwner(ctx context.Context, userID, apiKeyID int64, jobID string) (*ImageGenerationJob, error)
	ListImageGenerationJobsForOwner(ctx context.Context, userID int64, filter ImageGenerationJobFilter) ([]*ImageGenerationJob, error)
	RenameImageGenerationJobForUser(ctx context.Context, userID int64, jobID, displayName string) (*ImageGenerationJob, error)
	DeleteImageGenerationJobForUser(ctx context.Context, userID int64, jobID, source string) error
	ClaimNextImageGenerationJob(ctx context.Context, now time.Time, leaseDuration time.Duration) (*ImageGenerationJob, error)
	RenewImageGenerationJobLease(ctx context.Context, jobID string, claimVersion int64, leaseUntil time.Time) error
	QueueImageGenerationJob(ctx context.Context, jobID string, claimVersion int64, heldCost float64, queuedAt time.Time) error
	MarkImageGenerationJobSubmitted(ctx context.Context, jobID string, claimVersion, accountID int64, upstreamModel, upstreamTaskID string, submittedAt time.Time) error
	MarkImageGenerationJobStoringFromSubmission(ctx context.Context, jobID string, claimVersion, accountID int64, upstreamModel, actualSize string, submittedAt time.Time) error
	ScheduleImageGenerationJobPoll(ctx context.Context, jobID string, claimVersion int64, nextAttemptAt time.Time) error
	MarkImageGenerationJobStoring(ctx context.Context, jobID string, claimVersion int64, actualSize string) error
	MarkImageGenerationJobSettling(ctx context.Context, jobID string, claimVersion int64, resultObjectRefs []string, actualSize string, settledAt time.Time) error
	MarkImageGenerationJobCompleted(ctx context.Context, jobID string, claimVersion int64, settledCost float64, completedAt time.Time) error
	MarkImageGenerationJobFailed(ctx context.Context, jobID string, claimVersion int64, code, message string, completedAt time.Time) error
	MarkImageGenerationJobSubmissionUnknown(ctx context.Context, jobID string, claimVersion int64, code, message string, completedAt time.Time) error
	ReleaseImageGenerationJobForRetry(ctx context.Context, jobID string, claimVersion int64, status, code, message string, nextAttemptAt time.Time) error
	RecoverExpiredImageGenerationJobLeases(ctx context.Context, now time.Time, limit int) ([]ImageGenerationJobRecovery, error)
	ListImageGenerationJobsForCleanup(ctx context.Context, before time.Time, limit int) ([]*ImageGenerationJob, error)
	DeleteImageGenerationJob(ctx context.Context, jobID string) error
}

func NewImageGenerationJobID() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "imgjob_" + hex.EncodeToString(random), nil
}

func RedactImageGenerationErrorMessage(value string, limit int) string {
	value = imageGenerationBearerPattern.ReplaceAllString(value, "Bearer [redacted]")
	value = imageGenerationAPIKeyPattern.ReplaceAllString(value, "[redacted-api-key]")
	value = imageGenerationURLQueryPattern.ReplaceAllString(value, "$1?[redacted]")
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if limit > 0 && len(value) > limit {
		value = value[:limit]
	}
	return value
}

//go:build image_generation_integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// This opt-in suite targets a caller-supplied, disposable PostgreSQL database.
// It intentionally does not use the repository's production containers or
// mutate any configured Sub2API server.
func TestImageGenerationJobRepositoryPostgresClaimIdempotencyAndFence(t *testing.T) {
	dsn := os.Getenv("IMAGE_GENERATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set IMAGE_GENERATION_TEST_DATABASE_URL to a disposable PostgreSQL DSN")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.PingContext(ctx))
	require.NoError(t, ApplyMigrations(ctx, db))
	const testJobPrefix = "imgjob_ext_it_"
	_, err = db.ExecContext(ctx, "DELETE FROM image_generation_jobs WHERE job_id LIKE $1", testJobPrefix+"%")
	require.NoError(t, err)

	repoA := NewImageGenerationJobRepository(db)
	repoB := NewImageGenerationJobRepository(db)
	requestHash := "external-it-request-hash"
	idempotencyKey := fmt.Sprintf("external-it-%d", time.Now().UnixNano())
	params := service.CreateImageGenerationJobParams{
		JobID:          fmt.Sprintf(testJobPrefix+"%d", time.Now().UnixNano()),
		Source:         service.ImageGenerationJobSourceWorkbench,
		Operation:      service.ImageGenerationJobOperationGeneration,
		Status:         service.ImageGenerationJobStatusQueued,
		PublicModel:    service.CangyuanImageModel1K,
		IdempotencyKey: &idempotencyKey,
		RequestHash:    &requestHash,
		PromptHash:     "external-it-prompt-hash",
		EstimatedCost:  0.025,
		RateMultiplier: 1,
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM image_generation_jobs WHERE job_id LIKE $1", testJobPrefix+"%")
	}()

	// Two independent repository instances must converge on one idempotent row.
	start := make(chan struct{})
	type createResult struct {
		job      *service.ImageGenerationJob
		replayed bool
		err      error
	}
	results := make(chan createResult, 2)
	var wg sync.WaitGroup
	for _, repo := range []service.ImageGenerationJobRepository{repoA, repoB} {
		wg.Add(1)
		go func(repo service.ImageGenerationJobRepository) {
			defer wg.Done()
			<-start
			job, replayed, createErr := repo.CreateImageGenerationJob(ctx, params)
			results <- createResult{job: job, replayed: replayed, err: createErr}
		}(repo)
	}
	close(start)
	wg.Wait()
	close(results)

	var created []*service.ImageGenerationJob
	replayedCount := 0
	for result := range results {
		require.NoError(t, result.err)
		require.NotNil(t, result.job)
		created = append(created, result.job)
		if result.replayed {
			replayedCount++
		}
	}
	require.Len(t, created, 2)
	require.Equal(t, created[0].JobID, created[1].JobID)
	require.Equal(t, 1, replayedCount)

	// Two workers racing on the same row must produce exactly one claim.
	claimNow := time.Now().UTC()
	type claimResult struct {
		job *service.ImageGenerationJob
		err error
	}
	claimResults := make(chan claimResult, 2)
	for _, repo := range []service.ImageGenerationJobRepository{repoA, repoB} {
		wg.Add(1)
		go func(repo service.ImageGenerationJobRepository) {
			defer wg.Done()
			job, claimErr := repo.ClaimNextImageGenerationJob(ctx, claimNow, time.Minute)
			claimResults <- claimResult{job: job, err: claimErr}
		}(repo)
	}
	wg.Wait()
	close(claimResults)

	var claimed *service.ImageGenerationJob
	claimCount := 0
	claimErrorCount := 0
	for result := range claimResults {
		if result.err != nil && result.job == nil {
			// The losing worker is expected to observe an empty candidate row.
			claimErrorCount++
			continue
		}
		require.NoError(t, result.err)
		if result.job != nil {
			claimCount++
			claimed = result.job
		}
	}
	require.Equal(t, 1, claimCount)
	require.Equal(t, 1, claimErrorCount)
	require.NotNil(t, claimed)
	require.Equal(t, service.ImageGenerationJobStatusSubmitting, claimed.Status)
	require.Equal(t, int64(1), claimed.ClaimVersion)

	// Let the submission lease expire. The recovery path must make the
	// ambiguous submission terminal and an old worker must be fenced out.
	recovered, err := repoA.RecoverExpiredImageGenerationJobLeases(ctx, claimNow.Add(2*time.Minute), 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), recovered)
	current, err := repoB.GetImageGenerationJob(ctx, params.JobID)
	require.NoError(t, err)
	require.Equal(t, service.ImageGenerationJobStatusSubmissionUnknown, current.Status)
	require.Equal(t, int64(1), current.ClaimVersion)
	require.ErrorIs(t, repoA.RenewImageGenerationJobLease(ctx, params.JobID, claimed.ClaimVersion, time.Now().Add(time.Minute)), service.ErrImageGenerationClaimLost)
}

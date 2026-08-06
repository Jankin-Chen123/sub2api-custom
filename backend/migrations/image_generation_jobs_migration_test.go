package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration194CreatesDurableImageGenerationJobs(t *testing.T) {
	content, err := FS.ReadFile("194_image_generation_jobs.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS image_generation_jobs")
	require.Contains(t, sql, "upstream_task_id")
	require.Contains(t, sql, "idempotency_key")
	require.Contains(t, sql, "claim_version BIGINT NOT NULL DEFAULT 0")
	require.Contains(t, sql, "lease_expires_at TIMESTAMPTZ")
	require.Contains(t, sql, "image_generation_jobs_claim_idx")
	require.Contains(t, sql, "image_generation_jobs_tenant_idempotency_uq")
	require.Contains(t, sql, "image_generation_jobs_lease_idx")
	require.Contains(t, sql, "image_generation_jobs_cleanup_idx")
	require.Contains(t, sql, "image_generation_jobs_source_ck")
	require.Contains(t, sql, "image_generation_jobs_operation_ck")
	require.Contains(t, sql, "image_generation_jobs_billing_type_ck")
	require.Contains(t, sql, "image_generation_jobs_cost_ck")
	require.Contains(t, sql, "ON DELETE SET NULL")
	require.Contains(t, sql, "submission_unknown")
	require.Contains(t, sql, "jsonb_typeof(result_object_refs) = 'array'")
}

func TestMigration194DoesNotPersistRawImageInputs(t *testing.T) {
	content, err := FS.ReadFile("194_image_generation_jobs.sql")
	require.NoError(t, err)
	lower := strings.ToLower(string(content))

	for _, forbidden := range []string{"raw_prompt", "authorization", "api_key_value", "b64_json"} {
		require.NotContains(t, lower, forbidden)
	}
	require.Contains(t, lower, "payload_object_ref")
	require.Contains(t, lower, "prompt_hash")
	require.Contains(t, lower, "comment on index image_generation_jobs_cleanup_idx")
}

func TestMigration195ClaimIndexIncludesCreatedJobs(t *testing.T) {
	content, err := FS.ReadFile("195_image_generation_jobs_claim_created.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "drop index if exists image_generation_jobs_claim_idx")
	require.Contains(t, sql, "create index if not exists image_generation_jobs_claim_idx")
	require.Contains(t, sql, "'created'")
	require.Contains(t, sql, "'settling'")
}

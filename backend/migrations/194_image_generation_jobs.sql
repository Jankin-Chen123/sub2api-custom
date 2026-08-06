-- Durable single-image generation/edit jobs used by dedicated image routing,
-- Codex orchestration, the OpenAI-compatible Images API, and the user workbench.
-- PostgreSQL is the task source of truth; Redis is only a wake-up/lock layer.

CREATE TABLE IF NOT EXISTS image_generation_jobs (
    id BIGSERIAL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    subscription_id BIGINT REFERENCES user_subscriptions(id) ON DELETE SET NULL,
    account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    billing_type SMALLINT NOT NULL DEFAULT 0,
    source VARCHAR(32) NOT NULL,
    operation VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'created',
    public_model VARCHAR(128) NOT NULL,
    upstream_model VARCHAR(128),
    requested_size VARCHAR(32),
    actual_size VARCHAR(32),
    quality VARCHAR(32),
    response_format VARCHAR(32),
    upstream_task_id VARCHAR(512),
    idempotency_key VARCHAR(255),
    request_hash VARCHAR(128),
    prompt_hash VARCHAR(128) NOT NULL,
    payload_object_ref VARCHAR(1024),
    result_object_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    base_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    rate_multiplier DECIMAL(20,10) NOT NULL DEFAULT 1,
    estimated_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    held_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    settled_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    error_code VARCHAR(128),
    error_message TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    claim_version BIGINT NOT NULL DEFAULT 0,
    lease_expires_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    settled_at TIMESTAMPTZ,

    CONSTRAINT image_generation_jobs_job_id_uq UNIQUE (job_id),
    CONSTRAINT image_generation_jobs_source_ck CHECK (source IN ('api', 'codex', 'workbench', 'admin_test')),
    CONSTRAINT image_generation_jobs_operation_ck CHECK (operation IN ('generation', 'edit')),
    CONSTRAINT image_generation_jobs_billing_type_ck CHECK (billing_type IN (0, 1)),
    CONSTRAINT image_generation_jobs_status_ck CHECK (status IN (
        'created', 'planning', 'queued', 'submitting', 'submitted', 'polling',
        'storing', 'settling', 'completed', 'failed', 'submission_unknown'
    )),
    CONSTRAINT image_generation_jobs_cost_ck CHECK (
        base_cost >= 0 AND rate_multiplier >= 0 AND estimated_cost >= 0 AND held_cost >= 0 AND settled_cost >= 0
    ),
    CONSTRAINT image_generation_jobs_attempt_ck CHECK (attempt_count >= 0),
    CONSTRAINT image_generation_jobs_claim_version_ck CHECK (claim_version >= 0),
    CONSTRAINT image_generation_jobs_result_refs_array_ck CHECK (jsonb_typeof(result_object_refs) = 'array')
);

CREATE UNIQUE INDEX IF NOT EXISTS image_generation_jobs_tenant_idempotency_uq
    ON image_generation_jobs (
        COALESCE(user_id, 0),
        COALESCE(api_key_id, 0),
        source,
        idempotency_key
    )
    WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';

CREATE INDEX IF NOT EXISTS image_generation_jobs_claim_idx
    ON image_generation_jobs (status, next_attempt_at, created_at)
    WHERE status IN ('queued', 'submitted', 'polling', 'storing', 'settling');

CREATE INDEX IF NOT EXISTS image_generation_jobs_lease_idx
    ON image_generation_jobs (lease_expires_at)
    WHERE lease_expires_at IS NOT NULL
      AND status IN ('submitting', 'submitted', 'polling', 'storing', 'settling');

CREATE INDEX IF NOT EXISTS image_generation_jobs_user_created_idx
    ON image_generation_jobs (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS image_generation_jobs_api_key_created_idx
    ON image_generation_jobs (api_key_id, created_at DESC);

CREATE INDEX IF NOT EXISTS image_generation_jobs_account_status_idx
    ON image_generation_jobs (account_id, status)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS image_generation_jobs_upstream_task_idx
    ON image_generation_jobs (account_id, upstream_task_id)
    WHERE upstream_task_id IS NOT NULL AND upstream_task_id <> '';

CREATE INDEX IF NOT EXISTS image_generation_jobs_cleanup_idx
    ON image_generation_jobs (completed_at)
    WHERE status IN ('completed', 'failed', 'submission_unknown');

COMMENT ON TABLE image_generation_jobs IS 'Durable single-image jobs for API, Codex, workbench, and dedicated Cangyuan routing';
COMMENT ON COLUMN image_generation_jobs.upstream_task_id IS 'Private upstream task identifier; never returned by public APIs';
COMMENT ON COLUMN image_generation_jobs.payload_object_ref IS 'Reference to encrypted/TTL-controlled task input; raw prompt and image bytes are not stored in this table';
COMMENT ON COLUMN image_generation_jobs.claim_version IS 'Monotonic fencing token incremented on every worker claim';
COMMENT ON COLUMN image_generation_jobs.error_message IS 'Length-limited, redacted error summary; never store raw upstream bodies or credentials';
COMMENT ON INDEX image_generation_jobs_cleanup_idx IS 'Terminal jobs are deleted only after the configured retention window and result-object cleanup succeeds';

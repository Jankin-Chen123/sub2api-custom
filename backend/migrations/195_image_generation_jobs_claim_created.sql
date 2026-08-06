-- Include the initial created state in the durable worker claim index.
-- Migration 194 predates the created -> queued preparation pass, so an
-- existing database needs the predicate rebuilt instead of mutating the old
-- migration checksum.

DROP INDEX IF EXISTS image_generation_jobs_claim_idx;

CREATE INDEX IF NOT EXISTS image_generation_jobs_claim_idx
    ON image_generation_jobs (status, next_attempt_at, created_at)
    WHERE status IN ('created', 'queued', 'submitted', 'polling', 'storing', 'settling');

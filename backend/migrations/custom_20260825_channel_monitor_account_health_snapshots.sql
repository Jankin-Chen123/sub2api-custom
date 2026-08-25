-- Latest dynamic health for one local group/account/provider/model tuple.
-- This is intentionally independent from accounts.priority and from the
-- per-run account probe observation table.
CREATE TABLE IF NOT EXISTS channel_monitor_account_health_snapshots (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    score DOUBLE PRECISION NOT NULL DEFAULT 50 CHECK (score >= 0 AND score <= 100),
    health_state TEXT NOT NULL CHECK (health_state IN ('unknown', 'healthy', 'degraded', 'unhealthy')),
    ewma_success_rate DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (ewma_success_rate >= 0 AND ewma_success_rate <= 1),
    ewma_latency_ms INTEGER,
    sample_count INTEGER NOT NULL DEFAULT 0 CHECK (sample_count >= 0),
    consecutive_successes INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_successes >= 0),
    consecutive_failures INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
    last_status TEXT NOT NULL DEFAULT '',
    last_probe_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_channel_monitor_account_health_snapshot
    ON channel_monitor_account_health_snapshots (group_id, account_id, provider, model);

CREATE INDEX IF NOT EXISTS idx_channel_monitor_account_health_lookup
    ON channel_monitor_account_health_snapshots (group_id, provider, model, updated_at DESC);

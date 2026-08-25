-- Persist account-level V1 monitor observations separately from the
-- channel-level history used by the existing frontend trend views.
CREATE TABLE IF NOT EXISTS channel_monitor_account_probe_results (
    id BIGSERIAL PRIMARY KEY,
    monitor_id BIGINT NOT NULL REFERENCES channel_monitors(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('operational', 'degraded', 'failed', 'error', 'skipped')),
    latency_ms INTEGER,
    checked_at TIMESTAMPTZ NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    skipped BOOLEAN NOT NULL DEFAULT FALSE,
    skip_reason TEXT NOT NULL DEFAULT '',
    round_duration_ms INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_channel_monitor_account_probe_results_lookup
    ON channel_monitor_account_probe_results (monitor_id, model, checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_channel_monitor_account_probe_results_account
    ON channel_monitor_account_probe_results (group_id, account_id, model, checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_channel_monitor_account_probe_results_retention
    ON channel_monitor_account_probe_results (checked_at, id);

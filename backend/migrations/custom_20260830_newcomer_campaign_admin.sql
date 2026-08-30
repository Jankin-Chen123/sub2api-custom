-- 2026-09 newcomer campaign: administrator-controlled tier settings and
-- manually assigned memberships. This is a new migration; the original
-- campaign migration is intentionally left unchanged.

CREATE TABLE IF NOT EXISTS newcomer_campaign_tier_configs (
    campaign_key VARCHAR(64) NOT NULL,
    tier_key VARCHAR(16) NOT NULL CHECK (tier_key IN ('premium', 'gold', 'diamond')),
    tier_name VARCHAR(32) NOT NULL,
    threshold INTEGER NOT NULL CHECK (threshold > 0),
    factor DECIMAL(10,6) NOT NULL CHECK (factor > 0 AND factor <= 1),
    duration_days INTEGER NOT NULL CHECK (duration_days > 0),
    updated_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_key, tier_key),
    CONSTRAINT newcomer_campaign_tier_configs_threshold_unique
        UNIQUE (campaign_key, threshold)
);

INSERT INTO newcomer_campaign_tier_configs
    (campaign_key, tier_key, tier_name, threshold, factor, duration_days)
VALUES
    ('newcomer_202609', 'premium', '高级', 2, 0.98, 30),
    ('newcomer_202609', 'gold', '黄金', 5, 0.96, 45),
    ('newcomer_202609', 'diamond', '钻石', 10, 0.94, 60)
ON CONFLICT (campaign_key, tier_key) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_newcomer_campaign_tier_configs_threshold
    ON newcomer_campaign_tier_configs (campaign_key, threshold);

COMMENT ON TABLE newcomer_campaign_tier_configs IS 'Current administrator-controlled configuration for newcomer campaign membership tiers';

CREATE TABLE IF NOT EXISTS newcomer_campaign_admin_memberships (
    id BIGSERIAL PRIMARY KEY,
    campaign_key VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier_key VARCHAR(16) NOT NULL CHECK (tier_key IN ('premium', 'gold', 'diamond')),
    factor DECIMAL(10,6) NOT NULL CHECK (factor > 0 AND factor <= 1),
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at > starts_at),
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'revoked')),
    granted_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    revoked_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    reason VARCHAR(255) NOT NULL DEFAULT '',
    revoked_at TIMESTAMPTZ NULL,
    revoke_reason VARCHAR(255) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS newcomer_campaign_admin_memberships_active_user
    ON newcomer_campaign_admin_memberships (campaign_key, user_id)
    WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_newcomer_campaign_admin_memberships_current
    ON newcomer_campaign_admin_memberships (campaign_key, user_id, status, expires_at DESC);

COMMENT ON TABLE newcomer_campaign_admin_memberships IS 'Auditable administrator grants independent from automatic invitation membership grants';

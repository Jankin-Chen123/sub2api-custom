-- 2026-09 newcomer campaign: independent, auditable state.
-- All timestamps are timestamptz values and are written as UTC by the service.

CREATE TABLE IF NOT EXISTS newcomer_campaign_payment_facts (
    order_id BIGINT PRIMARY KEY REFERENCES payment_orders(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    principal_amount DECIMAL(20,8) NOT NULL CHECK (principal_amount > 0),
    principal_currency VARCHAR(3) NOT NULL DEFAULT 'CNY'
        CHECK (principal_currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_newcomer_payment_facts_user_created
    ON newcomer_campaign_payment_facts (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_newcomer_payment_facts_currency
    ON newcomer_campaign_payment_facts (principal_currency, created_at DESC);

COMMENT ON TABLE newcomer_campaign_payment_facts IS 'Original online balance recharge principal captured at order creation';

CREATE TABLE IF NOT EXISTS newcomer_campaign_reward_ledger (
    id BIGSERIAL PRIMARY KEY,
    campaign_key VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_order_id BIGINT NULL REFERENCES payment_orders(id) ON DELETE SET NULL,
    reward_type VARCHAR(64) NOT NULL,
    entry_type VARCHAR(16) NOT NULL CHECK (entry_type IN ('grant', 'revoke')),
    amount DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reversed_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_newcomer_reward_ledger_user
    ON newcomer_campaign_reward_ledger (campaign_key, user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_newcomer_reward_ledger_order
    ON newcomer_campaign_reward_ledger (campaign_key, source_order_id)
    WHERE source_order_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS newcomer_campaign_invites (
    id BIGSERIAL PRIMARY KEY,
    campaign_key VARCHAR(64) NOT NULL,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invite_code VARCHAR(32) NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL,
    qualification_deadline TIMESTAMPTZ NOT NULL,
    qualifying_amount DECIMAL(20,8) NOT NULL DEFAULT 0 CHECK (qualifying_amount >= 0),
    qualifying_order_id BIGINT NULL REFERENCES payment_orders(id) ON DELETE SET NULL,
    qualifying_redeem_code_id BIGINT NULL REFERENCES redeem_codes(id) ON DELETE SET NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'qualified', 'revoked', 'expired')),
    qualified_at TIMESTAMPTZ NULL,
    revoked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT newcomer_campaign_invites_invitee_unique UNIQUE (campaign_key, invitee_id)
);

CREATE INDEX IF NOT EXISTS idx_newcomer_campaign_invites_inviter
    ON newcomer_campaign_invites (campaign_key, inviter_id, status);
CREATE INDEX IF NOT EXISTS idx_newcomer_campaign_invites_deadline
    ON newcomer_campaign_invites (campaign_key, qualification_deadline, status);
CREATE INDEX IF NOT EXISTS idx_newcomer_campaign_invites_qualifying_redeem
    ON newcomer_campaign_invites (qualifying_redeem_code_id)
    WHERE qualifying_redeem_code_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS newcomer_campaign_membership_grants (
    id BIGSERIAL PRIMARY KEY,
    campaign_key VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier_key VARCHAR(16) NOT NULL CHECK (tier_key IN ('premium', 'gold', 'diamond')),
    threshold INTEGER NOT NULL CHECK (threshold > 0),
    factor DECIMAL(10,6) NOT NULL CHECK (factor > 0 AND factor <= 1),
    duration_days INTEGER NOT NULL CHECK (duration_days > 0),
    granted_at TIMESTAMPTZ NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'revoked')),
    revoked_at TIMESTAMPTZ NULL,
    revoke_reason VARCHAR(255) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT newcomer_campaign_membership_tier_unique UNIQUE (campaign_key, user_id, tier_key)
);

CREATE INDEX IF NOT EXISTS idx_newcomer_campaign_membership_current
    ON newcomer_campaign_membership_grants (campaign_key, user_id, status, expires_at DESC, threshold DESC);

COMMENT ON TABLE newcomer_campaign_reward_ledger IS '2026-09 newcomer campaign reward grants and reversals';
COMMENT ON TABLE newcomer_campaign_invites IS '2026-09 newcomer campaign invitation qualification records';
COMMENT ON TABLE newcomer_campaign_membership_grants IS '2026-09 newcomer campaign membership grant history';

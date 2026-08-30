-- 2026-09 newcomer campaign hardening.
--
-- A refunded first-recharge reward must not make a user's balance negative.
-- The amount which cannot be recovered immediately is retained as a durable
-- debt.  The balance trigger below observes every SQL update of users.balance
-- (including updates made by older code paths), and consumes future positive
-- credits before they become spendable.  No total_recharged column is touched.
--
-- This is deliberately a new migration.  The original campaign migrations
-- are already applied in production and are immutable.

CREATE TABLE IF NOT EXISTS newcomer_campaign_clawback_debts (
    id BIGSERIAL PRIMARY KEY,
    campaign_key VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_order_id BIGINT NULL REFERENCES payment_orders(id) ON DELETE SET NULL,
    reward_type VARCHAR(64) NOT NULL,
    due_amount DECIMAL(20,8) NOT NULL CHECK (due_amount > 0),
    recovered_amount DECIMAL(20,8) NOT NULL DEFAULT 0
        CHECK (recovered_amount >= 0 AND recovered_amount <= due_amount),
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'settled')),
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_newcomer_clawback_debts_pending
    ON newcomer_campaign_clawback_debts (user_id, status, id);

CREATE TABLE IF NOT EXISTS newcomer_campaign_clawback_allocations (
    id BIGSERIAL PRIMARY KEY,
    debt_id BIGINT NOT NULL REFERENCES newcomer_campaign_clawback_debts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    source_type VARCHAR(64) NOT NULL,
    source_id VARCHAR(180) NOT NULL,
    idempotency_key VARCHAR(240) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_newcomer_clawback_allocations_debt
    ON newcomer_campaign_clawback_allocations (debt_id, created_at);
CREATE INDEX IF NOT EXISTS idx_newcomer_clawback_allocations_user
    ON newcomer_campaign_clawback_allocations (user_id, created_at DESC);

-- A sequence, rather than txid_current(), identifies each balance update. A
-- transaction can legitimately credit the same user more than once; using
-- only its transaction ID would make the second update hit the allocation's
-- idempotency key and bypass the remaining debt.
CREATE SEQUENCE IF NOT EXISTS newcomer_campaign_clawback_credit_seq;

COMMENT ON TABLE newcomer_campaign_clawback_debts IS
    'Auditable first-recharge reward amounts still owed after a refund';
COMMENT ON TABLE newcomer_campaign_clawback_allocations IS
    'Auditable balance credits applied to newcomer campaign clawback debts';

-- A caller may provide a business source for better audit detail.  Legacy
-- callers need no changes: txid_current() gives every fallback allocation a
-- stable transaction-local source.  A campaign reward explicitly opts out so
-- a reward grant can never repay a debt created by revoking that same reward.
CREATE OR REPLACE FUNCTION newcomer_campaign_consume_clawback_on_balance_credit()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    credit NUMERIC;
    remaining NUMERIC;
    allocation NUMERIC;
    debt RECORD;
    source_type TEXT;
    source_id TEXT;
    credit_nonce BIGINT;
BEGIN
    credit := NEW.balance - OLD.balance;
    IF credit <= 0 THEN
        RETURN NEW;
    END IF;

    -- The first-recharge reward is a campaign liability, not user money.  It
    -- is intentionally excluded from repaying its own refund clawback.
    IF current_setting('sub2api.balance_credit_kind', true) = 'campaign_reward' THEN
        RETURN NEW;
    END IF;

    source_type := COALESCE(NULLIF(current_setting('sub2api.balance_credit_source_type', true), ''), 'users_balance_update');
    credit_nonce := nextval('newcomer_campaign_clawback_credit_seq');
    source_id := format('%s#%s',
        COALESCE(NULLIF(current_setting('sub2api.balance_credit_source_id', true), ''), txid_current()::TEXT),
        credit_nonce);
    remaining := credit;

    -- FOR UPDATE serializes allocations for a user.  The deterministic id
    -- order makes concurrent credits consume debt in the same order.
    FOR debt IN
        SELECT id, due_amount, recovered_amount
        FROM newcomer_campaign_clawback_debts
        WHERE user_id = NEW.id
          AND status = 'pending'
          AND recovered_amount < due_amount
        ORDER BY id
        FOR UPDATE
    LOOP
        EXIT WHEN remaining <= 0;
        allocation := LEAST(remaining, debt.due_amount - debt.recovered_amount);
        IF allocation <= 0 THEN
            CONTINUE;
        END IF;

        INSERT INTO newcomer_campaign_clawback_allocations
            (debt_id, user_id, amount, source_type, source_id, idempotency_key)
        VALUES
            (debt.id, NEW.id, allocation, source_type, source_id,
             format('newcomer-clawback:%s:%s:%s', debt.id, source_type, source_id))
        ON CONFLICT (idempotency_key) DO NOTHING;

        IF FOUND THEN
            UPDATE newcomer_campaign_clawback_debts
            SET recovered_amount = recovered_amount + allocation,
                status = CASE
                    WHEN recovered_amount + allocation >= due_amount THEN 'settled'
                    ELSE 'pending'
                END,
                settled_at = CASE
                    WHEN recovered_amount + allocation >= due_amount THEN NOW()
                    ELSE settled_at
                END,
                updated_at = NOW()
            WHERE id = debt.id;
            remaining := remaining - allocation;
        END IF;
    END LOOP;

    -- Keep the user's balance at its pre-credit value for the portion used to
    -- settle debt.  This makes the operation safe even when the balance was
    -- already negative due to an unrelated usage overdraft.
    IF remaining < credit THEN
        NEW.balance := OLD.balance + remaining;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS newcomer_campaign_clawback_balance_credit ON users;
CREATE TRIGGER newcomer_campaign_clawback_balance_credit
BEFORE UPDATE OF balance ON users
FOR EACH ROW
EXECUTE FUNCTION newcomer_campaign_consume_clawback_on_balance_credit();

-- Stable activity-owned referral facts.  The ordinary affiliate profile can
-- be disabled, repaired, or removed after registration; campaign binding is
-- therefore resolved from this immutable code-to-inviter mapping instead of
-- guessing an inviter from an arbitrary current profile.
CREATE TABLE IF NOT EXISTS newcomer_campaign_invite_codes (
    campaign_key VARCHAR(64) NOT NULL,
    invite_code VARCHAR(32) NOT NULL,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source VARCHAR(32) NOT NULL DEFAULT 'campaign',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_key, invite_code)
);

CREATE INDEX IF NOT EXISTS idx_newcomer_campaign_invite_codes_inviter
    ON newcomer_campaign_invite_codes (campaign_key, inviter_id);

CREATE TABLE IF NOT EXISTS newcomer_campaign_referral_intents (
    campaign_key VARCHAR(64) NOT NULL,
    invitee_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invite_code VARCHAR(32) NOT NULL,
    signup_source VARCHAR(32) NOT NULL DEFAULT 'unknown',
    inviter_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'bound', 'invalid')),
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bound_at TIMESTAMPTZ NULL,
    PRIMARY KEY (campaign_key, invitee_id)
);

CREATE INDEX IF NOT EXISTS idx_newcomer_campaign_referral_intents_pending
    ON newcomer_campaign_referral_intents (campaign_key, status, updated_at);

-- Turn pre-hardening affiliate codes into activity-owned facts once.  Future
-- registration and campaign-code issuance paths write the mapping directly.
INSERT INTO newcomer_campaign_invite_codes (campaign_key, invite_code, inviter_id, source)
SELECT 'newcomer_202609', ua.aff_code, ua.user_id, 'affiliate_backfill'
FROM user_affiliates ua
WHERE ua.aff_code <> ''
ON CONFLICT (campaign_key, invite_code) DO NOTHING;

-- Long-lived configuration for the single newcomer campaign key.
-- The campaign can be reopened by changing this row; invitation history and
-- already-issued rewards remain under newcomer_202609 forever.

CREATE TABLE IF NOT EXISTS newcomer_campaign_config (
    campaign_key VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    updated_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT newcomer_campaign_config_window_valid CHECK (ends_at > starts_at)
);

INSERT INTO newcomer_campaign_config
    (campaign_key, name, starts_at, ends_at)
VALUES
    ('newcomer_202609', '迎新活动',
     TIMESTAMPTZ '2026-09-01 00:00:00+08',
     TIMESTAMPTZ '2026-10-01 00:00:00+08')
ON CONFLICT (campaign_key) DO NOTHING;

-- A user's activity eligibility is an event-time snapshot. This prevents a
-- later reopening/edit of the public window from losing an old user's still
-- valid 14-day qualification period.
CREATE TABLE IF NOT EXISTS newcomer_campaign_eligible_users (
    campaign_key VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    registered_at TIMESTAMPTZ NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    capture_deadline TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_key, user_id),
    CONSTRAINT newcomer_campaign_eligible_users_window_valid CHECK (window_end > window_start),
    CONSTRAINT newcomer_campaign_eligible_users_capture_valid CHECK (capture_deadline >= window_end)
);

CREATE INDEX IF NOT EXISTS idx_newcomer_campaign_eligible_users_registered
    ON newcomer_campaign_eligible_users (campaign_key, registered_at);

-- Preserve the registration-time eligibility snapshot on referral intents as
-- well. This is intentionally redundant with eligible_users: it keeps a
-- pending intent auditable even if the user eligibility row is repaired later.
ALTER TABLE newcomer_campaign_referral_intents
    ADD COLUMN IF NOT EXISTS eligible_at_registration BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE newcomer_campaign_referral_intents
    ADD COLUMN IF NOT EXISTS registration_window_start TIMESTAMPTZ NULL;
ALTER TABLE newcomer_campaign_referral_intents
    ADD COLUMN IF NOT EXISTS registration_window_end TIMESTAMPTZ NULL;
ALTER TABLE newcomer_campaign_referral_intents
    ADD COLUMN IF NOT EXISTS qualification_capture_deadline TIMESTAMPTZ NULL;

-- The original fixed September window is the first snapshot for users that
-- predate this migration. No historical invitation/reward rows are rewritten.
INSERT INTO newcomer_campaign_eligible_users
    (campaign_key, user_id, registered_at, window_start, window_end, capture_deadline)
SELECT 'newcomer_202609', u.id, u.created_at,
       TIMESTAMPTZ '2026-09-01 00:00:00+08',
       TIMESTAMPTZ '2026-10-01 00:00:00+08',
       TIMESTAMPTZ '2026-10-15 00:00:00+08'
FROM users u
WHERE u.created_at >= TIMESTAMPTZ '2026-09-01 00:00:00+08'
  AND u.created_at < TIMESTAMPTZ '2026-10-01 00:00:00+08'
ON CONFLICT (campaign_key, user_id) DO NOTHING;

UPDATE newcomer_campaign_referral_intents r
SET eligible_at_registration = TRUE,
    registration_window_start = e.window_start,
    registration_window_end = e.window_end,
    qualification_capture_deadline = e.capture_deadline,
    updated_at = NOW()
FROM newcomer_campaign_eligible_users e
WHERE r.campaign_key = e.campaign_key
  AND r.invitee_id = e.user_id
  AND NOT r.eligible_at_registration;

COMMENT ON TABLE newcomer_campaign_config IS
    'Long-lived configuration for the single newcomer campaign key; history is never reset';
COMMENT ON TABLE newcomer_campaign_eligible_users IS
    'Registration-time newcomer campaign eligibility and capture-window snapshots';

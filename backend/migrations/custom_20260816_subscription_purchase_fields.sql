-- Subscription purchase cards need their own validity and quota snapshots.
-- NULL quota values continue to fall back to the referenced group's limits.

ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS daily_limit_usd DECIMAL(20, 8),
    ADD COLUMN IF NOT EXISTS weekly_limit_usd DECIMAL(20, 8),
    ADD COLUMN IF NOT EXISTS monthly_limit_usd DECIMAL(20, 8);

ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS validity_days INT NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS daily_limit_usd DECIMAL(20, 8),
    ADD COLUMN IF NOT EXISTS weekly_limit_usd DECIMAL(20, 8),
    ADD COLUMN IF NOT EXISTS monthly_limit_usd DECIMAL(20, 8);

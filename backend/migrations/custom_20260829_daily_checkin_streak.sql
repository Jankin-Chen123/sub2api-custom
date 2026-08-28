-- Consecutive daily check-in state and the configurable seven-day bonus.
ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS bonus_amount DECIMAL(20,8) NOT NULL DEFAULT 0 CHECK (bonus_amount >= 0),
    ADD COLUMN IF NOT EXISTS streak_days INTEGER NOT NULL DEFAULT 1 CHECK (streak_days > 0);

INSERT INTO settings (key, value, updated_at)
VALUES ('daily_checkin_streak_bonus_amount', '5.00000000', NOW())
ON CONFLICT (key) DO NOTHING;

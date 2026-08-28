-- Daily lucky-wheel check-in prizes and auditable reward records.
CREATE TABLE IF NOT EXISTS daily_checkin_prizes (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    amount DECIMAL(20,8) NOT NULL CHECK (amount >= 0),
    probability DECIMAL(12,8) NOT NULL CHECK (probability > 0 AND probability <= 100),
    color VARCHAR(7) NOT NULL DEFAULT '#6366F1',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS daily_checkins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date DATE NOT NULL,
    prize_id BIGINT REFERENCES daily_checkin_prizes(id) ON DELETE SET NULL,
    prize_name VARCHAR(80) NOT NULL,
    amount DECIMAL(20,8) NOT NULL CHECK (amount >= 0),
    probability DECIMAL(12,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT daily_checkins_user_date_unique UNIQUE (user_id, checkin_date)
);

CREATE INDEX IF NOT EXISTS idx_daily_checkins_user_created
    ON daily_checkins (user_id, created_at DESC);

-- A usable default wheel. Administrators can atomically replace it in the panel.
INSERT INTO daily_checkin_prizes (name, amount, probability, color, sort_order)
SELECT seed.name, seed.amount, seed.probability, seed.color, seed.sort_order
FROM (VALUES
    ('幸运 $0.01', 0.01, 35.0, '#60A5FA', 0),
    ('幸运 $0.02', 0.02, 25.0, '#34D399', 1),
    ('幸运 $0.05', 0.05, 18.0, '#FBBF24', 2),
    ('幸运 $0.10', 0.10, 10.0, '#FB7185', 3),
    ('幸运 $0.20', 0.20, 6.0, '#A78BFA', 4),
    ('幸运 $0.50', 0.50, 3.0, '#2DD4BF', 5),
    ('幸运 $1.00', 1.00, 2.0, '#F97316', 6),
    ('幸运 $5.00', 5.00, 1.0, '#EC4899', 7)
) AS seed(name, amount, probability, color, sort_order)
WHERE NOT EXISTS (SELECT 1 FROM daily_checkin_prizes);

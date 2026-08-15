-- Positive balance redeem codes are reviewed by an administrator before
-- they create affiliate rebate quota. This prevents operator-issued gifts
-- from being treated as paid recharges.
ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS affiliate_rebate_status VARCHAR(20) NOT NULL DEFAULT 'not_applicable';

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS affiliate_rebate_amount DECIMAL(20,8) NULL;

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS affiliate_rebate_reviewed_at TIMESTAMPTZ NULL;

UPDATE redeem_codes
SET affiliate_rebate_status = 'pending'
WHERE status = 'used'
  AND type = 'balance'
  AND value > 0
  AND affiliate_rebate_status = 'not_applicable';

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_redeem_code_id BIGINT NULL REFERENCES redeem_codes(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_affiliate_ledger_redeem_code_uniq
    ON user_affiliate_ledger(source_redeem_code_id)
    WHERE action = 'accrue' AND source_redeem_code_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_affiliate_review
    ON redeem_codes(affiliate_rebate_status, used_at)
    WHERE status = 'used' AND type = 'balance' AND value > 0;

-- The previous implementation accrued redeem-code rebates without storing
-- the redeem-code ID. Link only unambiguous historical matches; ambiguous
-- rows remain pending for manual review instead of risking a duplicate payout.
WITH candidates AS (
    SELECT rc.id AS redeem_code_id,
           ual.id AS ledger_id,
           COUNT(*) OVER (PARTITION BY rc.id) AS code_matches,
           COUNT(*) OVER (PARTITION BY ual.id) AS ledger_matches
    FROM redeem_codes rc
    JOIN user_affiliates invitee_aff ON invitee_aff.user_id = rc.used_by
    JOIN user_affiliate_ledger ual
      ON ual.action = 'accrue'
     AND ual.source_redeem_code_id IS NULL
     AND ual.source_order_id IS NULL
     AND ual.source_user_id = rc.used_by
     AND ual.user_id = invitee_aff.inviter_id
     AND ual.amount > 0
     AND rc.used_at IS NOT NULL
     AND ual.created_at BETWEEN rc.used_at - INTERVAL '10 minutes'
                            AND rc.used_at + INTERVAL '10 minutes'
    WHERE rc.status = 'used'
      AND rc.type = 'balance'
      AND rc.value > 0
      AND rc.affiliate_rebate_status = 'pending'
), matched AS (
    SELECT redeem_code_id, ledger_id
    FROM candidates
    WHERE code_matches = 1 AND ledger_matches = 1
)
UPDATE user_affiliate_ledger ual
SET source_redeem_code_id = matched.redeem_code_id,
    updated_at = NOW()
FROM matched
WHERE ual.id = matched.ledger_id;

UPDATE redeem_codes rc
SET affiliate_rebate_status = 'approved',
    affiliate_rebate_amount = ual.amount,
    affiliate_rebate_reviewed_at = COALESCE(rc.affiliate_rebate_reviewed_at, ual.created_at)
FROM user_affiliate_ledger ual
WHERE ual.source_redeem_code_id = rc.id
  AND ual.action = 'accrue'
  AND rc.affiliate_rebate_status = 'pending';

-- Encrypted, short-lived image inputs and staged results.
-- The application encrypts ciphertext before writing it; this table must never
-- be treated as a source of plaintext prompts, images, or provider secrets.
CREATE TABLE IF NOT EXISTS image_generation_payloads (
    payload_ref TEXT PRIMARY KEY,
    ciphertext BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- AES-GCM output is base64 encoded, so it is larger than the 128 MiB
    -- plaintext admission limit enforced by the application.
    CONSTRAINT image_generation_payloads_ciphertext_ck
        CHECK (octet_length(ciphertext) > 0 AND octet_length(ciphertext) <= 268435456)
);

CREATE INDEX IF NOT EXISTS image_generation_payloads_expiry_idx
    ON image_generation_payloads (expires_at);

COMMENT ON TABLE image_generation_payloads IS
    'Encrypted temporary image-generation payloads; application ciphertext only.';

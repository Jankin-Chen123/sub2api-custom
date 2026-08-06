package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	imageGenerationPayloadKeyPrefix  = "image_generation_payload:"
	imageGenerationPayloadMaxBytes   = 128 << 20
	imageGenerationPayloadDefaultTTL = 6 * time.Hour
)

type imageGenerationPayloadStore struct {
	db        *sql.DB
	rdb       *redis.Client
	encryptor service.SecretEncryptor
}

func NewImageGenerationPayloadStore(rdb *redis.Client, encryptor service.SecretEncryptor) service.ImageGenerationPayloadStore {
	return &imageGenerationPayloadStore{rdb: rdb, encryptor: encryptor}
}

// NewDurableImageGenerationPayloadStore stores new image payloads in
// PostgreSQL. Redis remains an optional read-only fallback for jobs written by
// versions that stored payloads there, so a rolling upgrade does not strand an
// in-flight task. New writes never depend on Redis being available.
func NewDurableImageGenerationPayloadStore(db *sql.DB, rdb *redis.Client, encryptor service.SecretEncryptor) service.ImageGenerationPayloadStore {
	return &imageGenerationPayloadStore{db: db, rdb: rdb, encryptor: encryptor}
}

func (s *imageGenerationPayloadStore) Save(ctx context.Context, ref string, payload *service.ImageGenerationPayload, ttl time.Duration) error {
	if s == nil || (s.db == nil && s.rdb == nil) || s.encryptor == nil {
		return errors.New("encrypted image generation payload store is not configured")
	}
	if strings.TrimSpace(ref) == "" || payload == nil {
		return errors.New("image generation payload reference and value are required")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode image generation payload: %w", err)
	}
	if len(raw) > imageGenerationPayloadMaxBytes {
		return fmt.Errorf("image generation payload exceeds %d bytes", imageGenerationPayloadMaxBytes)
	}
	ciphertext, err := s.encryptor.Encrypt(string(raw))
	if err != nil {
		return fmt.Errorf("encrypt image generation payload: %w", err)
	}
	if ttl <= 0 {
		ttl = imageGenerationPayloadDefaultTTL
	}
	if s.db != nil {
		expiresAt := time.Now().Add(ttl)
		_, err := s.db.ExecContext(ctx, `
INSERT INTO image_generation_payloads (payload_ref, ciphertext, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (payload_ref) DO UPDATE SET
    ciphertext = EXCLUDED.ciphertext,
    expires_at = EXCLUDED.expires_at,
    updated_at = NOW()`, strings.TrimSpace(ref), []byte(ciphertext), expiresAt)
		return err
	}
	if s.rdb == nil {
		return errors.New("encrypted image generation payload store is not configured")
	}
	return s.rdb.Set(ctx, imageGenerationPayloadKey(ref), ciphertext, ttl).Err()
}

func (s *imageGenerationPayloadStore) Get(ctx context.Context, ref string) (*service.ImageGenerationPayload, error) {
	if s == nil || (s.db == nil && s.rdb == nil) || s.encryptor == nil {
		return nil, errors.New("encrypted image generation payload store is not configured")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, service.ErrImageGenerationPayloadNotFound
	}
	var ciphertext []byte
	if s.db != nil {
		err := s.db.QueryRowContext(ctx, `
SELECT ciphertext
FROM image_generation_payloads
WHERE payload_ref = $1 AND expires_at > NOW()`, ref).Scan(&ciphertext)
		if err == nil {
			return decodeImageGenerationPayload(s.encryptor, string(ciphertext))
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		// Read-only compatibility for payloads created before migration 196.
		// Once all old jobs expire, this branch is no longer used.
	}
	if s.rdb == nil {
		return nil, service.ErrImageGenerationPayloadNotFound
	}
	legacyCiphertext, err := s.rdb.Get(ctx, imageGenerationPayloadKey(ref)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, service.ErrImageGenerationPayloadNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeImageGenerationPayload(s.encryptor, legacyCiphertext)
}

func decodeImageGenerationPayload(encryptor service.SecretEncryptor, ciphertext string) (*service.ImageGenerationPayload, error) {
	if encryptor == nil {
		return nil, errors.New("encrypted image generation payload store is not configured")
	}
	plaintext, err := encryptor.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt image generation payload: %w", err)
	}
	if len(plaintext) > imageGenerationPayloadMaxBytes {
		return nil, errors.New("decrypted image generation payload exceeds the configured limit")
	}
	var payload service.ImageGenerationPayload
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		return nil, fmt.Errorf("decode image generation payload: %w", err)
	}
	return &payload, nil
}

func (s *imageGenerationPayloadStore) Delete(ctx context.Context, ref string) error {
	if s == nil {
		return nil
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if s.db != nil {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM image_generation_payloads WHERE payload_ref = $1`, ref); err != nil {
			return err
		}
		// Legacy Redis deletion is best effort. It must not make a durable
		// PostgreSQL payload undeletable during a Redis outage.
		if s.rdb != nil {
			_ = s.rdb.Del(ctx, imageGenerationPayloadKey(ref)).Err()
		}
		return nil
	}
	if s.rdb == nil {
		return nil
	}
	return s.rdb.Del(ctx, imageGenerationPayloadKey(ref)).Err()
}

func (s *imageGenerationPayloadStore) PurgeExpired(ctx context.Context, before time.Time, limit int) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	result, err := s.db.ExecContext(ctx, `
DELETE FROM image_generation_payloads
WHERE payload_ref IN (
    SELECT payload_ref
    FROM image_generation_payloads
    WHERE expires_at <= $1
    ORDER BY expires_at, payload_ref
    LIMIT $2
)`, before, limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func imageGenerationPayloadKey(ref string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ref)))
	return imageGenerationPayloadKeyPrefix + hex.EncodeToString(sum[:])
}

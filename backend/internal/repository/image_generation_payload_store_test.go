package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestImageGenerationPayloadStoreEncryptsRoundTripAndExpires(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewImageGenerationPayloadStore(rdb, &AESEncryptor{key: bytes.Repeat([]byte{7}, 32)})
	ref := service.ImageGenerationPayloadRef("imgjob_payload")
	payload := &service.ImageGenerationPayload{
		Request:       service.CangyuanImageRequest{Model: service.CangyuanImageModel2K, Prompt: "private prompt marker"},
		PendingResult: &service.CangyuanImageResult{Completed: true, Data: []service.CangyuanImageData{{B64JSON: "private base64 marker"}}},
	}

	require.NoError(t, store.Save(context.Background(), ref, payload, time.Hour))
	key := imageGenerationPayloadKey(ref)
	raw, err := mr.Get(key)
	require.NoError(t, err)
	require.NotContains(t, raw, "private prompt marker")
	require.NotContains(t, raw, "private base64 marker")
	require.Equal(t, time.Hour, mr.TTL(key))

	got, err := store.Get(context.Background(), ref)
	require.NoError(t, err)
	require.Equal(t, payload, got)

	mr.FastForward(time.Hour + time.Second)
	_, err = store.Get(context.Background(), ref)
	require.ErrorIs(t, err, service.ErrImageGenerationPayloadNotFound)
}

func TestImageGenerationPayloadStoreRejectsCorruptCiphertextAndDeletes(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewImageGenerationPayloadStore(rdb, &AESEncryptor{key: bytes.Repeat([]byte{8}, 32)})
	ref := service.ImageGenerationPayloadRef("imgjob_corrupt")
	require.NoError(t, rdb.Set(context.Background(), imageGenerationPayloadKey(ref), "corrupt", time.Hour).Err())

	_, err := store.Get(context.Background(), ref)
	require.Error(t, err)
	require.False(t, errors.Is(err, service.ErrImageGenerationPayloadNotFound))
	require.NoError(t, store.Delete(context.Background(), ref))
	_, err = store.Get(context.Background(), ref)
	require.ErrorIs(t, err, service.ErrImageGenerationPayloadNotFound)
}

func TestDurableImageGenerationPayloadStoreSurvivesRedisLoss(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	encryptor := &AESEncryptor{key: bytes.Repeat([]byte{9}, 32)}
	store := NewDurableImageGenerationPayloadStore(db, rdb, encryptor)
	ref := service.ImageGenerationPayloadRef("imgjob_durable")
	payload := &service.ImageGenerationPayload{
		Request: service.CangyuanImageRequest{Model: service.CangyuanImageModel2K, Prompt: "durable private prompt"},
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO image_generation_payloads")).
		WithArgs(ref, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, store.Save(context.Background(), ref, payload, time.Hour))

	// A Redis outage after the durable write must not affect a worker restart.
	mr.Close()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	storedCiphertext, err := encryptor.Encrypt(string(raw))
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ciphertext")).
		WithArgs(ref).
		WillReturnRows(sqlmock.NewRows([]string{"ciphertext"}).AddRow([]byte(storedCiphertext)))
	got, err := store.Get(context.Background(), ref)
	require.NoError(t, err)
	require.Equal(t, payload, got)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM image_generation_payloads")).
		WithArgs(ref).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, store.Delete(context.Background(), ref))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDurableImageGenerationPayloadStoreReadsLegacyRedisPayloadWhenRowMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	encryptor := &AESEncryptor{key: bytes.Repeat([]byte{10}, 32)}
	store := NewDurableImageGenerationPayloadStore(db, rdb, encryptor)
	ref := service.ImageGenerationPayloadRef("imgjob_legacy")
	payload := &service.ImageGenerationPayload{Request: service.CangyuanImageRequest{Prompt: "legacy"}}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	ciphertext, err := encryptor.Encrypt(string(raw))
	require.NoError(t, err)
	require.NoError(t, rdb.Set(context.Background(), imageGenerationPayloadKey(ref), ciphertext, time.Hour).Err())

	mock.ExpectQuery(regexp.QuoteMeta("SELECT ciphertext")).
		WithArgs(ref).
		WillReturnError(sql.ErrNoRows)
	got, err := store.Get(context.Background(), ref)
	require.NoError(t, err)
	require.Equal(t, payload, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDurableImageGenerationPayloadStorePurgesExpiredRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := NewDurableImageGenerationPayloadStore(db, nil, &AESEncryptor{key: bytes.Repeat([]byte{11}, 32)})
	before := time.Now().UTC()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM image_generation_payloads")).
		WithArgs(before, 25).
		WillReturnResult(sqlmock.NewResult(0, 3))
	cleaner, ok := store.(service.ImageGenerationPayloadExpiryCleaner)
	require.True(t, ok)
	deleted, err := cleaner.PurgeExpired(context.Background(), before, 25)
	require.NoError(t, err)
	require.Equal(t, int64(3), deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

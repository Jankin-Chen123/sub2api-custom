//go:build image_generation_integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// This opt-in test uses only a caller-supplied disposable PostgreSQL DSN. It
// intentionally constructs the durable store without Redis to prove that new
// image payloads do not depend on Redis availability.
func TestDurableImageGenerationPayloadStorePostgresSurvivesWithoutRedis(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("IMAGE_GENERATION_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set IMAGE_GENERATION_TEST_DATABASE_URL to a disposable PostgreSQL DSN")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.PingContext(ctx))
	require.NoError(t, ApplyMigrations(ctx, db))

	ref := fmt.Sprintf("image-generation/integration-%d", time.Now().UnixNano())
	_, _ = db.ExecContext(ctx, "DELETE FROM image_generation_payloads WHERE payload_ref = $1", ref)
	defer func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM image_generation_payloads WHERE payload_ref = $1", ref)
	}()

	encryptor := &AESEncryptor{key: []byte("0123456789abcdef0123456789abcdef")}
	store := NewDurableImageGenerationPayloadStore(db, nil, encryptor)
	payload := &service.ImageGenerationPayload{
		Request: service.CangyuanImageRequest{
			Model:  service.CangyuanImageModel2K,
			Prompt: "postgres durable payload integration marker",
		},
	}

	require.NoError(t, store.Save(ctx, ref, payload, time.Hour))
	var ciphertext []byte
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT ciphertext FROM image_generation_payloads WHERE payload_ref = $1", ref,
	).Scan(&ciphertext))
	require.NotContains(t, string(ciphertext), payload.Request.Prompt)

	got, err := store.Get(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, payload, got)

	// A fresh store instance with no Redis dependency represents a process
	// restart while Redis is unavailable.
	restartedStore := NewDurableImageGenerationPayloadStore(db, nil, encryptor)
	got, err = restartedStore.Get(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, payload, got)

	require.NoError(t, restartedStore.Delete(ctx, ref))
	_, err = restartedStore.Get(ctx, ref)
	require.ErrorIs(t, err, service.ErrImageGenerationPayloadNotFound)
}

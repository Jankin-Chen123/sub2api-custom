package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration196CreatesEncryptedImageGenerationPayloads(t *testing.T) {
	content, err := FS.ReadFile("196_image_generation_payloads.sql")
	require.NoError(t, err)
	sql := string(content)
	lower := strings.ToLower(sql)

	require.Contains(t, lower, "create table if not exists image_generation_payloads")
	require.Contains(t, lower, "payload_ref text primary key")
	require.Contains(t, lower, "ciphertext bytea not null")
	require.Contains(t, lower, "expires_at timestamptz not null")
	require.Contains(t, lower, "image_generation_payloads_expiry_idx")
	require.Contains(t, lower, "octet_length(ciphertext)")
	require.Contains(t, lower, "encrypted temporary image-generation payloads")
	require.NotContains(t, lower, "raw_prompt")
	require.NotContains(t, lower, "b64_json")
}

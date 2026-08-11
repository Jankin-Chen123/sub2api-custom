package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// These migrations were already applied in production before the upstream
// release introduced migrations with the same numeric prefixes. They are
// intentionally kept under their original names and checksums forever.
var legacyCustomMigrationChecksums = map[string]string{
	"194_image_generation_jobs.sql":               "c157bdf43c5212bbb1011464077129d07d2115d0dcf05921441d1b8f38452954",
	"195_image_generation_jobs_claim_created.sql": "8517b0039f7b8f8ef1cce9bffcc53f85cb3417a1b197767b2c9207bc894e4abc",
	"196_image_generation_payloads.sql":           "cb4c2c7a6f592ab039f496afe7620f6929ab4bbcea81e05efdff819da0d4c1d5",
	"197_image_generation_job_display_name.sql":   "449b5713f192d4ef05a8fc9f0be16b93dcdf50dc99ea30b89789d922da8ca518",
}

var customMigrationNamePattern = regexp.MustCompile(`^custom_[0-9]{8}_[a-z0-9]+(?:_[a-z0-9]+)*(?:_notx)?\.sql$`)

func TestLegacyCustomMigrationsRemainImmutable(t *testing.T) {
	for name, expectedChecksum := range legacyCustomMigrationChecksums {
		content, err := FS.ReadFile(name)
		require.NoError(t, err, "legacy custom migration %s must not be renamed or deleted", name)
		require.Equal(t, expectedChecksum, trimmedMigrationChecksum(content), "legacy custom migration %s changed after production", name)
	}
}

func Test194And195MigrationPrefixesAreExplicitlyAllowlisted(t *testing.T) {
	allowed := map[string]struct{}{
		"194_add_usage_log_upstream_response_model.sql":            {},
		"194_image_generation_jobs.sql":                            {},
		"195_add_usage_log_upstream_model_mismatch_index_notx.sql": {},
		"195_image_generation_jobs_claim_created.sql":              {},
	}

	entries, err := FS.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "194_") || strings.HasPrefix(name, "195_") {
			_, ok := allowed[name]
			require.True(t, ok, "new migration %s reuses a sensitive 194/195 prefix without explicit review", name)
		}
	}
}

func TestFutureCustomMigrationsUseIsolatedNamespace(t *testing.T) {
	entries, err := FS.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "custom_") {
			require.Regexp(t, customMigrationNamePattern, name,
				"future custom migrations must use custom_YYYYMMDD_description[_notx].sql")
		}
	}
}

func trimmedMigrationChecksum(content []byte) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(content))))
	return hex.EncodeToString(sum[:])
}

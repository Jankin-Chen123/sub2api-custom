package repository

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	migrationfs "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestSelectedReleaseMigrationSetPreservesCustomImageHistory(t *testing.T) {
	expectedChecksums := map[string]string{
		"194_image_generation_jobs.sql":               "c157bdf43c5212bbb1011464077129d07d2115d0dcf05921441d1b8f38452954",
		"195_image_generation_jobs_claim_created.sql": "8517b0039f7b8f8ef1cce9bffcc53f85cb3417a1b197767b2c9207bc894e4abc",
		"196_image_generation_payloads.sql":           "cb4c2c7a6f592ab039f496afe7620f6929ab4bbcea81e05efdff819da0d4c1d5",
		"197_image_generation_job_display_name.sql":   "449b5713f192d4ef05a8fc9f0be16b93dcdf50dc99ea30b89789d922da8ca518",
	}

	for name, expected := range expectedChecksums {
		content, err := migrationfs.FS.ReadFile(name)
		require.NoError(t, err, "read immutable custom migration %s", name)
		require.Equal(t, expected, migrationChecksum(string(content)), "custom migration %s must remain byte-for-byte compatible with production", name)
	}

	for _, name := range []string{
		"194_add_usage_log_upstream_response_model.sql",
		"195_add_usage_log_upstream_model_mismatch_index_notx.sql",
	} {
		_, err := migrationfs.FS.ReadFile(name)
		require.NoError(t, err, "required upstream migration %s is missing", name)
	}

	files, err := migrationfs.FS.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range files {
		name := entry.Name()
		require.NotContains(t, name, "channel_monitor_v2", "Channel Monitor V2 is deferred from this release")
		require.NotEqual(t, "195_channel_monitor_mode.sql", name, "Channel Monitor V2 mode migration is deferred from this release")
		require.NotEqual(t, "220_clear_non_grok_video_generation_config.sql", name, "destructive Grok pricing cleanup is deferred from this release")
	}
}

func TestApplyMigrationsFS_ProductionSnapshotWithCustom194Through197(t *testing.T) {
	const (
		officialColumnMigration = "194_add_usage_log_upstream_response_model.sql"
		officialIndexMigration  = "195_add_usage_log_upstream_model_mismatch_index_notx.sql"
	)
	customMigrations := []string{
		"194_image_generation_jobs.sql",
		"195_image_generation_jobs_claim_created.sql",
		"196_image_generation_payloads.sql",
		"197_image_generation_job_display_name.sql",
	}

	allNames := append([]string{officialColumnMigration, officialIndexMigration}, customMigrations...)
	contents := make(map[string]string, len(allNames))
	fsys := fstest.MapFS{}
	for _, name := range allNames {
		content, err := migrationfs.FS.ReadFile(name)
		require.NoError(t, err)
		contents[name] = strings.TrimSpace(string(content))
		fsys[name] = &fstest.MapFile{Data: content}
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)

	// Lexical order applies the new official 194 first, then observes the
	// production-applied custom 194 by its full filename and checksum.
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(officialColumnMigration).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(contents[officialColumnMigration])).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs(officialColumnMigration, migrationChecksum(contents[officialColumnMigration])).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	expectAppliedMigration(mock, customMigrations[0], contents[customMigrations[0]])

	// The official 195 index is non-transactional. The runner performs its
	// invalid-index precheck, builds it concurrently, and records it without
	// confusing it with the already-applied custom 195 filename.
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(officialIndexMigration).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs(usageLogsUpstreamModelMismatchIndex).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	indexStatement := strings.TrimSpace(splitSQLStatements(contents[officialIndexMigration])[0])
	mock.ExpectExec(regexp.QuoteMeta(indexStatement)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs(officialIndexMigration, migrationChecksum(contents[officialIndexMigration])).
		WillReturnResult(sqlmock.NewResult(1, 1))

	for _, name := range customMigrations[1:] {
		expectAppliedMigration(mock, name, contents[name])
	}
	mock.ExpectExec("SELECT pg_advisory_unlock\\(\\$1\\)").
		WithArgs(migrationsAdvisoryLockID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, applyMigrationsFS(context.Background(), db, fsys))
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectAppliedMigration(mock sqlmock.Sqlmock, name, content string) {
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(name).
		WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(migrationChecksum(content)))
}

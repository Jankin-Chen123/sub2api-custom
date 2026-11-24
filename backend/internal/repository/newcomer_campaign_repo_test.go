package repository

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newNewcomerCampaignRepoSQLMock(t *testing.T) (*ent.Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := ent.NewClient(ent.Driver(drv))
	t.Cleanup(func() {
		_ = client.Close()
		_ = db.Close()
	})
	return client, mock
}

func TestNewcomerCampaignRepository_NilClient(t *testing.T) {
	require.Nil(t, NewNewcomerCampaignRepository(nil))
}

func TestNewcomerCampaignRepository_QueryAndExec(t *testing.T) {
	client, mock := newNewcomerCampaignRepoSQLMock(t)
	repo := NewNewcomerCampaignRepository(client)
	require.NotNil(t, repo)

	ctx := context.Background()
	require.Same(t, repo, repo.ClientForContext(ctx))

	mock.ExpectQuery(`SELECT 1`).WillReturnRows(sqlmock.NewRows([]string{"val"}).AddRow(1))
	rows, err := repo.QueryContext(ctx, "SELECT 1")
	require.NoError(t, err)
	require.NoError(t, rows.Close())

	mock.ExpectExec(`UPDATE users SET balance = \$1`).WithArgs(5).WillReturnResult(sqlmock.NewResult(0, 1))
	res, err := repo.ExecContext(ctx, "UPDATE users SET balance = $1", 5)
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewcomerCampaignRepository_WithTxCommitAndRollback(t *testing.T) {
	client, mock := newNewcomerCampaignRepoSQLMock(t)
	repo := NewNewcomerCampaignRepository(client)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE newcomer_campaign_config`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.WithTx(ctx, func(txCtx context.Context, tx service.NewcomerCampaignSQLClient) error {
		require.NotNil(t, ent.TxFromContext(txCtx))
		require.NotSame(t, repo, repo.ClientForContext(txCtx))
		_, execErr := tx.ExecContext(txCtx, "UPDATE newcomer_campaign_config SET updated_at = NOW()")
		return execErr
	})
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectRollback()
	sentinel := errors.New("abort tx")
	err = repo.WithTx(ctx, func(context.Context, service.NewcomerCampaignSQLClient) error {
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
	require.NoError(t, mock.ExpectationsWereMet())
}

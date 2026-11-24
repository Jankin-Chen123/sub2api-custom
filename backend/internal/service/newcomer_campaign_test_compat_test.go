package service

import (
	"context"
	"database/sql"
	"errors"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

type entNewcomerCampaignTestRepo struct {
	client *dbent.Client
}

func (r *entNewcomerCampaignTestRepo) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if r == nil || r.client == nil {
		return nil, errors.New("newcomer campaign test client is nil")
	}
	return r.client.QueryContext(ctx, query, args...)
}

func (r *entNewcomerCampaignTestRepo) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if r == nil || r.client == nil {
		return nil, errors.New("newcomer campaign test client is nil")
	}
	return r.client.ExecContext(ctx, query, args...)
}

func (r *entNewcomerCampaignTestRepo) ClientForContext(ctx context.Context) NewcomerCampaignSQLClient {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r
}

func (r *entNewcomerCampaignTestRepo) WithTx(ctx context.Context, fn func(txCtx context.Context, tx NewcomerCampaignSQLClient) error) error {
	if r == nil || r.client == nil {
		return errors.New("newcomer campaign test client is nil")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// NewNewcomerCampaignService preserves the historical test constructor while
// keeping production code decoupled from *ent.Client.
func NewNewcomerCampaignService(entClient *dbent.Client, affiliateEnsurers ...newcomerAffiliateEnsurer) *NewcomerCampaignService {
	var repo NewcomerCampaignRepository
	if entClient != nil {
		repo = &entNewcomerCampaignTestRepo{client: entClient}
	}
	return NewNewcomerCampaignServiceWithRepository(repo, affiliateEnsurers...)
}

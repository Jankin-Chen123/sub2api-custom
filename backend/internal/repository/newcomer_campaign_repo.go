package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type newcomerCampaignRepository struct {
	client *ent.Client
}

// NewNewcomerCampaignRepository adapts *ent.Client into
// service.NewcomerCampaignRepository so newcomer campaign services avoid
// importing the ORM package directly.
func NewNewcomerCampaignRepository(client *ent.Client) service.NewcomerCampaignRepository {
	if client == nil {
		return nil
	}
	return &newcomerCampaignRepository{client: client}
}

func (r *newcomerCampaignRepository) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if r == nil || r.client == nil {
		return nil, errors.New("newcomer campaign repository client is nil")
	}
	return r.client.QueryContext(ctx, query, args...)
}

func (r *newcomerCampaignRepository) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if r == nil || r.client == nil {
		return nil, errors.New("newcomer campaign repository client is nil")
	}
	return r.client.ExecContext(ctx, query, args...)
}

func (r *newcomerCampaignRepository) ClientForContext(ctx context.Context) service.NewcomerCampaignSQLClient {
	if tx := ent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	if r == nil || r.client == nil {
		return nil
	}
	return r
}

func (r *newcomerCampaignRepository) WithTx(ctx context.Context, fn func(txCtx context.Context, tx service.NewcomerCampaignSQLClient) error) error {
	if r == nil || r.client == nil {
		return errors.New("newcomer campaign repository client is nil")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	txCtx := ent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

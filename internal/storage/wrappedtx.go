package storage

import (
	"context"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type WrappedTxCreator struct {
	pool *pgxpool.Pool
}

type WrappedTx struct {
	Tx     *pgx.Tx
	TxOpts *pgx.TxOptions
}

func NewWrappedTransactionCreator(pool *pgxpool.Pool) *WrappedTxCreator {
	return &WrappedTxCreator{
		pool: pool,
	}
}

func (wtc *WrappedTxCreator) NewTx(ctx context.Context, options pgx.TxOptions) (*WrappedTx, error) {
	tx, err := wtc.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &WrappedTx{
		Tx:     &tx,
		TxOpts: &options,
	}, nil
}

func (wt *WrappedTx) Commit(ctx context.Context) error {
	return (*wt.Tx).Commit(ctx)
}

func (wt *WrappedTx) Rollback(ctx context.Context) error {
	return (*wt.Tx).Rollback(ctx)
}

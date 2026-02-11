package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/util"
)

type Queries = db.Queries

type DB struct {
	Queries *Queries
	pool    *pgxpool.Pool
}

type Config struct {
	URL string
}

func New(ctx context.Context, config Config) (*DB, error) {
	pool, err := pgxpool.New(ctx, config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		pool:    pool,
		Queries: db.New(pool),
	}, nil
}

func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

func (d *DB) Close() {
	d.pool.Close()
}

type TxOptions struct {
	IsoLevel pgx.TxIsoLevel
	ReadOnly bool
}

func (d *DB) WithTx(ctx context.Context, fn func(q *Queries) error, opts ...TxOptions) error {
	defaultOpts := TxOptions{
		IsoLevel: pgx.ReadCommitted,
		ReadOnly: false,
	}
	if len(opts) > 0 {
		defaultOpts = opts[0]
	}

	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   defaultOpts.IsoLevel,
		AccessMode: util.Ternary(defaultOpts.ReadOnly, pgx.ReadOnly, pgx.ReadWrite),
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	txQueries := db.New(tx)
	if err := fn(txQueries); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("failed to rollback: %v (original error: %w)", rbErr, err)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

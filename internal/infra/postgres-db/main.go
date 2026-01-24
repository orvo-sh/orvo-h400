package postgresdb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/orvo-sh/orvo/internal/infra/postgres-db/db"
)

// DB wraps the pgxpool.Pool and provides access to SQLC queries
type DB struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// DBTX is the interface that both pgxpool.Pool and pgx.Tx implement
type DBTX interface {
	db.DBTX
}

// New creates a new PostgreSQL database connection pool
func New(ctx context.Context, connString string) (*DB, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		pool:    pool,
		queries: db.New(pool),
	}, nil
}

// Close closes the database connection pool
func (d *DB) Close() {
	d.pool.Close()
}

// Queries returns the SQLC queries instance
func (d *DB) Queries() *db.Queries {
	return d.queries
}

// Pool returns the underlying connection pool
func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

// TxOptions represents transaction options
type TxOptions struct {
	IsoLevel pgx.TxIsoLevel
	ReadOnly bool
}

// DefaultTxOptions returns default transaction options
func DefaultTxOptions() TxOptions {
	return TxOptions{
		IsoLevel: pgx.ReadCommitted,
		ReadOnly: false,
	}
}

// WithTx executes a function within a database transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func (d *DB) WithTx(ctx context.Context, fn func(q *db.Queries) error) error {
	return d.WithTxOptions(ctx, DefaultTxOptions(), fn)
}

// WithTxOptions executes a function within a database transaction with custom options
func (d *DB) WithTxOptions(ctx context.Context, opts TxOptions, fn func(q *db.Queries) error) error {
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: opts.IsoLevel,
		AccessMode: func() pgx.TxAccessMode {
			if opts.ReadOnly {
				return pgx.ReadOnly
			}
			return pgx.ReadWrite
		}(),
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // re-throw panic after rollback
		}
	}()

	// Create queries instance that uses the transaction
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

// WithTxResult executes a function within a database transaction and returns a result
// This is a generic version that allows returning values from the transaction
func WithTxResult[T any](ctx context.Context, d *DB, fn func(q *db.Queries) (T, error)) (T, error) {
	return WithTxResultOptions(ctx, d, DefaultTxOptions(), fn)
}

// WithTxResultOptions executes a function within a database transaction with custom options and returns a result
func WithTxResultOptions[T any](ctx context.Context, d *DB, opts TxOptions, fn func(q *db.Queries) (T, error)) (T, error) {
	var result T

	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: opts.IsoLevel,
		AccessMode: func() pgx.TxAccessMode {
			if opts.ReadOnly {
				return pgx.ReadOnly
			}
			return pgx.ReadWrite
		}(),
	})
	if err != nil {
		return result, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	txQueries := db.New(tx)

	result, err = fn(txQueries)
	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return result, fmt.Errorf("failed to rollback: %v (original error: %w)", rbErr, err)
		}
		return result, err
	}

	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

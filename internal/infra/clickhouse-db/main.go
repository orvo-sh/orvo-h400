package clickhousedb

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type DB struct {
	conn driver.Conn
}

type Config struct {
	Address  string
	Database string
	User     string
	Password string
}

func New(config Config) (*DB, error) {

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{config.Address},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.User,
			Password: config.Password,
		},
	})
	if err != nil {
		return nil, err
	}

	err = conn.Ping(context.Background())
	if err != nil {
		return nil, err
	}

	return &DB{
		conn: conn,
	}, nil
}

func (ch *DB) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	rows, err := ch.conn.Query(ctx, query, args...)
	return rows, err
}

func (ch *DB) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	row := ch.conn.QueryRow(ctx, query, args...)
	return row
}

func (ch *DB) Exec(ctx context.Context, query string, args ...any) error {
	err := ch.conn.Exec(ctx, query, args...)
	return err
}

func (ch *DB) PrepareBatch(ctx context.Context, query string) (driver.Batch, error) {
	return ch.conn.PrepareBatch(ctx, query)
}

func (ch *DB) Close() error {
	return ch.conn.Close()
}

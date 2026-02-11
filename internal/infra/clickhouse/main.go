package clickhouse

import (
	"context"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("clickhouse")

type DB struct {
	conn driver.Conn
}

type Config struct {
	Address  string
	Database string
	User     string
	Password string
}

func New(ctx context.Context, config Config) (*DB, error) {

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

	err = conn.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &DB{
		conn: conn,
	}, nil
}

// spanAttrs returns common span attributes for a ClickHouse operation.
func spanAttrs(query string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "clickhouse"),
	}
	stmt := strings.TrimSpace(query)
	if len(stmt) > 200 {
		stmt = stmt[:200]
	}
	if stmt != "" {
		attrs = append(attrs, attribute.String("db.statement", stmt))
	}
	return attrs
}

func (ch *DB) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	ctx, span := tracer.Start(ctx, "clickhouse.Query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(spanAttrs(query)...),
	)
	defer span.End()

	rows, err := ch.conn.Query(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return rows, err
}

func (ch *DB) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	ctx, span := tracer.Start(ctx, "clickhouse.QueryRow",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(spanAttrs(query)...),
	)
	defer span.End()

	row := ch.conn.QueryRow(ctx, query, args...)
	return row
}

func (ch *DB) Exec(ctx context.Context, query string, args ...any) error {
	ctx, span := tracer.Start(ctx, "clickhouse.Exec",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(spanAttrs(query)...),
	)
	defer span.End()

	err := ch.conn.Exec(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func (ch *DB) PrepareBatch(ctx context.Context, query string) (driver.Batch, error) {
	ctx, span := tracer.Start(ctx, "clickhouse.PrepareBatch",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(spanAttrs(query)...),
	)
	defer span.End()

	batch, err := ch.conn.PrepareBatch(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return batch, err
}

func (ch *DB) Close() error {
	return ch.conn.Close()
}

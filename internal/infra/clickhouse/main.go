package clickhouse

import (
	"context"
	"fmt"
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
	URL string
}

func New(ctx context.Context, config Config) (*DB, error) {
	opts, err := clickhouse.ParseDSN(config.URL)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: failed to parse ClickHouse DSN: %w", err)
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: failed to connect to ClickHouse: %w", err)
	}

	if err = conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse: failed to ping ClickHouse: %w", err)
	}

	return &DB{
		conn: conn,
	}, nil
}

func (ch *DB) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	if hasParentSpan(ctx) {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "clickhouse.Query",
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

	return ch.conn.Query(ctx, query, args...)
}

func (ch *DB) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	if hasParentSpan(ctx) {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "clickhouse.QueryRow",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(spanAttrs(query)...),
		)
		defer span.End()

		return ch.conn.QueryRow(ctx, query, args...)
	}

	return ch.conn.QueryRow(ctx, query, args...)
}

func (ch *DB) Exec(ctx context.Context, query string, args ...any) error {
	if hasParentSpan(ctx) {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "clickhouse.Exec",
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

	return ch.conn.Exec(ctx, query, args...)
}

func (ch *DB) PrepareBatch(ctx context.Context, query string) (driver.Batch, error) {
	if hasParentSpan(ctx) {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "clickhouse.PrepareBatch",
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

	return ch.conn.PrepareBatch(ctx, query)
}

func (ch *DB) Close() error {
	return ch.conn.Close()
}

func hasParentSpan(ctx context.Context) bool {
	sc := trace.SpanFromContext(ctx).SpanContext()
	return sc.IsValid()
}

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

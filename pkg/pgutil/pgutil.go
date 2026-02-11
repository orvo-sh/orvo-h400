package pgutil

import (
	"errors"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func Text(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func TextFromString(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

func NullText() pgtype.Text {
	return pgtype.Text{Valid: false}
}

func TextToPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func TextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func NullTimestamptz() pgtype.Timestamptz {
	return pgtype.Timestamptz{Valid: false}
}

func TimestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func TimestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func Int4(i int32) pgtype.Int4 {
	return pgtype.Int4{Int32: i, Valid: true}
}

func NullInt4() pgtype.Int4 {
	return pgtype.Int4{Valid: false}
}

func Int4ToPtr(i pgtype.Int4) *int32 {
	if !i.Valid {
		return nil
	}
	return &i.Int32
}

func Bool(b bool) pgtype.Bool {
	return pgtype.Bool{Bool: b, Valid: true}
}

func BoolToBool(b pgtype.Bool) bool {
	if !b.Valid {
		return false
	}
	return b.Bool
}

func BoolFromPtr(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{Valid: false}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func NullBool() pgtype.Bool {
	return pgtype.Bool{Valid: false}
}

func IsUniqueViolationError(err error, name string) bool {
	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23505" {
		return false
	}

	if pgErr.ConstraintName == name {
		return true
	}

	return true
}

func IsNoRowsError(err error) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	return false
}

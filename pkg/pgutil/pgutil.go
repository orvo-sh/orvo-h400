package pgutil

import (
	"errors"
	"regexp"
	"strings"
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

var uniqueViolationRegex = regexp.MustCompile(`Key \(([^)]+)\)=`)

func IsUniqueViolationError(err error, fields []string) bool {
	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23505" {
		return false
	}

	if len(fields) == 0 {
		return true
	}

	matches := uniqueViolationRegex.FindStringSubmatch(pgErr.Detail)
	if len(matches) < 2 {
		return false
	}

	columnList := matches[1]
	affectedColumns := strings.Split(columnList, ", ")

	for _, requiredField := range fields {
		found := false
		for _, col := range affectedColumns {
			if col == requiredField {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func IsNoRowsError(err error) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	return false
}

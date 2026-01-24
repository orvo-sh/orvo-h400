package pgutil

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Text creates a pgtype.Text from a string pointer
func Text(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// TextFromString creates a pgtype.Text from a string
func TextFromString(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// NullText creates an invalid (null) pgtype.Text
func NullText() pgtype.Text {
	return pgtype.Text{Valid: false}
}

// TextToPtr converts a pgtype.Text to a string pointer
func TextToPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// TextToString converts a pgtype.Text to a string (empty if null)
func TextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// Timestamptz creates a pgtype.Timestamptz from a time.Time
func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// NullTimestamptz creates an invalid (null) pgtype.Timestamptz
func NullTimestamptz() pgtype.Timestamptz {
	return pgtype.Timestamptz{Valid: false}
}

// TimestamptzToTime converts a pgtype.Timestamptz to time.Time
func TimestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// TimestamptzToPtr converts a pgtype.Timestamptz to a time.Time pointer
func TimestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// Int4 creates a pgtype.Int4 from an int32
func Int4(i int32) pgtype.Int4 {
	return pgtype.Int4{Int32: i, Valid: true}
}

// NullInt4 creates an invalid (null) pgtype.Int4
func NullInt4() pgtype.Int4 {
	return pgtype.Int4{Valid: false}
}

// Int4ToPtr converts a pgtype.Int4 to an int32 pointer
func Int4ToPtr(i pgtype.Int4) *int32 {
	if !i.Valid {
		return nil
	}
	return &i.Int32
}

// Bool creates a pgtype.Bool from a bool
func Bool(b bool) pgtype.Bool {
	return pgtype.Bool{Bool: b, Valid: true}
}

// BoolToBool converts a pgtype.Bool to bool
func BoolToBool(b pgtype.Bool) bool {
	if !b.Valid {
		return false
	}
	return b.Bool
}

// BoolFromPtr creates a pgtype.Bool from a bool pointer
func BoolFromPtr(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{Valid: false}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

// NullBool creates an invalid (null) pgtype.Bool
func NullBool() pgtype.Bool {
	return pgtype.Bool{Valid: false}
}

package repository

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case pgtype.Bool:
		return typed.Valid && typed.Bool
	default:
		return false
	}
}

func unix(value pgtype.Timestamptz) int64 {
	if !value.Valid {
		return 0
	}
	return value.Time.Unix()
}

func pgText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func pgDate(value time.Time) pgtype.Date {
	if value.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: value, Valid: true}
}

func pgTime(value time.Time) pgtype.Timestamptz {
	if value.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func nullableText(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func dateString(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

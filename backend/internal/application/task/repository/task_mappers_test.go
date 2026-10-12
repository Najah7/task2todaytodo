package repository

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestSQLCBooleanSupportsNativeAndPostgresBooleanValues(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "native true", value: true, want: true},
		{name: "native false", value: false},
		{name: "valid pg true", value: pgtype.Bool{Bool: true, Valid: true}, want: true},
		{name: "valid pg false", value: pgtype.Bool{Bool: false, Valid: true}},
		{name: "invalid pg true", value: pgtype.Bool{Bool: true, Valid: false}},
		{name: "unknown type", value: "true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sqlcBoolean(test.value); got != test.want {
				t.Fatalf("sqlcBoolean(%#v) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

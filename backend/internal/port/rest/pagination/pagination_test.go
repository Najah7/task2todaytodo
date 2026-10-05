package pagination

import (
	"errors"
	"testing"

	sharedpagination "github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
)

func TestParsePageSize(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		provided bool
		want     int
		wantErr  bool
	}{
		{name: "omitted", want: sharedpagination.DefaultPageSize},
		{name: "zero", value: "0", provided: true, want: sharedpagination.DefaultPageSize},
		{name: "ordinary", value: "37", provided: true, want: 37},
		{name: "maximum", value: "100", provided: true, want: 100},
		{name: "clamped", value: "101", provided: true, want: sharedpagination.MaxPageSize},
		{name: "large clamped", value: "999999999", provided: true, want: sharedpagination.MaxPageSize},
		{name: "negative", value: "-1", provided: true, wantErr: true},
		{name: "fraction", value: "1.5", provided: true, wantErr: true},
		{name: "empty", value: "", provided: true, wantErr: true},
		{name: "whitespace", value: " 2", provided: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePageSize(tt.value, tt.provided)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidPageSize) {
					t.Fatalf("ParsePageSize() error = %v, want ErrInvalidPageSize", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ParsePageSize() = %d, %v; want %d, nil", got, err, tt.want)
			}
		})
	}
}

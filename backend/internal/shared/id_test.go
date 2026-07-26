package shared

import (
	"errors"
	"testing"
)

func TestNewID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    ID
		wantErr error
	}{
		{name: "valid", value: "test-id", want: "test-id"},
		{name: "empty", wantErr: ErrIDEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewID(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewID() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ID = %q, want %q", got, tt.want)
			}
		})
	}
}

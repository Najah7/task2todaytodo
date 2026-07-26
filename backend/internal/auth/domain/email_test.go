package domain

import (
	"errors"
	"testing"
)

func TestNewEmail(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr error
	}{
		{name: "valid", value: "user@example.com"},
		{name: "empty", wantErr: ErrEmailEmpty},
		{name: "invalid format", value: "invalid-email", wantErr: ErrInvalidEmailFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewEmail(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewEmail() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.String() != tt.value {
				t.Errorf("email = %q, want %q", got, tt.value)
			}
		})
	}
}

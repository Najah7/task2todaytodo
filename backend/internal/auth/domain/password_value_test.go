package domain

import (
	"errors"
	"testing"
)

func TestNewPasswordHashesValidatedValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr error
	}{
		{name: "valid", value: "Password1!"},
		{name: "empty", wantErr: ErrPasswordEmpty},
		{name: "too short", value: "short", wantErr: ErrPasswordTooShort},
		{name: "missing lowercase", value: "PASSWORD1!", wantErr: ErrPasswordMissingLowercase},
		{name: "missing uppercase", value: "password1!", wantErr: ErrPasswordMissingUppercase},
		{name: "missing digit", value: "Password!!", wantErr: ErrPasswordMissingDigit},
		{name: "missing special character", value: "Password12", wantErr: ErrPasswordMissingSpecial},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPassword(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewPassword() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && (!got.IsHashed || got.String() != "hashed_"+tt.value) {
				t.Errorf("password = %+v, want hashed password for %q", got, tt.value)
			}
		})
	}
}

func TestNewHashedPassword(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr error
	}{
		{name: "valid", value: "hashed_Password1!"},
		{name: "empty", wantErr: ErrPasswordEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewHashedPassword(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewHashedPassword() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && (!got.IsHashed || got.String() != tt.value) {
				t.Errorf("hashed password = %+v, want %+v", got, tt.value)
			}
		})
	}
}

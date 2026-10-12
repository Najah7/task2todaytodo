package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewPasswordHashesAndVerifiesInput(t *testing.T) {
	password, err := NewPassword("Password1!")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(password.String(), "hashed_") || !strings.HasPrefix(password.String(), "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("password hash = %q, want Argon2id PHC format", password.String())
	}
	if !password.Verify("Password1!") || password.Verify("WrongPassword1!") {
		t.Fatal("Verify() did not distinguish correct and incorrect passwords")
	}
	if password.String() == "Password1!" {
		t.Fatal("stored password contains plaintext")
	}
}

func TestNewPasswordValidatesInput(t *testing.T) {
	tests := []struct {
		value string
		want  error
	}{
		{"", ErrPasswordEmpty},
		{"short", ErrPasswordTooShort},
		{"PASSWORD1!", ErrPasswordMissingLowercase},
		{"password1!", ErrPasswordMissingUppercase},
		{"Password!!", ErrPasswordMissingDigit},
		{"Password12", ErrPasswordMissingSpecial},
	}
	for _, test := range tests {
		if _, err := NewPassword(test.value); !errors.Is(err, test.want) {
			t.Errorf("NewPassword(%q) error = %v, want %v", test.value, err, test.want)
		}
	}
}

func TestNewHashedPasswordRejectsMalformedAndUnsafeParameters(t *testing.T) {
	for _, value := range []string{
		"hashed_Password1!",
		"$argon2id$v=19$m=1073741824,t=2,p=1$YWJjZGVmZ2hpamtsbW5vcA$YWJjZGVmZ2hpamtsbW5vcA",
	} {
		if _, err := NewHashedPassword(value); !errors.Is(err, ErrPasswordMustBeHashed) {
			t.Errorf("NewHashedPassword() error = %v, want invalid stored hash", err)
		}
	}
	if _, err := NewHashedPassword(""); !errors.Is(err, ErrPasswordEmpty) {
		t.Errorf("empty hash error = %v, want %v", err, ErrPasswordEmpty)
	}
}

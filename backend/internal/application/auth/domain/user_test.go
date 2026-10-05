package domain

import (
	"errors"
	"testing"
)

func makeUserInputs(t *testing.T) (string, Email, Password) {
	t.Helper()

	id := "user-1"
	email, err := NewEmail("user@example.com")
	if err != nil {
		t.Fatalf("NewEmail() error = %v", err)
	}
	password, err := NewPassword("Password1!")
	if err != nil {
		t.Fatalf("NewPassword() error = %v", err)
	}

	return id, email, password
}

func TestUserLifecycle(t *testing.T) {
	id, email, password := makeUserInputs(t)
	user := NewUser(UserID(id), email, password, NewUserName("", ""))

	updated, err := user.UpdateName(NewUserName("Jane", "Doe"))
	if err != nil {
		t.Fatalf("UpdateName() error = %v", err)
	}
	if updated.FullName() != "Jane Doe" {
		t.Errorf("FullName() = %q, want %q", updated.FullName(), "Jane Doe")
	}

	newEmail, err := NewEmail("jane@example.com")
	if err != nil {
		t.Fatalf("NewEmail() error = %v", err)
	}
	updated = updated.UpdateEmail(newEmail)

	newPassword, err := NewPassword("NewPassword1!")
	if err != nil {
		t.Fatalf("NewPassword() error = %v", err)
	}
	updated = updated.UpdatePassword(newPassword)
	if !updated.Login(newEmail, newPassword) {
		t.Error("Login() = false, want true for updated credentials")
	}
}

func TestUserUpdateNameRequiresFirstName(t *testing.T) {
	id, email, password := makeUserInputs(t)
	user := NewUser(UserID(id), email, password, NewUserName("", ""))

	got, err := user.UpdateName(NewUserName("", "Doe"))
	if !errors.Is(err, ErrFirstNameRequired) {
		t.Errorf("UpdateName() error = %v, want %v", err, ErrFirstNameRequired)
	}
	if !got.IsZero() {
		t.Errorf("user = %+v, want zero user", got)
	}
}

func TestUserLoginRejectsWrongCredentials(t *testing.T) {
	id, email, password := makeUserInputs(t)
	user := NewUser(UserID(id), email, password, NewUserName("", ""))

	wrongEmail, _ := NewEmail("other@example.com")
	wrongPassword, _ := NewPassword("WrongPassword1!")

	if user.Login(wrongEmail, password) {
		t.Error("Login() = true with a wrong email")
	}
	if user.Login(email, wrongPassword) {
		t.Error("Login() = true with a wrong password")
	}
}

func TestUserTimezoneDefaultsAndSurvivesOtherUpdates(t *testing.T) {
	id, email, password := makeUserInputs(t)
	user := NewUser(UserID(id), email, password, NewUserName("Jane", "Doe"))
	if user.Timezone.String() != "Asia/Tokyo" {
		t.Fatalf("timezone = %q, want Asia/Tokyo", user.Timezone)
	}

	timezone, err := NewUserTimezone("America/Los_Angeles")
	if err != nil {
		t.Fatalf("NewUserTimezone() error = %v", err)
	}
	user = user.UpdateTimezone(timezone)
	updatedName, err := user.UpdateName(NewUserName("Janet", "Doe"))
	if err != nil {
		t.Fatalf("UpdateName() error = %v", err)
	}
	updatedEmail := updatedName.UpdateEmail(email)
	updatedPassword := updatedEmail.UpdatePassword(password)
	if updatedPassword.Timezone != timezone {
		t.Errorf("timezone after profile updates = %q, want %q", updatedPassword.Timezone, timezone)
	}
}

func TestNewUserTimezoneRequiresIANAName(t *testing.T) {
	for _, value := range []string{"Mars/Olympus", "Local"} {
		if _, err := NewUserTimezone(value); !errors.Is(err, ErrInvalidUserTimezone) {
			t.Errorf("NewUserTimezone(%q) error = %v, want %v", value, err, ErrInvalidUserTimezone)
		}
	}
}

package auth

import (
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
)

func makeUserInputs(t *testing.T) (shared.ID, Email, Password) {
	t.Helper()

	id, err := shared.NewID("user-1")
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
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

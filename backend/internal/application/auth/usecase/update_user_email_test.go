package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
)

func TestUpdateUserEmailUseCaseExecute(t *testing.T) {
	user := existingUser(t)
	got, err := NewUpdateUserEmailUseCase(&stubUserRepository{user: user}, nil).Execute(context.Background(), user.ID, "new@example.com")
	assertServiceErrorIs(t, err, nil)
	if got.Email != "new@example.com" {
		t.Errorf("email = %q, want %q", got.Email, "new@example.com")
	}
}

func TestUpdateUserEmailUseCaseRejectsAnotherUsersEmail(t *testing.T) {
	user := existingUser(t)
	got, err := NewUpdateUserEmailUseCase(&stubUserRepository{user: user, userByEmail: user}, nil).Execute(context.Background(), "other-user", "user@example.com")
	assertServiceErrorIs(t, err, ErrUserEmailAlreadyExists)
	if got != (dao.User{}) {
		t.Errorf("user = %+v, want zero DAO user", got)
	}
}

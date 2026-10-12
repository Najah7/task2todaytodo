package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

func TestUpdateUserPasswordUseCaseExecute(t *testing.T) {
	user := existingUser(t)
	repo := &passwordUpdateRepository{user: userDAOFromDomain(user)}
	err := NewUpdateUserPasswordUseCase(passwordUpdateUOW{repository: repo}, nil).Execute(context.Background(), user.ID, "NewPassword1!")
	assertServiceErrorIs(t, err, nil)
	newHash, parseErr := domain.NewHashedPassword(repo.user.PasswordHash)
	assertServiceErrorIs(t, parseErr, nil)
	if !newHash.Verify("NewPassword1!") {
		t.Fatal("stored password does not match new password")
	}
}

func TestUpdateUserPasswordUseCaseDetectsUnexpectedPersistedValue(t *testing.T) {
	user := existingUser(t)
	err := NewUpdateUserPasswordUseCase(passwordUpdateUOW{repository: &passwordUpdateRepository{user: userDAOFromDomain(user), err: errUpdateUser}}, nil).Execute(context.Background(), user.ID, "NewPassword1!")
	assertServiceErrorIs(t, err, errUpdateUser)
}

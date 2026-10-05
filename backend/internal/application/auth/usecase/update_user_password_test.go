package usecase

import (
	"context"
	"testing"
)

func TestUpdateUserPasswordUseCaseExecute(t *testing.T) {
	user := existingUser(t)
	err := NewUpdateUserPasswordUseCase(&stubUserRepository{user: user}, nil).Execute(context.Background(), user.ID, "NewPassword1!")
	assertServiceErrorIs(t, err, nil)
}

func TestUpdateUserPasswordUseCaseDetectsUnexpectedPersistedValue(t *testing.T) {
	user := existingUser(t)
	err := NewUpdateUserPasswordUseCase(&stubUserRepository{user: user, updateResult: user}, nil).Execute(context.Background(), user.ID, "NewPassword1!")
	assertServiceErrorIs(t, err, ErrPasswordUpdateFailed)
}

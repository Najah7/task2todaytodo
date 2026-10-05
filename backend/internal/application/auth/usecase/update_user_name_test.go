package usecase

import (
	"context"
	"testing"
)

func TestUpdateUserNameUseCaseExecute(t *testing.T) {
	user := existingUser(t)
	got, err := NewUpdateUserNameUseCase(&stubUserRepository{user: user}, nil).Execute(context.Background(), user.ID, "Jane", "Doe")
	assertServiceErrorIs(t, err, nil)
	if got.FirstName != "Jane" || got.LastName != "Doe" {
		t.Errorf("name = %q %q, want %q %q", got.FirstName, got.LastName, "Jane", "Doe")
	}
}

func TestUpdateUserNameUseCasePropagatesRepositoryError(t *testing.T) {
	_, err := NewUpdateUserNameUseCase(&stubUserRepository{getErr: errGetUser}, nil).Execute(context.Background(), "user-1", "Jane", "Doe")
	assertServiceErrorIs(t, err, errGetUser)
}

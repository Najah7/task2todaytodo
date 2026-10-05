package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
)

func TestGetUserUseCaseExecute(t *testing.T) {
	want := existingUser(t)
	got, err := NewGetUserUseCase(&stubUserRepository{user: want}, nil).Execute(context.Background(), want.ID)
	assertServiceErrorIs(t, err, nil)
	if got != userReadDAOFromDomain(want) {
		t.Errorf("user = %+v, want %+v", got, userReadDAOFromDomain(want))
	}
}

func TestGetUserUseCasePropagatesRepositoryError(t *testing.T) {
	got, err := NewGetUserUseCase(&stubUserRepository{getErr: errGetUser}, nil).Execute(context.Background(), "user-1")
	assertServiceErrorIs(t, err, errGetUser)
	if got != (dao.User{}) {
		t.Errorf("user = %+v, want zero DAO user", got)
	}
}

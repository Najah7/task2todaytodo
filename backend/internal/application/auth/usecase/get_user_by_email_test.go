package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

func TestGetUserByEmailUseCaseExecute(t *testing.T) {
	want := existingUser(t)
	got, err := NewGetUserByEmailUseCase(&stubUserRepository{userByEmail: want}, nil).Execute(context.Background(), want.Email.String())
	assertServiceErrorIs(t, err, nil)
	if got != userReadDAOFromDomain(want) {
		t.Errorf("user = %+v, want %+v", got, userReadDAOFromDomain(want))
	}
}

func TestGetUserByEmailUseCaseValidatesEmail(t *testing.T) {
	got, err := NewGetUserByEmailUseCase(&stubUserRepository{}, nil).Execute(context.Background(), "invalid-email")
	assertServiceErrorIs(t, err, domain.ErrInvalidEmailFormat)
	if got != (dao.User{}) {
		t.Errorf("user = %+v, want zero DAO user", got)
	}
}

func TestGetUserByEmailUseCasePropagatesRepositoryError(t *testing.T) {
	got, err := NewGetUserByEmailUseCase(&stubUserRepository{getByEmailErr: errGetUserByEmail}, nil).Execute(context.Background(), "user@example.com")
	assertServiceErrorIs(t, err, errGetUserByEmail)
	if got != (dao.User{}) {
		t.Errorf("user = %+v, want zero DAO user", got)
	}
}

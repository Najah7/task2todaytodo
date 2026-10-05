package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
)

func TestSignUpUseCaseExecute(t *testing.T) {
	repo := &stubUserRepository{}
	svc := NewSignUpUseCase(repo, nil)

	got, err := svc.Execute(context.Background(), "new-user", "new@example.com", "Password1!")
	assertServiceErrorIs(t, err, nil)
	if got.ID != "new-user" || got.Email != "new@example.com" || got.Password != "" || repo.createdUser.Password.String() != "hashed_Password1!" {
		t.Errorf("created user = %+v, want a persisted, hashed user", got)
	}
}

func TestSignUpUseCaseRejectsDuplicates(t *testing.T) {
	tests := []struct {
		name string
		repo *stubUserRepository
		want error
	}{
		{name: "email", repo: &stubUserRepository{userByEmail: existingUser(t)}, want: ErrUserEmailAlreadyExists},
		{name: "ID", repo: &stubUserRepository{user: existingUser(t)}, want: ErrUserIDAlreadyExists},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSignUpUseCase(tt.repo, nil).Execute(context.Background(), "user-1", "new@example.com", "Password1!")
			assertServiceErrorIs(t, err, tt.want)
			if got != (dao.User{}) {
				t.Errorf("user = %+v, want zero DAO user", got)
			}
		})
	}
}

func TestSignUpUseCasePropagatesRepositoryError(t *testing.T) {
	repo := &stubUserRepository{createErr: errCreateUser}
	_, err := NewSignUpUseCase(repo, nil).Execute(context.Background(), "new-user", "new@example.com", "Password1!")
	assertServiceErrorIs(t, err, errCreateUser)
}

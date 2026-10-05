package usecase

import (
	"context"
	"testing"
)

func TestLoginUserUseCaseExecute(t *testing.T) {
	user := existingUser(t)
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "success", password: "Password1!"},
		{name: "wrong password", password: "WrongPassword1!", wantErr: ErrInvalidCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewLoginUserUseCase(&stubUserRepository{userByEmail: user}, nil).Execute(context.Background(), "user@example.com", tt.password)
			assertServiceErrorIs(t, err, tt.wantErr)
			if tt.wantErr == nil {
				if got != user.ID {
					t.Errorf("user ID = %q, want %q", got, user.ID)
				}
			}
		})
	}
}

func TestLoginUserUseCasePropagatesRepositoryError(t *testing.T) {
	_, err := NewLoginUserUseCase(&stubUserRepository{getByEmailErr: errGetUserByEmail}, nil).Execute(context.Background(), "user@example.com", "Password1!")
	assertServiceErrorIs(t, err, errGetUserByEmail)
}

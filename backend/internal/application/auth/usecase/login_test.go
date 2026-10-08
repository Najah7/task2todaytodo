package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
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

func TestLoginUserUseCaseMapsWrappedDriverNoRowsToInvalidCredentials(t *testing.T) {
	driverErr := fmt.Errorf("get user by email: %w", pgx.ErrNoRows)
	_, err := NewLoginUserUseCase(&stubUserRepository{getByEmailErr: driverErr}, nil).Execute(context.Background(), "unknown@example.com", "Password1!")

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Execute() error = %v, want %v", err, ErrInvalidCredentials)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("Execute() exposed missing-user driver error: %v", err)
	}
}

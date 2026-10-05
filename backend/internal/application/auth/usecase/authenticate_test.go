package usecase

import (
	"context"
	"testing"
)

func TestAuthenticateUseCaseExecute(t *testing.T) {
	want := serviceAccessToken(t)
	got, err := NewAuthenticateUseCase(&stubAccessTokenRepository{token: want}, nil).Execute(context.Background(), want.Token)
	assertServiceErrorIs(t, err, nil)
	if got != want.UserID {
		t.Errorf("user ID = %q, want %q", got, want.UserID)
	}
}

func TestAuthenticateUseCasePropagatesRepositoryError(t *testing.T) {
	got, err := NewAuthenticateUseCase(&stubAccessTokenRepository{getErr: errGetAccessToken}, nil).Execute(context.Background(), "token-1")
	assertServiceErrorIs(t, err, errGetAccessToken)
	if got != "" {
		t.Errorf("user ID = %q, want empty user ID", got)
	}
}

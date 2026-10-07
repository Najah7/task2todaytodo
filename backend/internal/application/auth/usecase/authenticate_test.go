package usecase

import (
	"context"
	"testing"
)

func TestAuthenticateUseCaseExecute(t *testing.T) {
	want := servicePersonalAccessToken(t)
	got, err := NewAuthenticateUseCase(&stubPersonalAccessTokenRepository{token: want}, nil).Execute(context.Background(), want.Token)
	assertServiceErrorIs(t, err, nil)
	if got != want.UserID {
		t.Errorf("user ID = %q, want %q", got, want.UserID)
	}
}

func TestAuthenticateUseCasePropagatesRepositoryError(t *testing.T) {
	got, err := NewAuthenticateUseCase(&stubPersonalAccessTokenRepository{getErr: errGetPersonalAccessToken}, nil).Execute(context.Background(), "token-1")
	assertServiceErrorIs(t, err, errGetPersonalAccessToken)
	if got != "" {
		t.Errorf("user ID = %q, want empty user ID", got)
	}
}

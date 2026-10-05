package usecase

import (
	"context"
	"testing"

	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

func TestGenerateAccessTokenUseCaseExecute(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &stubAccessTokenRepository{}
		got, err := NewGenerateAccessTokenUseCase(repo, nil).Execute(context.Background(), "user-1")
		assertAccessTokenErrorIs(t, err, nil)
		if got.Token == "" || got.UserID != "user-1" || got.Token != repo.createdToken.Token || got.ExpiresAt != repo.createdToken.ExpiresAt {
			t.Errorf("access token = %+v, want persisted token for user-1", got)
		}
	})

	t.Run("empty user ID", func(t *testing.T) {
		got, err := NewGenerateAccessTokenUseCase(&stubAccessTokenRepository{}, nil).Execute(context.Background(), "")
		assertAccessTokenErrorIs(t, err, domain.ErrAccessTokenUserIDEmpty)
		if got.Token != "" {
			t.Errorf("access token = %+v, want zero access token", got)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		_, err := NewGenerateAccessTokenUseCase(&stubAccessTokenRepository{createErr: errCreateAccessToken}, nil).Execute(context.Background(), "user-1")
		assertAccessTokenErrorIs(t, err, errCreateAccessToken)
	})
}

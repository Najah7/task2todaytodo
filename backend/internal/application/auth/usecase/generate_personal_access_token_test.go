package usecase

import (
	"context"
	"testing"

	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

func TestGeneratePersonalAccessTokenUseCaseExecute(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &stubPersonalAccessTokenRepository{}
		got, err := NewGeneratePersonalAccessTokenUseCase(repo, nil).Execute(context.Background(), "user-1")
		assertPersonalAccessTokenErrorIs(t, err, nil)
		if got.Token == "" || got.UserID != "user-1" || got.Token != repo.createdToken.Token || got.ExpiresAt != repo.createdToken.ExpiresAt {
			t.Errorf("access token = %+v, want persisted token for user-1", got)
		}
	})

	t.Run("empty user ID", func(t *testing.T) {
		got, err := NewGeneratePersonalAccessTokenUseCase(&stubPersonalAccessTokenRepository{}, nil).Execute(context.Background(), "")
		assertPersonalAccessTokenErrorIs(t, err, domain.ErrPersonalAccessTokenUserIDEmpty)
		if got.Token != "" {
			t.Errorf("access token = %+v, want zero access token", got)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		_, err := NewGeneratePersonalAccessTokenUseCase(&stubPersonalAccessTokenRepository{createErr: errCreatePersonalAccessToken}, nil).Execute(context.Background(), "user-1")
		assertPersonalAccessTokenErrorIs(t, err, errCreatePersonalAccessToken)
	})
}

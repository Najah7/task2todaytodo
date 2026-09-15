package usecase

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/auth/domain"
)

type AccessTokenService struct {
	repo AccessTokenRepository
}

func NewAccessTokenService(repo AccessTokenRepository) *AccessTokenService {
	return &AccessTokenService{
		repo: repo,
	}
}

func (s *AccessTokenService) GetByToken(ctx context.Context, token string) (domain.AccessToken, error) {
	t, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return domain.NewZeroAccessToken(), err
	}
	return t, nil
}

func (s *AccessTokenService) Generate(ctx context.Context, userID domain.UserID) (domain.AccessToken, error) {
	newToken, err := domain.NewAccessToken(userID)
	if err != nil {
		return domain.NewZeroAccessToken(), err
	}

	return s.repo.Create(ctx, newToken)
}

func (s *AccessTokenService) Revoke(ctx context.Context, token string) error {
	t, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return err
	}

	if t.IsRevoked() {
		return nil
	}

	return s.repo.Revoke(ctx, token)
}

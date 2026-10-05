package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type GenerateAccessTokenUseCase struct {
	repo   AccessTokenRepository
	logger logging.Logger
}

func NewGenerateAccessTokenUseCase(repo AccessTokenRepository, logger logging.Logger) *GenerateAccessTokenUseCase {
	return &GenerateAccessTokenUseCase{
		repo: repo, logger: logging.OrNop(logger),
	}
}

func (uc *GenerateAccessTokenUseCase) Execute(ctx context.Context, userID domain.UserID) (dao.AccessToken, error) {
	newToken, err := domain.NewAccessToken(userID)
	if err != nil {
		return dao.AccessToken{}, err
	}

	token, err := uc.repo.Create(ctx, newToken)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "access_token.create", err)
		return dao.AccessToken{}, err
	}
	uc.logger.Info(ctx, "access token issued", "operation", "access_token.create")
	return token, nil
}

package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type GeneratePersonalAccessTokenUseCase struct {
	repo   PersonalAccessTokenRepository
	logger logging.Logger
}

func NewGeneratePersonalAccessTokenUseCase(repo PersonalAccessTokenRepository, logger logging.Logger) *GeneratePersonalAccessTokenUseCase {
	return &GeneratePersonalAccessTokenUseCase{
		repo: repo, logger: logging.OrNop(logger),
	}
}

func (uc *GeneratePersonalAccessTokenUseCase) Execute(ctx context.Context, userID domain.UserID) (dao.PersonalAccessToken, error) {
	newToken, err := domain.NewPersonalAccessToken(userID)
	if err != nil {
		return dao.PersonalAccessToken{}, err
	}

	token, err := uc.repo.Create(ctx, newToken)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "personal_access_token.create", err)
		return dao.PersonalAccessToken{}, err
	}
	uc.logger.Info(ctx, "personal access token issued", "operation", "personal_access_token.create")
	return token, nil
}

package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type AuthenticateUseCase struct {
	repo   PersonalAccessTokenRepository
	logger logging.Logger
}

func NewAuthenticateUseCase(repo PersonalAccessTokenRepository, logger logging.Logger) *AuthenticateUseCase {
	return &AuthenticateUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *AuthenticateUseCase) Execute(ctx context.Context, token string) (domain.UserID, error) {
	dao, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		if ctx.Err() == nil && logging.IsNotFound(err) {
			logAuthenticationRejected(uc.logger, ctx, "invalid_token")
		} else {
			logUnexpectedFailure(uc.logger, ctx, "authenticate.load_token", err)
		}
		return "", err
	}

	personalAccessToken, err := domain.NewExistingPersonalAccessToken(
		dao.Token,
		domain.UserID(dao.UserID),
		dao.ExpiresAt,
		dao.RevokedAt,
		dao.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, domain.ErrPersonalAccessTokenExpired) || errors.Is(err, domain.ErrPersonalAccessTokenRevoked) {
			logAuthenticationRejected(uc.logger, ctx, "inactive_token")
		} else {
			logUnexpectedFailure(uc.logger, ctx, "authenticate.restore_token", err)
		}
		return "", err
	}

	if personalAccessToken.IsExpired() {
		logAuthenticationRejected(uc.logger, ctx, "expired_token")
		return "", domain.ErrPersonalAccessTokenExpired
	}

	return personalAccessToken.UserID, nil
}

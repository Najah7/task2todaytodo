package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type AuthenticateUseCase struct {
	repo   AccessTokenRepository
	logger logging.Logger
}

func NewAuthenticateUseCase(repo AccessTokenRepository, logger logging.Logger) *AuthenticateUseCase {
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

	accessToken, err := domain.NewExistingAccessToken(
		dao.Token,
		domain.UserID(dao.UserID),
		dao.ExpiresAt,
		dao.RevokedAt,
		dao.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, domain.ErrAccessTokenExpired) || errors.Is(err, domain.ErrAccessTokenRevoked) {
			logAuthenticationRejected(uc.logger, ctx, "inactive_token")
		} else {
			logUnexpectedFailure(uc.logger, ctx, "authenticate.restore_token", err)
		}
		return "", err
	}

	if accessToken.IsExpired() {
		logAuthenticationRejected(uc.logger, ctx, "expired_token")
		return "", domain.ErrAccessTokenExpired
	}

	return accessToken.UserID, nil
}

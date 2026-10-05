package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/logging"
)

type RevokeAccessTokenUseCase struct {
	repo   AccessTokenRepository
	logger logging.Logger
}

func NewRevokeAccessTokenUseCase(repo AccessTokenRepository, logger logging.Logger) *RevokeAccessTokenUseCase {
	return &RevokeAccessTokenUseCase{
		repo: repo, logger: logging.OrNop(logger),
	}
}

func (uc *RevokeAccessTokenUseCase) Execute(ctx context.Context, token string) error {
	t, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "access_token.revoke.load", err)
		return err
	}

	if t.RevokedAt != 0 {
		return nil
	}

	if err := uc.repo.Revoke(ctx, token); err != nil {
		logUnexpectedFailure(uc.logger, ctx, "access_token.revoke.persist", err)
		return err
	}
	uc.logger.Info(ctx, "access token revoked", "operation", "access_token.revoke")
	return nil
}

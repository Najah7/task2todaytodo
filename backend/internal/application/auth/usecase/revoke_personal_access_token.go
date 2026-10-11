package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RevokePersonalAccessTokenUseCase struct {
	repo   PersonalAccessTokenRepository
	logger telemetry.Logger
}

func NewRevokePersonalAccessTokenUseCase(repo PersonalAccessTokenRepository, logger telemetry.Logger) *RevokePersonalAccessTokenUseCase {
	return &RevokePersonalAccessTokenUseCase{
		repo: repo, logger: telemetry.OrNop(logger),
	}
}

func (uc *RevokePersonalAccessTokenUseCase) Execute(ctx context.Context, token string) error {
	t, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "personal_access_token.revoke.load", err)
		return err
	}

	if t.RevokedAt != 0 {
		return nil
	}

	if err := uc.repo.Revoke(ctx, token); err != nil {
		logUnexpectedFailure(uc.logger, ctx, "personal_access_token.revoke.persist", err)
		return err
	}
	uc.logger.Info(ctx, "personal access token revoked", "operation", "personal_access_token.revoke")
	return nil
}

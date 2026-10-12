package usecase

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type UpdateUserPasswordUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewUpdateUserPasswordUseCase(uow UOW, logger telemetry.Logger) *UpdateUserPasswordUseCase {
	return &UpdateUserPasswordUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *UpdateUserPasswordUseCase) Execute(ctx context.Context, userID domain.UserID, newPassword string) error {
	p, err := domain.NewPassword(newPassword)
	if err != nil {
		return err
	}

	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := repos.EmailAuth().GetUserByIDForAuth(ctx, string(userID)); err != nil {
			return err
		}
		if err := repos.EmailAuth().UpdateUserPasswordForReset(ctx, string(userID), p.String()); err != nil {
			return err
		}
		if err := repos.EmailAuth().RevokeAllUserPersonalAccessTokens(ctx, string(userID)); err != nil {
			return err
		}
		return repos.EmailAuth().RevokeUserAuthLinkTokens(ctx, string(userID))
	})
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_password.persist", err)
		return err
	}
	uc.logger.Info(ctx, "user password changed", "operation", "user.update_password")
	return nil
}

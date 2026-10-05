package usecase

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UpdateUserPasswordUseCase struct {
	repo   UserRepository
	logger logging.Logger
}

func NewUpdateUserPasswordUseCase(repo UserRepository, logger logging.Logger) *UpdateUserPasswordUseCase {
	return &UpdateUserPasswordUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *UpdateUserPasswordUseCase) Execute(ctx context.Context, userID domain.UserID, newPassword string) error {
	p, err := domain.NewPassword(newPassword)
	if err != nil {
		return err
	}

	userRecord, err := uc.repo.Get(ctx, userID)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_password.load", err)
		return err
	}
	user, err := restoreUser(userRecord)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_password.restore", err)
		return err
	}

	newUser := user.UpdatePassword(p)
	updated, err := uc.repo.Update(ctx, newUser)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_password.persist", err)
		return err
	}
	if updated.Password != newUser.Password.String() {
		uc.logger.Error(ctx, "password update verification failed", "operation", "user.update_password")
		return ErrPasswordUpdateFailed
	}
	uc.logger.Info(ctx, "user password changed", "operation", "user.update_password")
	return nil
}

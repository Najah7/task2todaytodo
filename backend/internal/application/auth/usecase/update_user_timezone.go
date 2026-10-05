package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UpdateUserTimezoneUseCase struct {
	repo   UserRepository
	logger logging.Logger
}

func NewUpdateUserTimezoneUseCase(repo UserRepository, logger logging.Logger) *UpdateUserTimezoneUseCase {
	return &UpdateUserTimezoneUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *UpdateUserTimezoneUseCase) Execute(ctx context.Context, userID domain.UserID, timezone string) (dao.User, error) {
	userRecord, err := uc.repo.Get(ctx, userID)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_timezone.load", err)
		return dao.User{}, err
	}
	user, err := restoreUser(userRecord)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_timezone.restore", err)
		return dao.User{}, err
	}
	newTimezone, err := domain.NewUserTimezone(timezone)
	if err != nil {
		return dao.User{}, err
	}
	updatedUser := user.UpdateTimezone(newTimezone)
	if err := uc.repo.UpdateTimezone(ctx, userID, updatedUser.Timezone); err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_timezone.persist", err)
		return dao.User{}, err
	}

	userRecord.Timezone = updatedUser.Timezone.String()
	return userWithoutPassword(userRecord), nil
}

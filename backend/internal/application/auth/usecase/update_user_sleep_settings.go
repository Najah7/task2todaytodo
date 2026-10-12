package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var ErrInvalidSleepTime = errors.New("sleep times must use HH:mm format")

type UpdateUserSleepSettingsUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewUpdateUserSleepSettingsUseCase(uow UOW, logger telemetry.Logger) *UpdateUserSleepSettingsUseCase {
	return &UpdateUserSleepSettingsUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *UpdateUserSleepSettingsUseCase) Execute(ctx context.Context, userID domain.UserID, wakeUpTime, bedtime string) (dao.User, error) {
	if _, err := time.Parse("15:04", wakeUpTime); err != nil {
		return dao.User{}, ErrInvalidSleepTime
	}
	if _, err := time.Parse("15:04", bedtime); err != nil {
		return dao.User{}, ErrInvalidSleepTime
	}
	var result dao.User
	err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		availability := repos.Availability()
		if availability == nil {
			return ErrAvailabilityValidationUnavailable
		}
		if err := availability.LockUser(ctx, string(userID)); err != nil {
			return err
		}
		user, err := repos.Users().Get(ctx, userID)
		if err != nil {
			return err
		}
		if err := availability.ValidateUserProfile(ctx, string(userID), user.Timezone, wakeUpTime, bedtime); err != nil {
			return err
		}
		if err := repos.Users().UpdateSleepSettings(ctx, userID, wakeUpTime, bedtime); err != nil {
			return err
		}
		user.WakeUpTime, user.Bedtime = wakeUpTime, bedtime
		result = userWithoutPassword(user)
		return nil
	})
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_sleep_settings.persist", err)
		return dao.User{}, err
	}
	return result, nil
}

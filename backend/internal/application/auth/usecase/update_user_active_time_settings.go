package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var ErrInvalidActiveTimeTime = errors.New("active_time times must use HH:mm format")

type UpdateUserActiveTimeSettingsUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewUpdateUserActiveTimeSettingsUseCase(uow UOW, logger telemetry.Logger) *UpdateUserActiveTimeSettingsUseCase {
	return &UpdateUserActiveTimeSettingsUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *UpdateUserActiveTimeSettingsUseCase) Execute(ctx context.Context, userID domain.UserID, activeStartTime, activeEndTime string) (dao.User, error) {
	if _, err := time.Parse("15:04", activeStartTime); err != nil {
		return dao.User{}, ErrInvalidActiveTimeTime
	}
	if _, err := time.Parse("15:04", activeEndTime); err != nil {
		return dao.User{}, ErrInvalidActiveTimeTime
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
		if err := availability.ValidateUserProfile(ctx, string(userID), user.Timezone, activeStartTime, activeEndTime); err != nil {
			return err
		}
		if err := repos.Users().UpdateActiveTimeSettings(ctx, userID, activeStartTime, activeEndTime); err != nil {
			return err
		}
		user.ActiveStartTime, user.ActiveEndTime = activeStartTime, activeEndTime
		result = userWithoutPassword(user)
		return nil
	})
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_active_time_settings.persist", err)
		return dao.User{}, err
	}
	return result, nil
}

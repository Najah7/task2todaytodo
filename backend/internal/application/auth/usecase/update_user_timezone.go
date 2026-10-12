package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type UpdateUserTimezoneUseCase struct {
	repo   UserRepository
	uow    UOW
	logger telemetry.Logger
}

func NewUpdateUserTimezoneUseCase(repo UserRepository, logger telemetry.Logger, uow ...UOW) *UpdateUserTimezoneUseCase {
	uc := &UpdateUserTimezoneUseCase{repo: repo, logger: telemetry.OrNop(logger)}
	if len(uow) > 0 {
		uc.uow = uow[0]
	}
	return uc
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
	allowed, err := uc.repo.TimezoneExists(ctx, timezone)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_timezone.validate", err)
		return dao.User{}, err
	}
	if !allowed {
		return dao.User{}, domain.ErrInvalidUserTimezone
	}
	updatedUser := user.UpdateTimezone(newTimezone)
	persist := func(repo UserRepository, availability UserAvailabilityValidator) error {
		if uc.uow != nil {
			if availability == nil {
				return ErrAvailabilityValidationUnavailable
			}
			if err := availability.LockUser(ctx, string(userID)); err != nil {
				return err
			}
		}
		current, err := repo.Get(ctx, userID)
		if err != nil {
			return err
		}
		if uc.uow != nil && availability == nil {
			return ErrAvailabilityValidationUnavailable
		}
		if availability != nil {
			if err := availability.ValidateUserProfile(ctx, string(userID), updatedUser.Timezone.String(), current.WakeUpTime, current.Bedtime); err != nil {
				return err
			}
		}
		if err := repo.UpdateTimezone(ctx, userID, updatedUser.Timezone); err != nil {
			return err
		}
		userRecord = current
		userRecord.Timezone = updatedUser.Timezone.String()
		return nil
	}
	var persistErr error
	if uc.uow != nil {
		persistErr = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
			return persist(repos.Users(), repos.Availability())
		})
	} else {
		persistErr = persist(uc.repo, nil)
	}
	if persistErr != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_timezone.persist", persistErr)
		return dao.User{}, persistErr
	}

	return userWithoutPassword(userRecord), nil
}

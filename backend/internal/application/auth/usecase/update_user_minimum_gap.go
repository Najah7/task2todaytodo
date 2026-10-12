package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var ErrInvalidMinimumGapMinutes = errors.New("minimum gap minutes must be between 0 and 1440")

type UpdateUserMinimumGapUseCase struct {
	repo   UserRepository
	logger telemetry.Logger
}

func NewUpdateUserMinimumGapUseCase(repo UserRepository, logger telemetry.Logger) *UpdateUserMinimumGapUseCase {
	return &UpdateUserMinimumGapUseCase{repo: repo, logger: telemetry.OrNop(logger)}
}

func (uc *UpdateUserMinimumGapUseCase) Execute(ctx context.Context, userID domain.UserID, minutes int) (dao.User, error) {
	if minutes < 0 || minutes > 1440 {
		return dao.User{}, ErrInvalidMinimumGapMinutes
	}
	if err := uc.repo.UpdateMinimumGapMinutes(ctx, userID, minutes); err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_minimum_gap", err)
		return dao.User{}, err
	}
	user, err := uc.repo.Get(ctx, userID)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_minimum_gap", err)
		return dao.User{}, err
	}
	return userWithoutPassword(user), nil
}

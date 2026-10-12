package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type UpdateUserEmailUseCase struct {
	repo   UserRepository
	logger telemetry.Logger
}

func NewUpdateUserEmailUseCase(repo UserRepository, logger telemetry.Logger) *UpdateUserEmailUseCase {
	return &UpdateUserEmailUseCase{repo: repo, logger: telemetry.OrNop(logger)}
}

func (uc *UpdateUserEmailUseCase) Execute(ctx context.Context, userID domain.UserID, newEmail string) (dao.User, error) {
	e, err := domain.NewEmail(newEmail)
	if err != nil {
		return dao.User{}, err
	}

	userWithEmail, err := uc.repo.GetByEmail(ctx, e.String())
	if err == nil && userWithEmail.ID != "" && userWithEmail.ID != string(userID) {
		return dao.User{}, ErrUserEmailAlreadyExists
	} else if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_email.check_email", err)
	}

	updated, err := uc.repo.UpdateEmail(ctx, userID, e)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_email.persist", err)
		return dao.User{}, err
	}
	return userWithoutPassword(updated), nil
}

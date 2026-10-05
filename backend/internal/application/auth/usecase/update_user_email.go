package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UpdateUserEmailUseCase struct {
	repo   UserRepository
	logger logging.Logger
}

func NewUpdateUserEmailUseCase(repo UserRepository, logger logging.Logger) *UpdateUserEmailUseCase {
	return &UpdateUserEmailUseCase{repo: repo, logger: logging.OrNop(logger)}
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

	userRecord, err := uc.repo.Get(ctx, userID)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_email.load", err)
		return dao.User{}, err
	}
	user, err := restoreUser(userRecord)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_email.restore", err)
		return dao.User{}, err
	}

	updated, err := uc.repo.Update(ctx, user.UpdateEmail(e))
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_email.persist", err)
		return dao.User{}, err
	}
	return userWithoutPassword(updated), nil
}

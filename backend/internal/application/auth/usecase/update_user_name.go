package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UpdateUserNameUseCase struct {
	repo   UserRepository
	logger logging.Logger
}

func NewUpdateUserNameUseCase(repo UserRepository, logger logging.Logger) *UpdateUserNameUseCase {
	return &UpdateUserNameUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *UpdateUserNameUseCase) Execute(ctx context.Context, userID domain.UserID, firstName, lastName string) (dao.User, error) {
	userRecord, err := uc.repo.Get(ctx, userID)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_name.load", err)
		return dao.User{}, err
	}

	u, err := restoreUser(userRecord)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_name.restore", err)
		return dao.User{}, err
	}

	newUser, err := u.UpdateName(domain.NewUserName(firstName, lastName))
	if err != nil {
		return dao.User{}, err
	}

	updated, err := uc.repo.Update(ctx, newUser)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.update_name.persist", err)
		return dao.User{}, err
	}
	return userWithoutPassword(updated), nil
}

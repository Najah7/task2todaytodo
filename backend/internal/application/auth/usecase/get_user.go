package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type GetUserUseCase struct {
	repo   UserRepository
	logger logging.Logger
}

func NewGetUserUseCase(repo UserRepository, logger logging.Logger) *GetUserUseCase {
	return &GetUserUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *GetUserUseCase) Execute(ctx context.Context, userID domain.UserID) (dao.User, error) {
	user, err := uc.repo.Get(ctx, userID)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.get", err)
		return dao.User{}, err
	}
	return userWithoutPassword(user), nil
}

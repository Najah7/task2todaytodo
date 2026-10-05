package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type GetUserByEmailUseCase struct {
	repo   UserRepository
	logger logging.Logger
}

func NewGetUserByEmailUseCase(repo UserRepository, logger logging.Logger) *GetUserByEmailUseCase {
	return &GetUserByEmailUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *GetUserByEmailUseCase) Execute(ctx context.Context, email string) (dao.User, error) {
	validatedEmail, err := domain.NewEmail(email)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "user.get_by_email", err)
		return dao.User{}, err
	}

	user, err := uc.repo.GetByEmail(ctx, validatedEmail.String())
	if err != nil {
		return dao.User{}, err
	}
	return userWithoutPassword(user), nil
}

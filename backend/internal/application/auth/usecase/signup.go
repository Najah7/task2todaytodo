package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type SignUpUseCase struct {
	repo   UserRepository
	logger logging.Logger
}

func NewSignUpUseCase(repo UserRepository, logger logging.Logger) *SignUpUseCase {
	return &SignUpUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *SignUpUseCase) Execute(ctx context.Context, ID, email, password string) (dao.User, error) {

	e, err := domain.NewEmail(email)
	if err != nil {
		return dao.User{}, err
	}

	p, err := domain.NewPassword(password)
	if err != nil {
		return dao.User{}, err
	}

	userID := domain.UserID(ID)
	newUser := domain.NewUser(userID, e, p, domain.NewUserName("", ""))

	u, err := uc.repo.GetByEmail(ctx, e.String())
	if err == nil && u.ID != "" {
		return dao.User{}, ErrUserEmailAlreadyExists
	} else if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "signup.check_email", err)
	}

	u, err = uc.repo.Get(ctx, userID)
	if err == nil && u.ID != "" {
		return dao.User{}, ErrUserIDAlreadyExists
	} else if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "signup.check_user_id", err)
	}

	user, err := uc.repo.Create(ctx, newUser)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "signup.create_user", err)
		return dao.User{}, err
	}
	uc.logger.Info(ctx, "user signed up", "operation", "signup")
	return userWithoutPassword(user), nil
}

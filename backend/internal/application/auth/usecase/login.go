package usecase

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type LoginUserUseCase struct {
	repo   UserRepository
	logger telemetry.Logger
}

func NewLoginUserUseCase(repo UserRepository, logger telemetry.Logger) *LoginUserUseCase {
	return &LoginUserUseCase{repo: repo, logger: telemetry.OrNop(logger)}
}

func (uc *LoginUserUseCase) Execute(ctx context.Context, email, password string) (domain.UserID, error) {
	e, err := domain.NewEmail(email)
	if err != nil {
		return "", err
	}

	p, err := domain.NewPassword(password)
	if err != nil {
		return "", err
	}

	userRecord, err := uc.repo.GetByEmail(ctx, e.String())
	if err != nil {
		if ctx.Err() == nil && telemetry.IsNotFound(err) {
			logAuthenticationRejected(uc.logger, ctx, "invalid_credentials")
			return "", ErrInvalidCredentials
		} else {
			logUnexpectedFailure(uc.logger, ctx, "login.load_user", err)
		}
		return "", err
	}
	user, err := restoreUser(userRecord)
	if err != nil {
		logUnexpectedFailure(uc.logger, ctx, "login.restore_user", err)
		return "", err
	}
	if !user.Login(e, p) {
		logAuthenticationRejected(uc.logger, ctx, "invalid_credentials")
		return "", ErrInvalidCredentials
	}
	uc.logger.Info(ctx, "authentication succeeded", "operation", "login")
	return user.ID, nil
}

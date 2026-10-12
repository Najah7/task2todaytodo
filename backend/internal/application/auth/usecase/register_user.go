package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RegisterUserUseCase struct {
	deps authEmailDependencies
}

func NewRegisterUserUseCase(uow UOW, sender AuthLinkSender, frontendURL string, logger telemetry.Logger) *RegisterUserUseCase {
	return &RegisterUserUseCase{deps: newAuthEmailDependencies(uow, sender, frontendURL, logger)}
}

func (uc *RegisterUserUseCase) Execute(ctx context.Context, id, email, password string) error {
	e, err := domain.NewEmail(email)
	if err != nil {
		return ErrInvalidEmail
	}
	p, err := domain.NewPassword(password)
	if err != nil {
		return err
	}
	requestAuthEmailLink(ctx, uc.deps, authEmailRequest{
		email: e, purpose: EmailVerificationPurpose, createUserID: id,
		passwordHash: p.String(), createIfAbsent: true,
	})
	return nil
}

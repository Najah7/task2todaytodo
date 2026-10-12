package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RequestPasswordResetUseCase struct{ deps authEmailDependencies }

func NewRequestPasswordResetUseCase(uow UOW, sender AuthLinkSender, frontendURL string, logger telemetry.Logger) *RequestPasswordResetUseCase {
	return &RequestPasswordResetUseCase{deps: newAuthEmailDependencies(uow, sender, frontendURL, logger)}
}

func (uc *RequestPasswordResetUseCase) Execute(ctx context.Context, email string) error {
	e, err := domain.NewEmail(email)
	if err != nil {
		return ErrInvalidEmail
	}
	requestAuthEmailLink(ctx, uc.deps, authEmailRequest{email: e, purpose: PasswordResetPurpose})
	return nil
}

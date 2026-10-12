package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ResendVerificationEmailUseCase struct{ deps authEmailDependencies }

func NewResendVerificationEmailUseCase(uow UOW, sender AuthLinkSender, frontendURL string, logger telemetry.Logger) *ResendVerificationEmailUseCase {
	return &ResendVerificationEmailUseCase{deps: newAuthEmailDependencies(uow, sender, frontendURL, logger)}
}

func (uc *ResendVerificationEmailUseCase) Execute(ctx context.Context, email string) error {
	e, err := domain.NewEmail(email)
	if err != nil {
		return ErrInvalidEmail
	}
	requestAuthEmailLink(ctx, uc.deps, authEmailRequest{email: e, purpose: EmailVerificationPurpose})
	return nil
}

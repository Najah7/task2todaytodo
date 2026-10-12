package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ConfirmPasswordResetUseCase struct {
	uow    UOW
	logger telemetry.Logger
	now    func() time.Time
}

func NewConfirmPasswordResetUseCase(uow UOW, logger telemetry.Logger) *ConfirmPasswordResetUseCase {
	return &ConfirmPasswordResetUseCase{uow: uow, logger: telemetry.OrNop(logger), now: time.Now}
}

func (uc *ConfirmPasswordResetUseCase) Execute(ctx context.Context, rawToken, newPassword string) error {
	hash, err := parseAuthLinkToken(rawToken)
	if err != nil {
		return ErrInvalidAuthLink
	}
	err = uc.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		token, err := repositories.EmailAuth().GetAuthLinkToken(ctx, hash)
		if telemetry.IsNotFound(err) {
			return ErrInvalidAuthLink
		}
		if err != nil {
			return err
		}
		if !validAuthLink(token, PasswordResetPurpose, uc.now()) {
			return ErrInvalidAuthLink
		}
		return nil
	})
	if err != nil {
		if err == ErrInvalidAuthLink {
			return err
		}
		logUnexpectedFailure(uc.logger, ctx, "password_reset.preflight", err)
		return err
	}
	password, err := domain.NewPassword(newPassword)
	if err != nil {
		return err
	}
	err = uc.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		authRepo := repositories.EmailAuth()
		initial, err := authRepo.GetAuthLinkToken(ctx, hash)
		if telemetry.IsNotFound(err) {
			return ErrInvalidAuthLink
		}
		if err != nil {
			return err
		}
		user, err := authRepo.GetUserByIDForAuth(ctx, initial.UserID)
		if telemetry.IsNotFound(err) {
			return ErrInvalidAuthLink
		}
		if err != nil {
			return err
		}
		token, err := authRepo.GetAuthLinkTokenForUpdate(ctx, hash)
		if telemetry.IsNotFound(err) {
			return ErrInvalidAuthLink
		}
		if err != nil {
			return err
		}
		if !validAuthLink(token, PasswordResetPurpose, uc.now()) {
			return ErrInvalidAuthLink
		}
		consumed, err := authRepo.ConsumeAuthLinkToken(ctx, hash)
		if err != nil {
			return err
		}
		if !consumed {
			return ErrInvalidAuthLink
		}
		if err := authRepo.UpdateUserPasswordForReset(ctx, user.ID, password.String()); err != nil {
			return err
		}
		if err := authRepo.RevokeAllUserPersonalAccessTokens(ctx, user.ID); err != nil {
			return err
		}
		return authRepo.RevokeUserAuthLinkTokens(ctx, user.ID)
	})
	if err != nil {
		if err != ErrInvalidAuthLink {
			logUnexpectedFailure(uc.logger, ctx, "password_reset.confirm", err)
			return err
		}
		return ErrInvalidAuthLink
	}
	uc.logger.Info(ctx, "password reset completed", "operation", "password_reset.confirm")
	return nil
}

package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ConfirmEmailVerificationUseCase struct {
	uow    UOW
	logger telemetry.Logger
	now    func() time.Time
}

func NewConfirmEmailVerificationUseCase(uow UOW, logger telemetry.Logger) *ConfirmEmailVerificationUseCase {
	return &ConfirmEmailVerificationUseCase{uow: uow, logger: telemetry.OrNop(logger), now: time.Now}
}

func (uc *ConfirmEmailVerificationUseCase) Execute(ctx context.Context, rawToken, password string) (dao.PersonalAccessToken, error) {
	hash, err := parseAuthLinkToken(rawToken)
	if err != nil {
		return dao.PersonalAccessToken{}, ErrInvalidAuthLink
	}
	var issued dao.PersonalAccessToken
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
		if !validAuthLink(token, EmailVerificationPurpose, uc.now()) {
			return ErrInvalidAuthLink
		}
		stored, err := domain.NewHashedPassword(user.PasswordHash)
		if err != nil {
			return err
		}
		if !stored.Verify(password) {
			return ErrInvalidAuthLink
		}
		consumed, err := authRepo.ConsumeAuthLinkToken(ctx, hash)
		if err != nil {
			return err
		}
		if !consumed {
			return ErrInvalidAuthLink
		}
		if err := authRepo.MarkUserEmailVerified(ctx, user.ID); err != nil {
			return err
		}
		if err := authRepo.RevokeActiveUserAuthLinkTokensForPurpose(ctx, user.ID, EmailVerificationPurpose); err != nil {
			return err
		}
		accessToken, err := domain.NewPersonalAccessToken(domain.UserID(user.ID))
		if err != nil {
			return err
		}
		issued, err = repositories.PersonalAccessTokens().CreateForAuth(ctx, accessToken, user.AuthVersion)
		return err
	})
	if err != nil {
		if err != ErrInvalidAuthLink {
			logUnexpectedFailure(uc.logger, ctx, "email_verification.confirm", err)
		}
		return dao.PersonalAccessToken{}, err
	}
	uc.logger.Info(ctx, "email verification completed", "operation", "email_verification.confirm")
	return issued, nil
}

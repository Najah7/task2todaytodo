package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

// LoginAndIssueTokenUseCase serializes credential verification and PAT creation
// with password reset by locking the user row for the whole transaction.
type LoginAndIssueTokenUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewLoginAndIssueTokenUseCase(uow UOW, logger telemetry.Logger) *LoginAndIssueTokenUseCase {
	return &LoginAndIssueTokenUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *LoginAndIssueTokenUseCase) Execute(ctx context.Context, email, password string) (dao.PersonalAccessToken, error) {
	e, err := domain.NewEmail(email)
	if err != nil {
		logAuthenticationRejected(uc.logger, ctx, "invalid_credentials")
		return dao.PersonalAccessToken{}, ErrInvalidCredentials
	}
	var issued dao.PersonalAccessToken
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		userRecord, err := repos.EmailAuth().GetUserByEmailForAuth(ctx, e.String())
		if err != nil {
			if ctx.Err() == nil && telemetry.IsNotFound(err) {
				logAuthenticationRejected(uc.logger, ctx, "invalid_credentials")
				return ErrInvalidCredentials
			}
			logUnexpectedFailure(uc.logger, ctx, "login.load_user", err)
			return err
		}
		user, err := restoreUser(userRecord)
		if err != nil {
			logUnexpectedFailure(uc.logger, ctx, "login.restore_user", err)
			return err
		}
		if !user.Login(e, password) {
			logAuthenticationRejected(uc.logger, ctx, "invalid_credentials")
			return ErrInvalidCredentials
		}
		if userRecord.EmailVerifiedAt == 0 {
			logAuthenticationRejected(uc.logger, ctx, "email_unverified")
			return ErrEmailUnverified
		}
		token, err := domain.NewPersonalAccessToken(user.ID)
		if err != nil {
			return err
		}
		issued, err = repos.PersonalAccessTokens().CreateForAuth(ctx, token, userRecord.AuthVersion)
		if err != nil {
			logUnexpectedFailure(uc.logger, ctx, "personal_access_token.create", err)
			return err
		}
		return nil
	})
	if err != nil {
		return dao.PersonalAccessToken{}, err
	}
	uc.logger.Info(ctx, "authentication succeeded", "operation", "login")
	return issued, nil
}

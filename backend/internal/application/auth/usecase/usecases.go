package usecase

import "github.com/Najah7/task2todaytodo/internal/telemetry"

type UserUseCases struct {
	Get                 *GetUserUseCase
	GetByEmail          *GetUserByEmailUseCase
	UpdateName          *UpdateUserNameUseCase
	UpdateEmail         *UpdateUserEmailUseCase
	UpdatePassword      *UpdateUserPasswordUseCase
	UpdateTimezone      *UpdateUserTimezoneUseCase
	UpdateSleepSettings *UpdateUserSleepSettingsUseCase
	UpdateMinimumGap    *UpdateUserMinimumGapUseCase
}

type EmailAuthUseCases struct {
	RegisterUser         *RegisterUserUseCase
	ResendVerification   *ResendVerificationEmailUseCase
	ConfirmVerification  *ConfirmEmailVerificationUseCase
	RequestPasswordReset *RequestPasswordResetUseCase
	ConfirmPasswordReset *ConfirmPasswordResetUseCase
}

func NewEmailAuthUseCases(uow UOW, sender AuthLinkSender, frontendURL string, logger telemetry.Logger) EmailAuthUseCases {
	return EmailAuthUseCases{
		RegisterUser:         NewRegisterUserUseCase(uow, sender, frontendURL, logger),
		ResendVerification:   NewResendVerificationEmailUseCase(uow, sender, frontendURL, logger),
		ConfirmVerification:  NewConfirmEmailVerificationUseCase(uow, logger),
		RequestPasswordReset: NewRequestPasswordResetUseCase(uow, sender, frontendURL, logger),
		ConfirmPasswordReset: NewConfirmPasswordResetUseCase(uow, logger),
	}
}

type PersonalAccessTokenUseCases struct {
	Authenticate  *AuthenticateUseCase
	LoginAndIssue *LoginAndIssueTokenUseCase
	Revoke        *RevokePersonalAccessTokenUseCase
}

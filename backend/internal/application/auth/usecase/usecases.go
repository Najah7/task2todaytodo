package usecase

type UserUseCases struct {
	Get                 *GetUserUseCase
	GetByEmail          *GetUserByEmailUseCase
	Create              *SignUpUseCase
	UpdateName          *UpdateUserNameUseCase
	UpdateEmail         *UpdateUserEmailUseCase
	UpdatePassword      *UpdateUserPasswordUseCase
	UpdateTimezone      *UpdateUserTimezoneUseCase
	UpdateSleepSettings *UpdateUserSleepSettingsUseCase
	UpdateMinimumGap    *UpdateUserMinimumGapUseCase
}

type PersonalAccessTokenUseCases struct {
	Authenticate *AuthenticateUseCase
	Login        *LoginUserUseCase
	Generate     *GeneratePersonalAccessTokenUseCase
	Revoke       *RevokePersonalAccessTokenUseCase
}

package usecase

type UserUseCases struct {
	Get            *GetUserUseCase
	GetByEmail     *GetUserByEmailUseCase
	Create         *SignUpUseCase
	UpdateName     *UpdateUserNameUseCase
	UpdateEmail    *UpdateUserEmailUseCase
	UpdatePassword *UpdateUserPasswordUseCase
	UpdateTimezone *UpdateUserTimezoneUseCase
}

type AccessTokenUseCases struct {
	Authenticate *AuthenticateUseCase
	Login        *LoginUserUseCase
	Generate     *GenerateAccessTokenUseCase
	Revoke       *RevokeAccessTokenUseCase
}

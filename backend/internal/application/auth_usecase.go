package application

import (
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

func NewUserUseCases(authStore AuthStore, logger logging.Logger) authusecase.UserUseCases {
	return authusecase.UserUseCases{
		Get:            authusecase.NewGetUserUseCase(authStore.Users, logger),
		GetByEmail:     authusecase.NewGetUserByEmailUseCase(authStore.Users, logger),
		Create:         authusecase.NewSignUpUseCase(authStore.Users, logger),
		UpdateName:     authusecase.NewUpdateUserNameUseCase(authStore.Users, logger),
		UpdateEmail:    authusecase.NewUpdateUserEmailUseCase(authStore.Users, logger),
		UpdatePassword: authusecase.NewUpdateUserPasswordUseCase(authStore.Users, logger),
		UpdateTimezone: authusecase.NewUpdateUserTimezoneUseCase(authStore.Users, logger),
	}
}

func NewPersonalAccessTokenUseCases(authStore AuthStore, logger logging.Logger) authusecase.PersonalAccessTokenUseCases {
	return authusecase.PersonalAccessTokenUseCases{
		Authenticate: authusecase.NewAuthenticateUseCase(authStore.PersonalAccessTokens, logger),
		Login:        authusecase.NewLoginUserUseCase(authStore.Users, logger),
		Generate:     authusecase.NewGeneratePersonalAccessTokenUseCase(authStore.PersonalAccessTokens, logger),
		Revoke:       authusecase.NewRevokePersonalAccessTokenUseCase(authStore.PersonalAccessTokens, logger),
	}
}

func NewRoleUseCases(authStore AuthStore, logger logging.Logger) authusecase.RoleUseCases {
	return authusecase.RoleUseCases{
		List:        authusecase.NewListRolesUseCase(authStore.Roles, logger),
		Permissions: authusecase.NewListPermissionsUseCase(authStore.Roles, logger),
	}
}

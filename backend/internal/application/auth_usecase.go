package application

import (
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

func NewUserUseCases(authStore AuthStore, uow authusecase.UOW, logger telemetry.Logger) authusecase.UserUseCases {
	return authusecase.UserUseCases{
		Get:                 authusecase.NewGetUserUseCase(authStore.Users, logger),
		GetByEmail:          authusecase.NewGetUserByEmailUseCase(authStore.Users, logger),
		Create:              authusecase.NewSignUpUseCase(authStore.Users, logger),
		UpdateName:          authusecase.NewUpdateUserNameUseCase(authStore.Users, logger),
		UpdateEmail:         authusecase.NewUpdateUserEmailUseCase(authStore.Users, logger),
		UpdatePassword:      authusecase.NewUpdateUserPasswordUseCase(authStore.Users, logger),
		UpdateTimezone:      authusecase.NewUpdateUserTimezoneUseCase(authStore.Users, logger, uow),
		UpdateSleepSettings: authusecase.NewUpdateUserSleepSettingsUseCase(uow, logger),
	}
}

func NewPersonalAccessTokenUseCases(authStore AuthStore, logger telemetry.Logger) authusecase.PersonalAccessTokenUseCases {
	return authusecase.PersonalAccessTokenUseCases{
		Authenticate: authusecase.NewAuthenticateUseCase(authStore.PersonalAccessTokens, logger),
		Login:        authusecase.NewLoginUserUseCase(authStore.Users, logger),
		Generate:     authusecase.NewGeneratePersonalAccessTokenUseCase(authStore.PersonalAccessTokens, logger),
		Revoke:       authusecase.NewRevokePersonalAccessTokenUseCase(authStore.PersonalAccessTokens, logger),
	}
}

func NewRoleUseCases(authStore AuthStore, logger telemetry.Logger) authusecase.RoleUseCases {
	return authusecase.RoleUseCases{
		List:        authusecase.NewListRolesUseCase(authStore.Roles, logger),
		Permissions: authusecase.NewListPermissionsUseCase(authStore.Roles, logger),
	}
}

func NewTimezoneUseCases(authStore AuthStore, logger telemetry.Logger) authusecase.TimezoneUseCases {
	return authusecase.TimezoneUseCases{
		List: authusecase.NewListTimezonesUseCase(authStore.Users, logger),
	}
}

package application

import (
	"context"

	adminusecase "github.com/Najah7/task2todaytodo/internal/application/admin/usecase"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type UseCase struct {
	User                authusecase.UserUseCases
	PersonalAccessToken authusecase.PersonalAccessTokenUseCases
	Role                authusecase.RoleUseCases
	Project             *projectusecase.UseCases
	Task                taskusecase.TaskUseCases
	ActionItem          taskusecase.ActionItemUseCases
	Schedule            scheduleusecase.ScheduleUseCases
	Timezone            authusecase.TimezoneUseCases
	Admin               adminusecase.UseCases
}

func NewUseCase(authStore AuthStore, projectStore ProjectStore, taskStore TaskStore, adminStore AdminStore, projectUOW projectusecase.UnitOfWork, taskUOW taskusecase.UOW, scheduleUOW scheduleusecase.UOW, ID shared.ID, logger telemetry.Logger) (UseCase, func(context.Context) error) {
	schedules := NewScheduleUseCases(scheduleUOW, authStore, ID, logger)
	admin, closeAdmin := NewAdminUseCases(adminStore, scheduleBackfillAdapter{usecase: schedules.Backfill}, ID, logger)
	return UseCase{
		User:                NewUserUseCases(authStore, logger),
		PersonalAccessToken: NewPersonalAccessTokenUseCases(authStore, logger),
		Role:                NewRoleUseCases(authStore, logger),
		Timezone:            authusecase.TimezoneUseCases{List: authusecase.NewListTimezonesUseCase(authStore.Users, logger)},
		Project:             NewProjectUseCases(projectStore, projectUOW, logger, newTaskProjectProgressReader(projectStore.Projects, taskStore.ActionItems)),
		Task:                NewTaskUseCases(taskStore, taskUOW, authStore, ID, logger),
		ActionItem:          NewActionItemUseCases(taskUOW, logger),
		Schedule:            schedules,
		Admin:               admin,
	}, closeAdmin
}

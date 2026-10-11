package application

import (
	"context"

	adminusecase "github.com/Najah7/task2todaytodo/internal/application/admin/usecase"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	inboxusecase "github.com/Najah7/task2todaytodo/internal/application/inbox/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
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
	Inbox               inboxusecase.InboxUseCases
}

func NewUseCase(store Store, uow UOW, id IDIssuer, logger telemetry.Logger) (UseCase, func(context.Context) error) {
	schedules := NewScheduleUseCases(uow.Schedule, store.Auth, id, logger)
	admin, closeAdmin := NewAdminUseCases(store.Admin, schedules.Backfill, id, logger)
	return UseCase{
		User:                NewUserUseCases(store.Auth, logger),
		PersonalAccessToken: NewPersonalAccessTokenUseCases(store.Auth, logger),
		Role:                NewRoleUseCases(store.Auth, logger),
		Timezone:            NewTimezoneUseCases(store.Auth, logger),
		Project:             NewProjectUseCases(store.Project, uow.Project, logger),
		Task:                NewTaskUseCases(store.Task, uow.Task, store.Auth, id, logger),
		ActionItem:          NewActionItemUseCases(uow.Task, logger),
		Schedule:            schedules,
		Admin:               admin,
		Inbox:               NewInboxUseCases(store.Auth, store.Task, schedules, store.Inbox, logger),
	}, closeAdmin
}

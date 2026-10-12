package application

import (
	"context"
	"time"

	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
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
	Admin               AdminUseCases
	Inbox               inboxusecase.InboxUseCases
	Allocation          allocationusecase.UseCases
}

func NewUseCase(store Store, uow UOW, id IDIssuer, logger telemetry.Logger) (UseCase, func(context.Context) error) {
	schedules := NewScheduleUseCases(uow.Schedule, store.Auth, id, logger)
	prefill := allocationusecase.NewPrefillAllocationsExecutor(store.Allocation, uow.Allocation, id.NewULID)
	admin, closeAdmin := NewAdminUseCases(store.Admin, schedules.Backfill, id, logger, prefill)
	return UseCase{
		User:                NewUserUseCases(store.Auth, uow.Auth, logger),
		PersonalAccessToken: NewPersonalAccessTokenUseCases(store.Auth, logger),
		Role:                NewRoleUseCases(store.Auth, logger),
		Timezone:            NewTimezoneUseCases(store.Auth, logger),
		Project:             NewProjectUseCases(store.Project, uow.Project, logger),
		Task:                NewTaskUseCases(store.Task, uow.Task, store.Auth, id, logger),
		ActionItem:          NewActionItemUseCases(uow.Task, logger),
		Schedule:            schedules,
		Admin:               admin,
		Inbox:               NewInboxUseCases(store.Auth, store.Task, schedules, store.Inbox, logger),
		Allocation:          allocationusecase.NewUseCases(store.Allocation, uow.Allocation, id.NewULID, time.Now),
	}, closeAdmin
}

package application

import (
	"context"
	"time"

	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	inboxusecase "github.com/Najah7/task2todaytodo/internal/application/inbox/usecase"
	metricusecase "github.com/Najah7/task2todaytodo/internal/application/metric/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type UseCase struct {
	User                    authusecase.UserUseCases
	EmailAuth               authusecase.EmailAuthUseCases
	PersonalAccessToken     authusecase.PersonalAccessTokenUseCases
	Role                    authusecase.RoleUseCases
	Project                 *projectusecase.UseCases
	AcceptProjectInvitation *projectusecase.AcceptProjectInvitationUseCase
	Task                    taskusecase.TaskUseCases
	ActionItem              taskusecase.ActionItemUseCases
	Schedule                scheduleusecase.ScheduleUseCases
	Timezone                authusecase.TimezoneUseCases
	Admin                   AdminUseCases
	Inbox                   inboxusecase.InboxUseCases
	Allocation              allocationusecase.UseCases
	Todo                    todousecase.UseCases
	Metric                  metricusecase.UseCases
}

type AuthEmailSettings struct {
	Sender           authusecase.AuthLinkSender
	InvitationSender projectusecase.ProjectInvitationSender
	FrontendURL      string
}

func NewUseCase(store Store, uow UOW, id IDIssuer, logger telemetry.Logger, emailSettings ...AuthEmailSettings) (UseCase, func(context.Context) error) {
	var email AuthEmailSettings
	if len(emailSettings) > 0 {
		email = emailSettings[0]
	}
	schedules := NewScheduleUseCases(uow.Schedule, store.Auth, id, logger, uow.ScheduleRead)
	prefill := allocationusecase.NewPrefillAllocationsExecutor(store.Allocation, uow.Allocation, id.NewULID)
	admin, closeAdmin := NewAdminUseCases(store.Admin, schedules.Backfill, id, logger, prefill)
	return UseCase{
		User:                    NewUserUseCases(store.Auth, uow.Auth, uow.AuthAccount, logger),
		EmailAuth:               authusecase.NewEmailAuthUseCases(uow.AuthAccount, email.Sender, email.FrontendURL, logger),
		PersonalAccessToken:     NewPersonalAccessTokenUseCases(store.Auth, uow.AuthAccount, logger),
		Role:                    NewRoleUseCases(store.Auth, logger),
		Timezone:                NewTimezoneUseCases(store.Auth, logger),
		Project:                 NewProjectUseCases(store.Project, uow.Project, logger, email.InvitationSender, email.FrontendURL, uow.ProjectRead),
		AcceptProjectInvitation: projectusecase.NewAcceptProjectInvitationUseCase(uow.ProjectInvitationAcceptance, logger),
		Task:                    NewTaskUseCases(store.Task, uow.Task, store.Auth, id, logger, uow.TaskRead),
		ActionItem:              NewActionItemUseCases(uow.Task, logger, uow.TaskRead),
		Schedule:                schedules,
		Admin:                   admin,
		Inbox:                   NewInboxUseCases(store.Auth, store.Task, schedules, store.Inbox, logger),
		Allocation:              allocationusecase.NewUseCases(store.Allocation, uow.Allocation, id.NewULID, time.Now, uow.AllocationRead),
		Todo:                    todousecase.NewUseCases(uow.Todo, id.NewULID, logger),
		Metric: metricusecase.UseCases{
			TodoCompletion: metricusecase.NewTodoCompletionUseCase(store.Metric),
			OverdueWork:    metricusecase.NewOverdueWorkUseCase(store.Metric),
		},
	}, closeAdmin
}

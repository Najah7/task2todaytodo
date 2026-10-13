package application

import (
	"context"
	"time"

	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	billingrepository "github.com/Najah7/task2todaytodo/internal/application/billing/repository"
	billingusecase "github.com/Najah7/task2todaytodo/internal/application/billing/usecase"
	inboxusecase "github.com/Najah7/task2todaytodo/internal/application/inbox/usecase"
	metricusecase "github.com/Najah7/task2todaytodo/internal/application/metric/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	stripeadapter "github.com/Najah7/task2todaytodo/internal/port/adapter/stripe"
	"github.com/Najah7/task2todaytodo/internal/settings"
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
	Billing                 billingusecase.UseCases
}

type AuthEmailSettings struct {
	Sender           authusecase.AuthLinkSender
	InvitationSender projectusecase.ProjectInvitationSender
	FrontendURL      string
}

func NewUseCase(store Store, uow UOW, id IDIssuer, logger telemetry.Logger, emailSettings ...AuthEmailSettings) (UseCase, func(context.Context) error) {
	return NewUseCaseWithBilling(store, uow, id, logger, settings.BillingSettings{}, emailSettings...)
}

func NewUseCaseWithBilling(store Store, uow UOW, id IDIssuer, logger telemetry.Logger, billingSettings settings.BillingSettings, emailSettings ...AuthEmailSettings) (UseCase, func(context.Context) error) {
	var email AuthEmailSettings
	if len(emailSettings) > 0 {
		email = emailSettings[0]
	}
	schedules := NewScheduleUseCases(uow.Schedule, store.Auth, id, logger, uow.ScheduleRead)
	prefill := allocationusecase.NewPrefillAllocationsExecutor(store.Allocation, uow.Allocation, id.NewULID)
	admin, closeAdmin := NewAdminUseCases(store.Admin, schedules.Backfill, id, logger, prefill)
	stripeClient := stripeadapter.NewClient(billingSettings.StripeAPIKey, billingSettings.ProPriceID, billingSettings.TeamPriceID)
	planAccess := billingusecase.NewGetStatusUseCase(store.Billing, billingSettings.Enabled)
	for _, candidate := range []any{uow.Project, uow.ProjectRead, uow.ProjectInvitationAcceptance, uow.Task, uow.TaskRead, uow.Schedule, uow.ScheduleRead, uow.Todo} {
		if access, ok := candidate.(interface {
			ConfigureProjectWriteAccess(*billingrepository.Repository, bool)
		}); ok {
			access.ConfigureProjectWriteAccess(store.Billing, billingSettings.Enabled)
		}
	}
	return UseCase{
		User:                    NewUserUseCases(store.Auth, uow.Auth, uow.AuthAccount, logger),
		EmailAuth:               authusecase.NewEmailAuthUseCases(uow.AuthAccount, email.Sender, email.FrontendURL, logger),
		PersonalAccessToken:     NewPersonalAccessTokenUseCases(store.Auth, uow.AuthAccount, logger),
		Role:                    NewRoleUseCases(store.Auth, logger),
		Timezone:                NewTimezoneUseCases(store.Auth, logger),
		Project:                 NewProjectUseCasesWithAccess(store.Project, uow.Project, logger, email.InvitationSender, email.FrontendURL, planAccess, uow.ProjectRead),
		AcceptProjectInvitation: projectusecase.NewAcceptProjectInvitationUseCase(uow.ProjectInvitationAcceptance, logger),
		Task:                    NewTaskUseCasesWithAccess(store.Task, uow.Task, store.Auth, id, logger, planAccess, uow.TaskRead),
		ActionItem:              NewActionItemUseCasesWithAccess(uow.Task, logger, planAccess, uow.TaskRead),
		Schedule:                schedules,
		Admin:                   admin,
		Inbox:                   NewInboxUseCases(store.Auth, store.Task, schedules, store.Inbox, logger),
		Allocation:              allocationusecase.NewUseCasesWithAccess(store.Allocation, uow.Allocation, id.NewULID, time.Now, uow.AllocationRead, planAccess),
		Todo:                    todousecase.NewUseCasesWithAccess(uow.Todo, id.NewULID, logger, planAccess),
		Metric: metricusecase.UseCases{
			TodoCompletion: metricusecase.NewTodoCompletionUseCase(store.Metric, planAccess),
			OverdueWork:    metricusecase.NewOverdueWorkUseCase(store.Metric, planAccess),
		},
		Billing: billingusecase.NewUseCases(store.Billing, uow.Billing, stripeClient, billingSettings.Enabled, billingSettings.ProPriceID, billingSettings.TeamPriceID, email.FrontendURL),
	}, closeAdmin
}

package rest

import (
	"fmt"

	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

type Handler struct {
	User                        *UserHandler
	Signup                      *SignupHandler
	EmailAuth                   *EmailAuthHandler
	Login                       *LoginHandler
	PersonalAccessToken         *PersonalAccessTokenHandler
	Project                     *ProjectHandler
	Task                        *TaskHandler
	ActionItem                  *ActionItemHandler
	Schedule                    *ScheduleHandler
	ProjectMember               *ProjectMemberHandler
	ProjectInvitationAcceptance *ProjectInvitationAcceptanceHandler
	Role                        *RoleHandler
	Timezone                    *TimezoneHandler
	Admin                       *AdminHandler
	Inbox                       *InboxHandler
	Allocation                  *AllocationHandler
	Todos                       *TodosHandler
	Metrics                     *MetricsHandler
	Billing                     *BillingHandler
}

func NewHandler(usecases application.UseCase, issueID func() string, pageTokenKey []byte, webhookSecrets ...string) (*Handler, error) {
	pageTokens, err := pagination.NewCodec(pageTokenKey)
	if err != nil {
		return nil, fmt.Errorf("initialize page token codec: %w", err)
	}

	webhookSecret := ""
	if len(webhookSecrets) > 0 {
		webhookSecret = webhookSecrets[0]
	}
	return &Handler{
		User:                        NewUserHandler(usecases.User),
		Signup:                      NewSignupHandler(usecases.EmailAuth.RegisterUser, issueID),
		EmailAuth:                   NewEmailAuthHandler(usecases.EmailAuth),
		Login:                       NewLoginHandler(usecases.PersonalAccessToken),
		PersonalAccessToken:         NewPersonalAccessTokenHandler(usecases.PersonalAccessToken),
		Project:                     NewProjectHandler(usecases.Project, usecases.Task, issueID, pageTokens),
		Task:                        NewTaskHandler(usecases.Task, issueID, pageTokens),
		ActionItem:                  NewActionItemHandler(usecases.ActionItem, issueID, pageTokens),
		Schedule:                    NewScheduleHandler(usecases.Schedule, pageTokens),
		ProjectMember:               NewProjectMemberHandler(usecases.Project, pageTokens, issueID, usecases.Role.List),
		ProjectInvitationAcceptance: NewProjectInvitationAcceptanceHandler(usecases.AcceptProjectInvitation, issueID),
		Role:                        NewRoleHandler(usecases.Role),
		Timezone:                    NewTimezoneHandler(usecases.Timezone),
		Admin:                       NewAdminHandler(usecases.Admin.Job, usecases.Admin.Operation, pageTokens),
		Inbox:                       NewInboxHandler(usecases.Inbox, pageTokens),
		Allocation:                  NewAllocationHandler(usecases.Allocation),
		Todos:                       NewTodosHandler(TodosUseCases{GetToday: usecases.Todo.GetToday, RegenerateToday: usecases.Todo.RegenerateToday, AddActionItem: usecases.Todo.AddActionItem, AddTask: usecases.Todo.AddTask, Reflow: usecases.Todo.Reflow, CompleteFrame: usecases.Todo.CompleteFrame, ReopenFrame: usecases.Todo.ReopenFrame, RemoveActionItem: usecases.Todo.RemoveActionItem, HideSchedule: usecases.Todo.HideSchedule, RestoreSchedule: usecases.Todo.RestoreSchedule}),
		Metrics:                     NewMetricsHandler(usecases.Metric.TodoCompletion, usecases.Metric.OverdueWork),
		Billing:                     NewBillingHandler(usecases.Billing, webhookSecret),
	}, nil
}

package rest

import (
	"fmt"

	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

type Handler struct {
	User                *UserHandler
	Signup              *SignupHandler
	Login               *LoginHandler
	PersonalAccessToken *PersonalAccessTokenHandler
	Project             *ProjectHandler
	Task                *TaskHandler
	ActionItem          *ActionItemHandler
	Schedule            *ScheduleHandler
	ProjectMember       *ProjectMemberHandler
	Role                *RoleHandler
	Timezone            *TimezoneHandler
	Admin               *AdminHandler
	Inbox               *InboxHandler
	Allocation          *AllocationHandler
}

func NewHandler(usecases application.UseCase, issueID func() string, pageTokenKey []byte) (*Handler, error) {
	pageTokens, err := pagination.NewCodec(pageTokenKey)
	if err != nil {
		return nil, fmt.Errorf("initialize page token codec: %w", err)
	}

	return &Handler{
		User:                NewUserHandler(usecases.User),
		Signup:              NewSignupHandler(usecases.User, issueID),
		Login:               NewLoginHandler(usecases.PersonalAccessToken),
		PersonalAccessToken: NewPersonalAccessTokenHandler(usecases.PersonalAccessToken),
		Project:             NewProjectHandler(usecases.Project, usecases.Task, issueID, pageTokens),
		Task:                NewTaskHandler(usecases.Task, issueID, pageTokens),
		ActionItem:          NewActionItemHandler(usecases.ActionItem, issueID, pageTokens),
		Schedule:            NewScheduleHandler(usecases.Schedule, pageTokens),
		ProjectMember:       NewProjectMemberHandler(usecases.Project),
		Role:                NewRoleHandler(usecases.Role),
		Timezone:            NewTimezoneHandler(usecases.Timezone),
		Admin:               NewAdminHandler(usecases.Admin.Job, usecases.Admin.Operation, pageTokens),
		Inbox:               NewInboxHandler(usecases.Inbox, pageTokens),
		Allocation:          NewAllocationHandler(usecases.Allocation),
	}, nil
}

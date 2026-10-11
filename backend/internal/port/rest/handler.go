package rest

import (
	"fmt"

	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
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
}

func NewHandler(usecases application.UseCase, ids shared.ID, pageTokenKey []byte) (*Handler, error) {
	pageTokens, err := pagination.NewCodec(pageTokenKey)
	if err != nil {
		return nil, fmt.Errorf("initialize page token codec: %w", err)
	}

	return &Handler{
		User:                NewUserHandler(usecases.User),
		Signup:              NewSignupHandler(usecases.User, ids),
		Login:               NewLoginHandler(usecases.PersonalAccessToken),
		PersonalAccessToken: NewPersonalAccessTokenHandler(usecases.PersonalAccessToken),
		Project:             NewProjectHandler(usecases.Project, usecases.Task, ids, pageTokens),
		Task:                NewTaskHandler(usecases.Task, ids, pageTokens),
		ActionItem:          NewActionItemHandler(usecases.ActionItem, ids, pageTokens),
		Schedule:            NewScheduleHandler(usecases.Schedule, pageTokens),
		ProjectMember:       NewProjectMemberHandler(usecases.Project),
		Role:                NewRoleHandler(usecases.Role),
		Timezone:            NewTimezoneHandler(usecases.Timezone),
		Admin:               NewAdminHandler(usecases.Admin, pageTokens),
	}, nil
}

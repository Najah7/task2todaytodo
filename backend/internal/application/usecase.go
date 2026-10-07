package application

import (
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	tagusecase "github.com/Najah7/task2todaytodo/internal/application/tag/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UseCase struct {
	User                authusecase.UserUseCases
	PersonalAccessToken authusecase.PersonalAccessTokenUseCases
	Role                authusecase.RoleUseCases
	Project             *projectusecase.UseCases
	Task                taskusecase.TaskUseCases
	TodoItem            taskusecase.TodoItemUseCases
	Schedule            scheduleusecase.ScheduleUseCases
	Tag                 tagusecase.CatalogUseCases
	TaskTag             taskusecase.TaskTagUseCases
}

func NewUseCase(authStore AuthStore, projectStore ProjectStore, taskStore TaskStore, scheduleStore ScheduleStore, tagStore TagStore, projectUOW projectusecase.UnitOfWork, taskUOW taskusecase.UOW, scheduleUOW scheduleusecase.UOW, ID shared.ID, logger logging.Logger) UseCase {
	tags := NewTagUseCases(tagStore, logger)
	schedules := NewScheduleUseCases(scheduleStore, scheduleUOW, authStore, ID, logger)
	return UseCase{
		User:                NewUserUseCases(authStore, logger),
		PersonalAccessToken: NewPersonalAccessTokenUseCases(authStore, logger),
		Role:                NewRoleUseCases(authStore, logger),
		Project:             NewProjectUseCases(projectStore, projectUOW, logger),
		Task:                NewTaskUseCases(taskStore, taskUOW, ID, logger),
		TodoItem:            NewTodoItemUseCases(taskUOW, authStore, ID, logger),
		Schedule:            schedules,
		TaskTag:             NewTaskTagUseCases(taskUOW, logger),
		Tag:                 tags,
	}
}

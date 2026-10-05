package application

import (
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UseCase struct {
	User         authusecase.UserUseCases
	AccessToken  authusecase.AccessTokenUseCases
	Role         authusecase.RoleUseCases
	Project      taskusecase.ProjectUseCases
	Task         taskusecase.TaskUseCases
	TodoItem     taskusecase.TodoItemUseCases
	TaskSchedule taskusecase.TaskScheduleUseCases
	TaskTag      taskusecase.TaskTagUseCases
}

func NewUseCase(authStore AuthStore, taskStore TaskStore, taskUOW taskusecase.UOW, ID shared.ID, logger logging.Logger) UseCase {
	return UseCase{
		User:         NewUserUseCases(authStore, logger),
		AccessToken:  NewAccessTokenUseCases(authStore, logger),
		Role:         NewRoleUseCases(authStore, logger),
		Project:      NewProjectUseCases(taskStore, taskUOW, logger),
		Task:         NewTaskUseCases(taskStore, taskUOW, ID, logger),
		TodoItem:     NewTodoItemUseCases(taskUOW, authStore, ID, logger),
		TaskSchedule: NewTaskScheduleUseCases(taskUOW, authStore, ID, logger),
		TaskTag:      NewTaskTagUseCases(taskStore, taskUOW, logger),
	}
}

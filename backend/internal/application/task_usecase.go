package application

import (
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

func NewProjectUseCases(taskStore TaskStore, taskUOW usecase.UOW, logger logging.Logger) usecase.ProjectUseCases {
	return usecase.ProjectUseCases{
		Create:     usecase.NewCreateProjectUseCase(taskStore.Projects, logger),
		List:       usecase.NewListProjectsUseCase(taskStore.Projects, taskStore.Tasks, logger),
		Get:        usecase.NewGetProjectUseCase(taskStore.Projects, taskStore.Tasks, logger),
		Update:     usecase.NewUpdateProjectUseCase(taskStore.Projects, taskStore.Tasks, logger),
		Delete:     usecase.NewDeleteProjectUseCase(taskUOW, logger),
		CreateTask: usecase.NewCreateTaskInProjectUseCase(taskUOW, logger),
		ListTasks:  usecase.NewListProjectTasksUseCase(taskStore.Projects, taskStore.Tasks, logger),
		AddTask:    usecase.NewAddTaskToProjectUseCase(taskUOW, taskStore.Tasks, logger),
		RemoveTask: usecase.NewRemoveTaskFromProjectUseCase(taskUOW, taskStore.Tasks, logger),
		Members: &usecase.ProjectMemberUseCases{
			ListMembers:  usecase.NewListProjectMembersUseCase(taskUOW, logger),
			UpsertMember: usecase.NewUpsertProjectMemberUseCase(taskUOW, logger),
			DeleteMember: usecase.NewDeleteProjectMemberUseCase(taskUOW, logger),
		},
		Revisions: usecase.NewListProjectRevisionsUseCase(taskStore.Projects, logger),
	}
}

func NewTaskUseCases(taskStore TaskStore, taskUOW usecase.UOW, ID shared.ID, logger logging.Logger) usecase.TaskUseCases {
	return usecase.TaskUseCases{
		Create:        usecase.NewCreateTaskUseCase(taskStore.Tasks, logger),
		List:          usecase.NewListTasksUseCase(taskStore.Tasks, logger),
		Get:           usecase.NewGetTaskUseCase(taskUOW, logger),
		Update:        usecase.NewUpdateTaskUseCase(taskStore.Tasks, taskStore.Tasks, logger),
		Delete:        usecase.NewDeleteTaskUseCase(taskUOW, logger),
		Start:         usecase.NewStartTaskUseCase(taskUOW, logger),
		Hold:          usecase.NewHoldTaskUseCase(taskUOW, logger),
		Wait:          usecase.NewWaitTaskUseCase(taskUOW, logger),
		Complete:      usecase.NewCompleteTaskUseCase(taskUOW, time.Now, logger),
		Reopen:        usecase.NewReopenTaskUseCase(taskUOW, logger),
		Assign:        usecase.NewAssignTaskUseCase(taskUOW, logger),
		ListAssignees: usecase.NewListTaskAssigneesUseCase(taskStore.Tasks, logger),
		Revisions:     usecase.NewListTaskRevisionsUseCase(taskStore.Tasks, logger),
	}
}

func NewTodoItemUseCases(taskUOW usecase.UOW, authStore AuthStore, ID shared.ID, logger logging.Logger) usecase.TodoItemUseCases {
	timezones := userTimezoneReader{users: authStore.Users}
	return usecase.TodoItemUseCases{
		Create:          usecase.NewCreateTodoItemUseCase(taskUOW, timezones, logger),
		List:            usecase.NewListTodoItemsUseCase(taskUOW, logger),
		Update:          usecase.NewUpdateTodoItemUseCase(taskUOW, logger, ID),
		Delete:          usecase.NewDeleteTodoItemUseCase(taskUOW, logger),
		Complete:        usecase.NewCompleteTodoItemUseCase(taskUOW, logger, ID),
		Reopen:          usecase.NewReopenTodoItemUseCase(taskUOW, logger),
		Skip:            usecase.NewSkipTodoItemUseCase(taskUOW, logger),
		Restore:         usecase.NewRestoreTodoItemUseCase(taskUOW, logger),
		Reorder:         usecase.NewReorderTodoItemUseCase(taskUOW, logger, ID),
		UpdateFrequency: usecase.NewUpdateTodoItemFrequencyUseCase(taskUOW, logger),
	}
}

func NewTaskScheduleUseCases(taskUOW usecase.UOW, authStore AuthStore, ID shared.ID, logger logging.Logger) usecase.TaskScheduleUseCases {
	timezones := userTimezoneReader{users: authStore.Users}
	return usecase.TaskScheduleUseCases{
		Create:          usecase.NewCreateTaskScheduleUseCase(taskUOW, timezones, logger),
		List:            usecase.NewListTaskSchedulesUseCase(taskUOW, logger),
		Update:          usecase.NewUpdateTaskScheduleUseCase(taskUOW, logger, ID),
		Delete:          usecase.NewDeleteTaskScheduleUseCase(taskUOW, logger),
		Complete:        usecase.NewCompleteTaskScheduleUseCase(taskUOW, logger, ID),
		Reopen:          usecase.NewReopenTaskScheduleUseCase(taskUOW, logger),
		Skip:            usecase.NewSkipTaskScheduleUseCase(taskUOW, logger),
		Restore:         usecase.NewRestoreTaskScheduleUseCase(taskUOW, logger),
		Reschedule:      usecase.NewRescheduleTaskScheduleUseCase(taskUOW, logger, ID),
		UpdateFrequency: usecase.NewUpdateTaskScheduleFrequencyUseCase(taskUOW, logger),
	}
}

func NewTaskTagUseCases(taskStore TaskStore, taskUOW usecase.UOW, logger logging.Logger) usecase.TaskTagUseCases {
	return usecase.TaskTagUseCases{
		Create:         usecase.NewCreateTaskTagUseCase(taskStore.TaskTags, logger),
		List:           usecase.NewListTaskTagsUseCase(taskStore.TaskTags, logger),
		Rename:         usecase.NewRenameTaskTagUseCase(taskStore.TaskTags, logger),
		Delete:         usecase.NewDeleteTaskTagUseCase(taskStore.TaskTags, logger),
		AddToTask:      usecase.NewAddTagToTaskUseCase(taskUOW, logger),
		RemoveFromTask: usecase.NewRemoveTagFromTaskUseCase(taskUOW, logger),
	}
}

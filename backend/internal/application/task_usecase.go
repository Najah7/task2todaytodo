package application

import (
	"time"

	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	tagusecase "github.com/Najah7/task2todaytodo/internal/application/tag/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

func NewTaskUseCases(taskStore TaskStore, taskUOW usecase.UOW, authStore AuthStore, ID shared.ID, logger logging.Logger) usecase.TaskUseCases {
	return usecase.TaskUseCases{
		Create:            usecase.NewCreateTaskUseCase(taskUOW, logger),
		List:              usecase.NewListTasksUseCase(taskStore.Tasks, logger, taskStore.ActionItems),
		Get:               usecase.NewGetTaskUseCase(taskUOW, logger),
		Update:            usecase.NewUpdateTaskUseCase(taskUOW, logger),
		Delete:            usecase.NewDeleteTaskUseCase(taskUOW, logger),
		Start:             usecase.NewStartTaskUseCase(taskUOW, logger),
		Hold:              usecase.NewHoldTaskUseCase(taskUOW, logger),
		Wait:              usecase.NewWaitTaskUseCase(taskUOW, logger),
		Complete:          usecase.NewCompleteTaskUseCase(taskUOW, time.Now, logger),
		Reopen:            usecase.NewReopenTaskUseCase(taskUOW, logger),
		Assign:            usecase.NewAssignTaskUseCase(taskUOW, logger),
		ListAssignees:     usecase.NewListTaskAssigneesUseCase(taskStore.Tasks, logger),
		CreateInProject:   usecase.NewCreateTaskInProjectUseCase(taskUOW, logger),
		ListByProject:     usecase.NewListProjectTasksUseCase(taskStore.Tasks, taskStore.Tasks, logger, taskStore.ActionItems),
		AddToProject:      usecase.NewAddTaskToProjectUseCase(taskUOW, taskStore.Tasks, logger),
		RemoveFromProject: usecase.NewRemoveTaskFromProjectUseCase(taskUOW, taskStore.Tasks, logger),
		Revisions:         usecase.NewListTaskRevisionsUseCase(taskStore.Tasks, logger),
	}
}

func NewScheduleUseCases(scheduleStore ScheduleStore, scheduleUOW scheduleusecase.UOW, authStore AuthStore, ID shared.ID, logger logging.Logger) scheduleusecase.ScheduleUseCases {
	return scheduleusecase.ScheduleUseCases{
		Create:               scheduleusecase.NewCreateScheduleUseCase(scheduleUOW, authStore.Users, logger),
		List:                 scheduleusecase.NewListSchedulesUseCase(scheduleUOW, logger),
		ListProject:          scheduleusecase.NewListProjectSchedulesUseCase(scheduleUOW, logger),
		Get:                  scheduleusecase.NewGetScheduleUseCase(scheduleUOW, logger),
		Update:               scheduleusecase.NewUpdateScheduleUseCase(scheduleUOW, logger, ID),
		Delete:               scheduleusecase.NewDeleteScheduleUseCase(scheduleUOW, logger),
		Complete:             scheduleusecase.NewCompleteScheduleUseCase(scheduleUOW, logger, ID),
		Reopen:               scheduleusecase.NewReopenScheduleUseCase(scheduleUOW, logger),
		Skip:                 scheduleusecase.NewSkipScheduleUseCase(scheduleUOW, logger),
		Restore:              scheduleusecase.NewRestoreScheduleUseCase(scheduleUOW, logger),
		Reschedule:           scheduleusecase.NewRescheduleScheduleUseCase(scheduleUOW, logger, ID),
		UpdateFrequency:      scheduleusecase.NewUpdateScheduleFrequencyUseCase(scheduleUOW, logger),
		SetProject:           scheduleusecase.NewSetScheduleProjectUseCase(scheduleUOW, logger),
		RemoveFromProject:    scheduleusecase.NewRemoveScheduleFromProjectUseCase(scheduleUOW, logger),
		Assign:               scheduleusecase.NewAssignScheduleUseCase(scheduleUOW, logger),
		ListAssignees:        scheduleusecase.NewListScheduleAssigneesUseCase(scheduleUOW, logger),
		ListTags:             scheduleusecase.NewListScheduleTagsUseCase(scheduleUOW, logger),
		ListTagsForSchedules: scheduleusecase.NewListScheduleTagsForSchedulesUseCase(scheduleUOW, logger),
		AddTag:               scheduleusecase.NewAddTagToScheduleUseCase(scheduleUOW, logger),
		RemoveTag:            scheduleusecase.NewRemoveTagFromScheduleUseCase(scheduleUOW, logger),
		Revisions:            scheduleusecase.NewListScheduleRevisionsUseCase(scheduleStore.Schedules, logger),
	}
}

func NewActionItemUseCases(taskUOW usecase.UOW, authStore AuthStore, ID shared.ID, logger logging.Logger) usecase.ActionItemUseCases {
	return usecase.ActionItemUseCases{
		Create:          usecase.NewCreateActionItemUseCase(taskUOW, authStore.Users, logger),
		List:            usecase.NewListActionItemsUseCase(taskUOW, logger),
		Update:          usecase.NewUpdateActionItemUseCase(taskUOW, logger, ID),
		Delete:          usecase.NewDeleteActionItemUseCase(taskUOW, logger),
		Complete:        usecase.NewCompleteActionItemUseCase(taskUOW, logger, ID),
		Reopen:          usecase.NewReopenActionItemUseCase(taskUOW, logger),
		Skip:            usecase.NewSkipActionItemUseCase(taskUOW, logger),
		Restore:         usecase.NewRestoreActionItemUseCase(taskUOW, logger),
		Reorder:         usecase.NewReorderActionItemUseCase(taskUOW, logger, ID),
		UpdateFrequency: usecase.NewUpdateActionItemFrequencyUseCase(taskUOW, logger),
	}
}

func NewTaskTagUseCases(taskUOW usecase.UOW, logger logging.Logger) usecase.TaskTagUseCases {
	return usecase.TaskTagUseCases{
		AddToTask:      usecase.NewAddTagToTaskUseCase(taskUOW, logger),
		RemoveFromTask: usecase.NewRemoveTagFromTaskUseCase(taskUOW, logger),
	}
}

func NewTagUseCases(tagStore TagStore, logger logging.Logger) tagusecase.CatalogUseCases {
	return tagusecase.NewCatalogUseCases(tagStore.Tags, logger)
}

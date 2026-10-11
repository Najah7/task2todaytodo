package application

import (
	"time"

	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

func NewTaskUseCases(taskStore TaskStore, taskUOW usecase.UOW, authStore AuthStore, _ IDIssuer, logger telemetry.Logger) usecase.TaskUseCases {
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

func NewScheduleUseCases(scheduleUOW scheduleusecase.UOW, authStore AuthStore, id IDIssuer, logger telemetry.Logger) scheduleusecase.ScheduleUseCases {
	return scheduleusecase.ScheduleUseCases{
		CreateBlueprint:          scheduleusecase.NewCreateBlueprintUseCase(scheduleUOW, id.NewULID, authStore.Users, logger),
		GetBlueprint:             scheduleusecase.NewGetBlueprintUseCase(scheduleUOW),
		ListBlueprintRevisions:   scheduleusecase.NewListBlueprintRevisionsUseCase(scheduleUOW),
		UpdateBlueprint:          scheduleusecase.NewUpdateBlueprintUseCase(scheduleUOW, id.NewULID, authStore.Users),
		DeleteBlueprintFollowing: scheduleusecase.NewDeleteBlueprintFollowingUseCase(scheduleUOW, id.NewULID),
		ListOccurrences:          scheduleusecase.NewListOccurrencesUseCase(scheduleUOW),
		GetOccurrence:            scheduleusecase.NewGetOccurrenceUseCase(scheduleUOW, logger),
		OccurrenceRevisions:      scheduleusecase.NewListOccurrenceRevisionsUseCase(scheduleUOW),
		UpdateOccurrence:         scheduleusecase.NewUpdateOccurrenceUseCase(scheduleUOW, logger, id.NewULID),
		CompleteOccurrence:       scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "complete", logger, id.NewULID),
		ReopenOccurrence:         scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "reopen", logger, id.NewULID),
		SkipOccurrence:           scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "skip", logger, id.NewULID),
		RestoreOccurrence:        scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "restore", logger, id.NewULID),
		DeleteOccurrence:         scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "delete", logger, id.NewULID),
		RescheduleOccurrence:     scheduleusecase.NewRescheduleOccurrenceUseCase(scheduleUOW, logger, id.NewULID),
		Backfill:                 scheduleusecase.NewBackfillUseCase(scheduleUOW, id.NewULID),
		ListAssignees:            scheduleusecase.NewListScheduleAssigneesUseCase(scheduleUOW, logger),
	}
}

func NewActionItemUseCases(taskUOW usecase.UOW, logger telemetry.Logger) usecase.ActionItemUseCases {
	return usecase.ActionItemUseCases{
		Create:   usecase.NewCreateActionItemUseCase(taskUOW, logger),
		List:     usecase.NewListActionItemsUseCase(taskUOW, logger),
		Update:   usecase.NewUpdateActionItemUseCase(taskUOW, logger),
		Delete:   usecase.NewDeleteActionItemUseCase(taskUOW, logger),
		Complete: usecase.NewCompleteActionItemUseCase(taskUOW, logger),
		Reopen:   usecase.NewReopenActionItemUseCase(taskUOW, logger),
		Reorder:  usecase.NewReorderActionItemUseCase(taskUOW, logger),
	}
}

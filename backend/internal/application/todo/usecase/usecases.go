package usecase

import "github.com/Najah7/task2todaytodo/internal/telemetry"

type UseCases struct {
	GetToday         *GetTodayUseCase
	Regenerate       *RegenerateTodayUseCase
	AddActionItem    *AddActionItemUseCase
	AddTask          *AddTaskUseCase
	Reflow           *ReflowUseCase
	CompleteFrame    *CompleteFrameUseCase
	ReopenFrame      *ReopenFrameUseCase
	RemoveActionItem *RemoveActionItemUseCase
	HideSchedule     *HideScheduleUseCase
	RestoreSchedule  *RestoreScheduleUseCase
}

func NewUseCases(uow UOW, newID func() string, loggers ...telemetry.Logger) UseCases {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return UseCases{
		GetToday: NewGetTodayUseCase(uow, newID, logger), Regenerate: NewRegenerateTodayUseCase(uow, newID, logger),
		AddActionItem: NewAddActionItemUseCase(uow, newID, logger), AddTask: NewAddTaskUseCase(uow, newID, logger), Reflow: NewReflowUseCase(uow, newID, logger),
		CompleteFrame: NewCompleteFrameUseCase(uow, logger), ReopenFrame: NewReopenFrameUseCase(uow, logger),
		RemoveActionItem: NewRemoveActionItemUseCase(uow, newID, logger), HideSchedule: NewHideScheduleUseCase(uow, newID, logger),
		RestoreSchedule: NewRestoreScheduleUseCase(uow, newID, logger),
	}
}

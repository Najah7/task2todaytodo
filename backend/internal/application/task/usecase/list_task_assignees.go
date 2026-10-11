package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ListTaskAssigneesUseCase struct {
	tasks  TaskRepository
	logger telemetry.Logger
}

func NewListTaskAssigneesUseCase(tasks TaskRepository, logger telemetry.Logger) *ListTaskAssigneesUseCase {
	return &ListTaskAssigneesUseCase{tasks: tasks, logger: telemetry.OrNop(logger)}
}

func (uc *ListTaskAssigneesUseCase) Execute(ctx context.Context, actor domain.UserID, task domain.TaskID) (assignees []dao.TaskAssignee, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTaskAssigneesUseCase.Execute", err) }()
	return uc.tasks.ListEligibleTaskAssignees(ctx, actor, task)
}

package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListTaskAssigneesUseCase struct {
	tasks  TaskRepository
	logger logging.Logger
}

func NewListTaskAssigneesUseCase(tasks TaskRepository, logger logging.Logger) *ListTaskAssigneesUseCase {
	return &ListTaskAssigneesUseCase{tasks: tasks, logger: logging.OrNop(logger)}
}

func (uc *ListTaskAssigneesUseCase) Execute(ctx context.Context, actor domain.UserID, task domain.TaskID) (assignees []dao.TaskAssignee, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTaskAssigneesUseCase.Execute", err) }()
	return uc.tasks.ListEligibleTaskAssignees(ctx, actor, task)
}

package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListTasksUseCase struct {
	tasks      TaskCursorRepository
	repo       taskListRepository
	projection TaskListProjectionReader
	logger     logging.Logger
}

func NewListTasksUseCase(tasks TaskCursorRepository, logger logging.Logger, projection ...TaskListProjectionReader) *ListTasksUseCase {
	uc := &ListTasksUseCase{logger: logging.OrNop(logger), tasks: tasks}
	if repo, ok := tasks.(taskListRepository); ok {
		uc.repo = repo
	}
	if len(projection) > 0 {
		uc.projection = projection[0]
	}
	return uc
}

func (uc *ListTasksUseCase) Execute(ctx context.Context, userID domain.UserID, request CursorPageRequest) (output CursorPage[dao.Task], err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTasksUseCase.Execute", err) }()

	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.Task]{}, err
	}
	rows, err := uc.tasks.ListByUserIDCursor(ctx, userID, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage[dao.Task]{}, err
	}
	rows, err = EnrichTaskList(ctx, uc.projection, userID, rows, time.Now())
	if err != nil {
		return CursorPage[dao.Task]{}, err
	}
	return taskPage(rows, request.Size), nil

}

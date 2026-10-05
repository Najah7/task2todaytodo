package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type listProjectTasksProjectRepository interface {
	GetByUserID(ctx context.Context, userID domain.UserID, id domain.ProjectID) (dao.Project, error)
}

type listProjectTasksTaskRepository interface {
	ProjectTasksCursorRepository
}

type ListProjectTasksUseCase struct {
	projects listProjectTasksProjectRepository
	tasks    listProjectTasksTaskRepository
	logger   logging.Logger
}

func NewListProjectTasksUseCase(projects listProjectTasksProjectRepository, tasks listProjectTasksTaskRepository, logger logging.Logger) *ListProjectTasksUseCase {
	return &ListProjectTasksUseCase{logger: logging.OrNop(logger), projects: projects, tasks: tasks}
}

func (uc *ListProjectTasksUseCase) Execute(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, request CursorPageRequest) (output CursorPage[dao.Task], err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListProjectTasksUseCase.Execute", err) }()

	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.Task]{}, err
	}
	_, err = uc.projects.GetByUserID(ctx, userID, projectID)
	if err != nil {
		return CursorPage[dao.Task]{}, err
	}
	rows, err := uc.tasks.ListByProjectAndUserIDCursor(ctx, userID, projectID, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage[dao.Task]{}, err
	}
	rows, err = applyTaskProgress(ctx, uc.tasks, rows, time.Now())
	if err != nil {
		return CursorPage[dao.Task]{}, err
	}
	return taskPage(rows, request.Size), nil

}

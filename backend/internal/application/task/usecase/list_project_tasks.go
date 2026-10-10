package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type listProjectTasksTaskRepository interface {
	ProjectTasksCursorRepository
}

type ListProjectTasksUseCase struct {
	projects   TaskProjectRepository
	tasks      listProjectTasksTaskRepository
	projection TaskListProjectionReader
	logger     logging.Logger
}

func NewListProjectTasksUseCase(projects TaskProjectRepository, tasks listProjectTasksTaskRepository, logger logging.Logger, projection ...TaskListProjectionReader) *ListProjectTasksUseCase {
	uc := &ListProjectTasksUseCase{logger: logging.OrNop(logger), projects: projects, tasks: tasks}
	if len(projection) > 0 {
		uc.projection = projection[0]
	}
	return uc
}

func (uc *ListProjectTasksUseCase) Execute(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, request CursorPageRequest) (output CursorPage[dao.Task], err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListProjectTasksUseCase.Execute", err) }()

	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.Task]{}, err
	}
	_, err = uc.projects.GetProjectByUserIDWithPermission(ctx, string(userID), string(projectID), shared.ProjectRead())
	if err != nil {
		return CursorPage[dao.Task]{}, err
	}
	rows, err := uc.tasks.ListByProjectAndUserIDCursor(ctx, userID, projectID, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage[dao.Task]{}, err
	}
	rows, err = EnrichTaskList(ctx, uc.projection, userID, rows, time.Now())
	if err != nil {
		return CursorPage[dao.Task]{}, err
	}
	return taskPage(rows, request.Size), nil

}

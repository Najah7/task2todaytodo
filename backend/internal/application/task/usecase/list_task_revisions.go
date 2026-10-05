package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type taskRevisionReader interface {
	ListTaskRevisionsByActor(context.Context, domain.UserID, domain.TaskID, int, *CursorAnchor) ([]dao.TaskRevision, error)
}

type ListTaskRevisionsUseCase struct {
	repo   taskRevisionReader
	logger logging.Logger
}

func NewListTaskRevisionsUseCase(repo taskRevisionReader, logger logging.Logger) *ListTaskRevisionsUseCase {
	return &ListTaskRevisionsUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *ListTaskRevisionsUseCase) Execute(ctx context.Context, actor domain.UserID, task domain.TaskID, request CursorPageRequest) (page CursorPage[dao.TaskRevision], err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTaskRevisionsUseCase.Execute", err) }()
	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.TaskRevision]{}, err
	}
	rows, err := uc.repo.ListTaskRevisionsByActor(ctx, actor, task, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage[dao.TaskRevision]{}, err
	}
	items, more := pagination.Window(rows, request.Size)
	page = CursorPage[dao.TaskRevision]{Items: items}
	if more && len(items) > 0 {
		last := items[len(items)-1]
		page.Next = &CursorAnchor{Revision: last.Revision}
	}
	return page, nil
}

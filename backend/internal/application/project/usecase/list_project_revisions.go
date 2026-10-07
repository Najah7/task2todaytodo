package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type projectRevisionReader interface {
	ListProjectRevisionsByActor(context.Context, domain.UserID, domain.ProjectID, int, *CursorAnchor) ([]dao.ProjectRevision, error)
}

type ListProjectRevisionsUseCase struct {
	repo   projectRevisionReader
	logger logging.Logger
}

func NewListProjectRevisionsUseCase(repo projectRevisionReader, logger logging.Logger) *ListProjectRevisionsUseCase {
	return &ListProjectRevisionsUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *ListProjectRevisionsUseCase) Execute(ctx context.Context, actor domain.UserID, project domain.ProjectID, request CursorPageRequest) (page CursorPage[dao.ProjectRevision], err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ListProjectRevisionsUseCase.Execute", err) }()
	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.ProjectRevision]{}, err
	}
	rows, err := uc.repo.ListProjectRevisionsByActor(ctx, actor, project, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage[dao.ProjectRevision]{}, err
	}
	items, more := pagination.Window(rows, request.Size)
	page = CursorPage[dao.ProjectRevision]{Items: items}
	if more && len(items) > 0 {
		last := items[len(items)-1]
		page.Next = &CursorAnchor{Revision: last.Revision}
	}
	return page, nil
}

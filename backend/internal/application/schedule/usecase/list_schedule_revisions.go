package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListScheduleRevisionsUseCase struct {
	repository scheduleRevisionReader
	logger     logging.Logger
}

type scheduleRevisionReader interface {
	ListScheduleRevisionsByActor(context.Context, domain.UserID, domain.ScheduleID, int, *CursorAnchor) ([]dao.ScheduleRevision, error)
}

func NewListScheduleRevisionsUseCase(repository scheduleRevisionReader, logger logging.Logger) *ListScheduleRevisionsUseCase {
	return &ListScheduleRevisionsUseCase{repository: repository, logger: logging.OrNop(logger)}
}

func (uc *ListScheduleRevisionsUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID, request CursorPageRequest) (page CursorPage[dao.ScheduleRevision], err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "ListScheduleRevisionsUseCase.Execute", err) }()
	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.ScheduleRevision]{}, err
	}
	rows, err := uc.repository.ListScheduleRevisionsByActor(ctx, actorID, scheduleID, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage[dao.ScheduleRevision]{}, err
	}
	items, more := pagination.Window(rows, request.Size)
	page = CursorPage[dao.ScheduleRevision]{Items: items}
	if more && len(items) > 0 {
		page.Next = &CursorAnchor{Revision: items[len(items)-1].Revision}
	}
	return page, nil
}

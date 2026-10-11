package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
)

type ListOccurrenceRevisionsUseCase struct{ uow UOW }

func NewListOccurrenceRevisionsUseCase(uow UOW) *ListOccurrenceRevisionsUseCase {
	return &ListOccurrenceRevisionsUseCase{uow: uow}
}

func (uc *ListOccurrenceRevisionsUseCase) ExecutePage(ctx context.Context, actorID, blueprintID, occurrenceDate string, size int, beforeRevision int32) (CursorPage[dao.ScheduleOccurrenceRevision], error) {
	if size < 1 || size > pagination.MaxPageSize || beforeRevision < 0 {
		return CursorPage[dao.ScheduleOccurrenceRevision]{}, ErrInvalidSchedulePage
	}
	var page CursorPage[dao.ScheduleOccurrenceRevision]
	err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		_, err := repos.Blueprints().GetPersistedOccurrenceForHistory(ctx, actorID, blueprintID, occurrenceDate)
		if errors.Is(err, ErrOccurrenceNotFound) {
			if _, virtualErr := virtualOccurrenceForDate(ctx, repos.Blueprints(), actorID, blueprintID, occurrenceDate, shared.ScheduleRead()); virtualErr != nil {
				return virtualErr
			}
			page = CursorPage[dao.ScheduleOccurrenceRevision]{Items: []dao.ScheduleOccurrenceRevision{}}
			return nil
		}
		if err != nil {
			return err
		}
		rows, err := repos.Blueprints().ListOccurrenceRevisions(ctx, actorID, blueprintID, occurrenceDate, size+1, beforeRevision)
		if err != nil {
			return err
		}
		page.Items = rows
		if len(rows) > size {
			page.Items = rows[:size]
			if len(page.Items) != 0 {
				page.Next = &CursorAnchor{Revision: page.Items[len(page.Items)-1].Revision}
			}
		}
		return nil
	})
	return page, err
}

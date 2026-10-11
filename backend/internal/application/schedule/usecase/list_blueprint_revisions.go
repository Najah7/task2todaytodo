package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
)

type ListBlueprintRevisionsUseCase struct{ uow UOW }

func NewListBlueprintRevisionsUseCase(uow UOW) *ListBlueprintRevisionsUseCase {
	return &ListBlueprintRevisionsUseCase{uow: uow}
}

func (uc *ListBlueprintRevisionsUseCase) ExecutePage(ctx context.Context, actorID, blueprintID string, size int, beforeRevision int32) (CursorPage[dao.BlueprintRevision], error) {
	if size < 1 || size > pagination.MaxPageSize {
		return CursorPage[dao.BlueprintRevision]{}, ErrInvalidSchedulePage
	}
	rows, err := uc.Execute(ctx, actorID, blueprintID, size+1, beforeRevision)
	if err != nil {
		return CursorPage[dao.BlueprintRevision]{}, err
	}
	page := CursorPage[dao.BlueprintRevision]{Items: rows}
	if len(rows) > size {
		page.Items = rows[:size]
		if len(page.Items) > 0 {
			page.Next = &CursorAnchor{Revision: page.Items[len(page.Items)-1].Revision}
		}
	}
	return page, nil
}

func (uc *ListBlueprintRevisionsUseCase) Execute(ctx context.Context, actorID, blueprintID string, limit int, beforeRevision int32) ([]dao.BlueprintRevision, error) {
	var result []dao.BlueprintRevision
	err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		result, err = repos.Blueprints().ListBlueprintRevisions(ctx, actorID, blueprintID, limit, beforeRevision)
		return err
	})
	return result, err
}

package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListActionItemsUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewListActionItemsUseCase(uow UOW, logger logging.Logger) *ListActionItemsUseCase {
	return &ListActionItemsUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *ListActionItemsUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID) (output []dao.ActionItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListActionItemsUseCase.Execute", err) }()
	var items []dao.ActionItem
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := repos.Tasks().GetByUserIDWithPermission(ctx, userID, taskID, shared.ActionItemRead()); err != nil {
			return err
		}
		var err error
		items, err = repos.ActionItems().ListByTask(ctx, userID, taskID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []dao.ActionItem{}
	}
	return items, nil
}

func (uc *ListActionItemsUseCase) ExecutePage(ctx context.Context, userID domain.UserID, taskID domain.TaskID, request CursorPageRequest) (output CursorPage[dao.ActionItem], err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListActionItemsUseCase.ExecutePage", err) }()
	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.ActionItem]{}, err
	}
	var rows []dao.ActionItem
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := repos.Tasks().GetByUserIDWithPermission(ctx, userID, taskID, shared.ActionItemRead()); err != nil {
			return err
		}
		var err error
		rows, err = repos.ActionItems().ListByTaskCursor(ctx, userID, taskID, request.Size+1, request.Anchor)
		return err
	})
	if err != nil {
		return CursorPage[dao.ActionItem]{}, err
	}
	selected, more := pagination.Window(rows, request.Size)
	page := CursorPage[dao.ActionItem]{Items: selected}
	if more && len(selected) > 0 {
		last := selected[len(selected)-1]
		page.Next = &CursorAnchor{Position: last.Position, ID: last.ID}
	}
	return page, nil
}

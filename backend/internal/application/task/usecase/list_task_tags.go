package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type taskTagListRepository interface {
	ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.TaskTag, error)
}

type ListTaskTagsUseCase struct {
	repo   taskTagListRepository
	logger logging.Logger
}

func NewListTaskTagsUseCase(repo taskTagListRepository, logger logging.Logger) *ListTaskTagsUseCase {
	return &ListTaskTagsUseCase{logger: logging.OrNop(logger), repo: repo}
}

func (uc *ListTaskTagsUseCase) Execute(ctx context.Context, userID domain.UserID) (output []dao.TaskTag, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTaskTagsUseCase.Execute", err) }()

	return uc.repo.ListByUserID(ctx, userID)

}

func (uc *ListTaskTagsUseCase) ExecutePage(ctx context.Context, userID domain.UserID, request CursorPageRequest) (output CursorPage[dao.TaskTag], err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTaskTagsUseCase.ExecutePage", err) }()

	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.TaskTag]{}, err
	}
	repo, ok := uc.repo.(interface {
		ListByUserIDCursor(ctx context.Context, userID domain.UserID, limit int, anchor *CursorAnchor) ([]dao.TaskTag, error)
	})
	if !ok {
		return CursorPage[dao.TaskTag]{}, ErrInvalidTaskPage
	}
	rows, err := repo.ListByUserIDCursor(ctx, userID, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage[dao.TaskTag]{}, err
	}
	items, more := pagination.Window(rows, request.Size)
	page := CursorPage[dao.TaskTag]{Items: items}
	if more && len(items) > 0 {
		last := items[len(items)-1]
		page.Next = &CursorAnchor{Name: last.Name, ID: last.ID}
	}
	return page, nil

}

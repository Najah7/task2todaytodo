package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/application/tag/dao"
	"github.com/Najah7/task2todaytodo/internal/application/tag/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type CatalogUseCases struct {
	Create *CreateTagUseCase
	List   *ListTagsUseCase
	Get    *GetTagUseCase
	Rename *RenameTagUseCase
	Delete *DeleteTagUseCase
}

type CreateTagUseCase struct {
	repo   Repository
	logger logging.Logger
}
type ListTagsUseCase struct {
	repo   Repository
	logger logging.Logger
}
type GetTagUseCase struct {
	repo   Repository
	logger logging.Logger
}
type RenameTagUseCase struct {
	repo   Repository
	logger logging.Logger
}
type DeleteTagUseCase struct {
	repo   Repository
	logger logging.Logger
}

type CreateTagInput struct {
	ID     domain.TagID
	UserID string
	Name   string
}
type RenameTagInput struct {
	UserID string
	ID     domain.TagID
	Name   string
}

func NewCatalogUseCases(repo Repository, logger logging.Logger) CatalogUseCases {
	return CatalogUseCases{Create: NewCreateTagUseCase(repo, logger), List: NewListTagsUseCase(repo, logger), Get: NewGetTagUseCase(repo, logger), Rename: NewRenameTagUseCase(repo, logger), Delete: NewDeleteTagUseCase(repo, logger)}
}
func NewCreateTagUseCase(repo Repository, logger logging.Logger) *CreateTagUseCase {
	return &CreateTagUseCase{repo: repo, logger: logging.OrNop(logger)}
}
func NewListTagsUseCase(repo Repository, logger logging.Logger) *ListTagsUseCase {
	return &ListTagsUseCase{repo: repo, logger: logging.OrNop(logger)}
}
func NewGetTagUseCase(repo Repository, logger logging.Logger) *GetTagUseCase {
	return &GetTagUseCase{repo: repo, logger: logging.OrNop(logger)}
}
func NewRenameTagUseCase(repo Repository, logger logging.Logger) *RenameTagUseCase {
	return &RenameTagUseCase{repo: repo, logger: logging.OrNop(logger)}
}
func NewDeleteTagUseCase(repo Repository, logger logging.Logger) *DeleteTagUseCase {
	return &DeleteTagUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *CreateTagUseCase) Execute(ctx context.Context, input CreateTagInput) (dao.Tag, error) {
	tag, err := domain.NewTag(input.ID, input.UserID, input.Name)
	if err != nil {
		return dao.Tag{}, err
	}
	return uc.repo.Create(ctx, tag)
}
func (uc *ListTagsUseCase) ExecutePage(ctx context.Context, userID string, request CursorPageRequest) (CursorPage, error) {
	if request.Size < 1 || request.Size > 100 {
		return CursorPage{}, ErrInvalidTagPage
	}
	rows, err := uc.repo.ListByUserIDCursor(ctx, userID, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage{}, err
	}
	items, more := pagination.Window(rows, request.Size)
	page := CursorPage{Items: items}
	if more && len(items) > 0 {
		last := items[len(items)-1]
		page.Next = &CursorAnchor{Name: last.Name, ID: last.ID}
	}
	return page, nil
}
func (uc *GetTagUseCase) Execute(ctx context.Context, userID string, id domain.TagID) (dao.Tag, error) {
	return uc.repo.GetByUserID(ctx, userID, id)
}
func (uc *RenameTagUseCase) Execute(ctx context.Context, input RenameTagInput) (dao.Tag, error) {
	tag, err := domain.NewTag(input.ID, input.UserID, input.Name)
	if err != nil {
		return dao.Tag{}, err
	}
	return uc.repo.RenameByUserID(ctx, input.UserID, tag)
}
func (uc *DeleteTagUseCase) Execute(ctx context.Context, userID string, id domain.TagID) error {
	return uc.repo.DeleteByUserID(ctx, userID, id)
}

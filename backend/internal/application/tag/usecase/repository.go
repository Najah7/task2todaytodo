package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/tag/dao"
	"github.com/Najah7/task2todaytodo/internal/application/tag/domain"
)

var (
	ErrTagNotFound     = errors.New("tag not found")
	ErrTagNameConflict = errors.New("tag name already exists for owner")
	ErrInvalidTagPage  = errors.New("invalid tag page request")
)

type Repository interface {
	GetByUserID(ctx context.Context, userID string, id domain.TagID) (dao.Tag, error)
	ListByUserIDCursor(ctx context.Context, userID string, limit int, anchor *CursorAnchor) ([]dao.Tag, error)
	Create(ctx context.Context, tag domain.Tag) (dao.Tag, error)
	RenameByUserID(ctx context.Context, userID string, tag domain.Tag) (dao.Tag, error)
	DeleteByUserID(ctx context.Context, userID string, id domain.TagID) error
}

type CursorAnchor struct {
	Name string
	ID   string
}
type CursorPageRequest struct {
	Size   int
	Anchor *CursorAnchor
}
type CursorPage struct {
	Items []dao.Tag
	Next  *CursorAnchor
}

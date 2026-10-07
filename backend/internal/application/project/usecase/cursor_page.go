package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
)

var ErrInvalidProjectPage = errors.New("invalid project page")

func validateCursorPageRequest(request CursorPageRequest) error {
	if request.Size < 1 || request.Size > pagination.MaxPageSize {
		return ErrInvalidProjectPage
	}
	return nil
}

type projectCursorRepository interface {
	ListByUserIDCursor(context.Context, domain.UserID, int, *CursorAnchor) ([]dao.Project, error)
}

func projectPage(rows []dao.Project, size int) CursorPage[dao.Project] {
	items, more := pagination.Window(rows, size)
	page := CursorPage[dao.Project]{Items: items}
	if more && len(items) > 0 {
		last := items[len(items)-1]
		page.Next = &CursorAnchor{At: last.CursorCreatedAt, ID: last.ID}
	}
	return page
}

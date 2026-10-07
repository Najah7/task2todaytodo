package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

var ErrInvalidTaskPage = errors.New("invalid task page")

type CursorAnchor struct {
	At             string
	ID             string
	Revision       int32
	Name           string
	Position       int
	Date           string
	OccurrenceDate string
	SeriesID       string
	AsOf           string
}

type CursorPageRequest struct {
	Size     int
	FromDate string
	AsOf     time.Time
	Anchor   *CursorAnchor
}

func validateCursorPageRequest(request CursorPageRequest) error {
	if request.Size < 1 || request.Size > pagination.MaxPageSize {
		return ErrInvalidTaskPage
	}
	return nil
}

type CursorPage[T any] struct {
	Items []T
	Next  *CursorAnchor
}

type TaskCursorRepository interface {
	taskProgressSource
	ListByUserIDCursor(ctx context.Context, userID domain.UserID, limit int, anchor *CursorAnchor) ([]dao.Task, error)
}
type ProjectTasksCursorRepository interface {
	taskProgressSource
	ListByProjectAndUserIDCursor(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, limit int, anchor *CursorAnchor) ([]dao.Task, error)
}

func taskPage(rows []dao.Task, size int) CursorPage[dao.Task] {
	items, more := pagination.Window(rows, size)
	page := CursorPage[dao.Task]{Items: items}
	if more && len(items) > 0 {
		last := items[len(items)-1]
		page.Next = &CursorAnchor{At: last.CursorCreatedAt, ID: last.ID}
	}
	return page
}

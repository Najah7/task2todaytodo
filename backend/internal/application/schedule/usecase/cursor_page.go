package usecase

import (
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
)

type CursorAnchor struct {
	At             string
	ID             string
	BlueprintID    string
	Revision       int32
	Date           string
	OccurrenceDate string
	SeriesID       string
	AsOf           string
}

type CursorPageRequest struct {
	Size      int
	From      time.Time
	To        time.Time
	ProjectID string
	FromDate  string
	AsOf      time.Time
	Anchor    *CursorAnchor
}

type CursorPage[T any] struct {
	Items []T
	Next  *CursorAnchor
}

func validateCursorPageRequest(request CursorPageRequest) error {
	if request.Size < 1 || request.Size > pagination.MaxPageSize {
		return ErrInvalidSchedulePage
	}
	return nil
}

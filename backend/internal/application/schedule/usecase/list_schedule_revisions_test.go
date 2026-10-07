package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
)

type scheduleRevisionRepositoryFake struct {
	rows       []dao.ScheduleRevision
	actorID    domain.UserID
	scheduleID domain.ScheduleID
	limit      int
	anchor     *CursorAnchor
}

func (r *scheduleRevisionRepositoryFake) ListScheduleRevisionsByActor(_ context.Context, actor domain.UserID, id domain.ScheduleID, limit int, anchor *CursorAnchor) ([]dao.ScheduleRevision, error) {
	r.actorID, r.scheduleID, r.limit, r.anchor = actor, id, limit, anchor
	return r.rows, nil
}

func TestListScheduleRevisionsPaginatesByRevision(t *testing.T) {
	repository := &scheduleRevisionRepositoryFake{rows: []dao.ScheduleRevision{
		{ID: "schedule", Revision: 8},
		{ID: "schedule", Revision: 7},
		{ID: "schedule", Revision: 6},
	}}
	page, err := NewListScheduleRevisionsUseCase(repository, nil).Execute(context.Background(), "actor", "schedule", CursorPageRequest{Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if repository.actorID != "actor" || repository.scheduleID != "schedule" || repository.limit != 3 || repository.anchor != nil {
		t.Fatalf("repository call actor=%q schedule=%q limit=%d anchor=%+v", repository.actorID, repository.scheduleID, repository.limit, repository.anchor)
	}
	if len(page.Items) != 2 || page.Items[0].Revision != 8 || page.Items[1].Revision != 7 || page.Next == nil || page.Next.Revision != 7 {
		t.Fatalf("page = %+v, want revisions 8,7 and next revision anchor 7", page)
	}
}

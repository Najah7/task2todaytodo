package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type listTasksRepositoryFake struct {
	taskProgressSourceFake
	rows   []dao.Task
	err    error
	userID domain.UserID
	limit  int
	anchor *CursorAnchor
}

type taskProgressSourceFake struct {
	sources    dao.TaskProgressSources
	err        error
	taskIDs    []string
	projectIDs []string
	asOf       time.Time
	calls      int
}

func (source *taskProgressSourceFake) ReadTaskProgressSources(_ context.Context, taskIDs, projectIDs []string, asOf time.Time) (dao.TaskProgressSources, error) {
	source.calls++
	source.taskIDs, source.projectIDs, source.asOf = taskIDs, projectIDs, asOf
	return source.sources, source.err
}

func (repo *listTasksRepositoryFake) ListByUserIDCursor(_ context.Context, userID domain.UserID, limit int, anchor *CursorAnchor) ([]dao.Task, error) {
	repo.userID, repo.limit, repo.anchor = userID, limit, anchor
	return repo.rows, repo.err
}

func TestListTasksUseCaseReturnsCursorPage(t *testing.T) {
	rows := []dao.Task{{ID: "new", CursorCreatedAt: "2026-10-04T00:00:00.000002Z"}, {ID: "next", CursorCreatedAt: "2026-10-04T00:00:00.000001Z"}, {ID: "extra"}}
	repo := &listTasksRepositoryFake{rows: rows, taskProgressSourceFake: taskProgressSourceFake{sources: dao.TaskProgressSources{
		Counts: map[string]dao.TaskProgressCounts{
			"new":  {Total: 2, Completed: 1},
			"next": {Total: 4, Completed: 1},
		},
	}}}
	page, err := NewListTasksUseCase(repo, nil).Execute(context.Background(), "user-1", CursorPageRequest{Size: 2})
	if err != nil || len(page.Items) != 2 || page.Next == nil || page.Next.ID != "next" || page.Next.At != rows[1].CursorCreatedAt {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].Progress != 50 || page.Items[1].Progress != 25 || repo.calls != 1 {
		t.Fatalf("progress=%d,%d source calls=%d, want 50,25 and one read", page.Items[0].Progress, page.Items[1].Progress, repo.calls)
	}
	if repo.userID != "user-1" || repo.limit != 3 || repo.anchor != nil {
		t.Fatalf("repository args=%+v", repo)
	}
}

func TestListTasksUseCasePropagatesRepositoryError(t *testing.T) {
	want := errors.New("database unavailable")
	repo := &listTasksRepositoryFake{err: want}
	if _, err := NewListTasksUseCase(repo, nil).Execute(context.Background(), "user-1", CursorPageRequest{Size: 5}); !errors.Is(err, want) {
		t.Fatalf("error=%v, want %v", err, want)
	}
}

func TestListTasksUseCaseRejectsInvalidPageSize(t *testing.T) {
	for _, size := range []int{-1, 0, 101} {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			repo := &listTasksRepositoryFake{}
			_, err := NewListTasksUseCase(repo, nil).Execute(context.Background(), "user-1", CursorPageRequest{Size: size})
			if !errors.Is(err, ErrInvalidTaskPage) {
				t.Fatalf("error=%v, want ErrInvalidTaskPage", err)
			}
			if repo.limit != 0 {
				t.Fatalf("repository called with limit %d", repo.limit)
			}
		})
	}
}

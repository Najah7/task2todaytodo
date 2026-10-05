package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type taskScheduleCompletionUOWFake struct{ repos Repositories }

func (u taskScheduleCompletionUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repos)
}

type taskScheduleCompletionRepositoriesFake struct {
	taskProgressTestRepositories
	repo *taskScheduleCompletionRepositoryFake
}

func (r taskScheduleCompletionRepositoriesFake) TaskSchedules() TaskScheduleRepository { return r.repo }

type taskScheduleCompletionState struct {
	completed bool
	exception bool
	seriesID  domain.TaskScheduleID
}

type taskScheduleCompletionRepositoryFake struct {
	TaskScheduleRepository
	rows  map[domain.TaskScheduleID]taskScheduleCompletionState
	owner domain.UserID
	task  domain.TaskID
	calls int
}

func (r *taskScheduleCompletionRepositoryFake) SetCompletedForOwnedTask(_ context.Context, user domain.UserID, task domain.TaskID, id domain.TaskScheduleID, completed bool) error {
	r.calls++
	if user != r.owner || task != r.task {
		return domain.ErrTaskScheduleNotFound
	}
	row, ok := r.rows[id]
	if !ok {
		return domain.ErrTaskScheduleNotFound
	}
	row.completed = completed
	if row.seriesID != id {
		row.exception = true
	}
	r.rows[id] = row
	return nil
}

func newTaskScheduleCompletionFixture() (*taskScheduleCompletionUOWFake, *taskScheduleCompletionRepositoryFake) {
	repo := &taskScheduleCompletionRepositoryFake{
		owner: "owner", task: "task-1",
		rows: map[domain.TaskScheduleID]taskScheduleCompletionState{
			"root":     {seriesID: "root"},
			"future-1": {seriesID: "root"},
		},
	}
	uow := &taskScheduleCompletionUOWFake{repos: taskScheduleCompletionRepositoriesFake{repo: repo}}
	return uow, repo
}

func TestCompleteTaskScheduleUseCaseCompletesOnlyRequestedOccurrenceIdempotently(t *testing.T) {
	uow, repo := newTaskScheduleCompletionFixture()
	useCase := NewCompleteTaskScheduleUseCase(uow, nil)
	for range 2 {
		if err := useCase.Execute(context.Background(), "owner", "task-1", "root"); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	}
	if !repo.rows["root"].completed || repo.rows["root"].exception {
		t.Errorf("source state = %+v; want completed source with active recurrence", repo.rows["root"])
	}
	if repo.rows["future-1"].completed || repo.rows["future-1"].exception {
		t.Errorf("future state = %+v; completion changed future occurrence", repo.rows["future-1"])
	}
	if repo.calls != 2 {
		t.Errorf("repository calls = %d, want 2", repo.calls)
	}
}

func TestCompleteTaskScheduleUseCasePreservesGeneratedOccurrenceAsException(t *testing.T) {
	uow, repo := newTaskScheduleCompletionFixture()
	if err := NewCompleteTaskScheduleUseCase(uow, nil).Execute(context.Background(), "owner", "task-1", "future-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := repo.rows["future-1"]; !got.completed || !got.exception {
		t.Errorf("future occurrence = %+v; want completed exception", got)
	}
	if repo.rows["root"].completed {
		t.Error("source was completed as side effect")
	}
}

func TestReopenTaskScheduleUseCaseIsIdempotentAndKeepsOccurrenceException(t *testing.T) {
	uow, repo := newTaskScheduleCompletionFixture()
	row := repo.rows["future-1"]
	row.completed, row.exception = true, true
	repo.rows["future-1"] = row
	useCase := NewReopenTaskScheduleUseCase(uow, nil)
	for range 2 {
		if err := useCase.Execute(context.Background(), "owner", "task-1", "future-1"); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	}
	if got := repo.rows["future-1"]; got.completed || !got.exception {
		t.Errorf("reopened future occurrence = %+v; want reopened exception", got)
	}
}

func TestTaskScheduleCompletionUseCasesHideOtherOwners(t *testing.T) {
	uow, repo := newTaskScheduleCompletionFixture()
	for _, execute := range []func() error{
		func() error {
			return NewCompleteTaskScheduleUseCase(uow, nil).Execute(context.Background(), "other", "task-1", "root")
		},
		func() error {
			return NewReopenTaskScheduleUseCase(uow, nil).Execute(context.Background(), "other", "task-1", "root")
		},
	} {
		if err := execute(); !errors.Is(err, domain.ErrTaskScheduleNotFound) {
			t.Errorf("Execute() error = %v; want not found", err)
		}
	}
	if repo.rows["root"].completed {
		t.Error("other user's request changed source completion")
	}
}

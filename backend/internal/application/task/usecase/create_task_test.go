package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type createTaskRepositoryFake struct {
	task   domain.Task
	result dao.Task
	err    error
	calls  int
}

func (repo *createTaskRepositoryFake) Create(_ context.Context, task domain.Task) (dao.Task, error) {
	repo.calls++
	repo.task = task
	return repo.result, repo.err
}

func validCreateTaskInput() CreateTaskInput {
	return CreateTaskInput{
		ID:          domain.TaskID("task-1"),
		UserID:      domain.UserID("user-1"),
		Title:       "Task title",
		Description: "Task description",
	}
}

func TestCreateTaskUseCaseExecuteCreatesStandaloneTaskWithDefaults(t *testing.T) {
	want := dao.Task{ID: "task-1", UserID: "user-1", Title: "Task title"}
	repo := &createTaskRepositoryFake{result: want}

	got, err := NewCreateTaskUseCase(repo, nil).Execute(context.Background(), validCreateTaskInput())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got != want {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if repo.calls != 1 {
		t.Fatalf("Create() calls = %d, want 1", repo.calls)
	}
	if repo.task.ID != domain.TaskID("task-1") || repo.task.UserID != domain.UserID("user-1") {
		t.Errorf("created task identity = (%q, %q), want (%q, %q)", repo.task.ID, repo.task.UserID, "task-1", "user-1")
	}
	if repo.task.ProjectID != "" {
		t.Errorf("created task project ID = %q, want empty", repo.task.ProjectID)
	}
	if repo.task.Priority.String() != "low" || repo.task.Status.String() != "open" || repo.task.Progress != 0 {
		t.Errorf("created defaults = priority %q, status %q, progress %d; want low, open, 0", repo.task.Priority.String(), repo.task.Status.String(), repo.task.Progress)
	}
}

func TestCreateTaskUseCaseExecutePassesOptionalFields(t *testing.T) {
	dueDate := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	estimatedMinutes := 45
	input := validCreateTaskInput()
	input.DueDate = dueDate
	input.EstimatedMinutes = &estimatedMinutes
	input.Priority = "high"
	repo := &createTaskRepositoryFake{}

	_, err := NewCreateTaskUseCase(repo, nil).Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !repo.task.DueDate.Equal(dueDate) || repo.task.EstimatedMinutes == nil || *repo.task.EstimatedMinutes != estimatedMinutes || repo.task.Priority.String() != "high" {
		t.Errorf("created optional fields = due %v, estimate %v, priority %q; want due %v, estimate %d, high", repo.task.DueDate, repo.task.EstimatedMinutes, repo.task.Priority.String(), dueDate, estimatedMinutes)
	}
}

func TestCreateTaskUseCaseExecuteRejectsEmptyTitle(t *testing.T) {
	input := validCreateTaskInput()
	input.Title = " \t "
	repo := &createTaskRepositoryFake{}

	_, err := NewCreateTaskUseCase(repo, nil).Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrTaskTitleEmpty) {
		t.Errorf("Execute() error = %v, want %v", err, domain.ErrTaskTitleEmpty)
	}
	if repo.calls != 0 {
		t.Errorf("Create() calls = %d, want 0", repo.calls)
	}
}

func TestCreateTaskUseCaseExecuteRejectsInvalidPriority(t *testing.T) {
	input := validCreateTaskInput()
	input.Priority = "invalid"
	repo := &createTaskRepositoryFake{}

	_, err := NewCreateTaskUseCase(repo, nil).Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrTaskPriorityInvalid) {
		t.Errorf("Execute() error = %v, want %v", err, domain.ErrTaskPriorityInvalid)
	}
	if repo.calls != 0 {
		t.Errorf("Create() calls = %d, want 0", repo.calls)
	}
}

func TestCreateTaskUseCaseExecuteRejectsNegativeEstimate(t *testing.T) {
	input := validCreateTaskInput()
	negative := -1
	input.EstimatedMinutes = &negative
	repo := &createTaskRepositoryFake{}

	_, err := NewCreateTaskUseCase(repo, nil).Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrTaskEstimatedMinutesInvalid) {
		t.Errorf("Execute() error = %v, want %v", err, domain.ErrTaskEstimatedMinutesInvalid)
	}
	if repo.calls != 0 {
		t.Errorf("Create() calls = %d, want 0", repo.calls)
	}
}

func TestCreateTaskUseCaseExecutePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &createTaskRepositoryFake{err: wantErr}

	_, err := NewCreateTaskUseCase(repo, nil).Execute(context.Background(), validCreateTaskInput())
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if repo.calls != 1 {
		t.Errorf("Create() calls = %d, want 1", repo.calls)
	}
}

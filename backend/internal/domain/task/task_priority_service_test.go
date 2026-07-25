package task

import (
	"context"
	"errors"
	"testing"
)

func TestTaskPriorityServiceList(t *testing.T) {
	want := []TaskPriority{mustTaskPriorityForService(t, "urgent")}
	repo := &stubTaskPriorityRepository{priorities: want}

	got, err := NewTaskPriorityService(repo).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("List() = %+v, want %+v", got, want)
	}
	if !repo.listCalled {
		t.Error("repo.List() was not called")
	}
}

func TestTaskPriorityServiceListError(t *testing.T) {
	errList := errors.New("list task priorities")
	_, err := NewTaskPriorityService(&stubTaskPriorityRepository{err: errList}).List(context.Background())
	if !errors.Is(err, errList) {
		t.Errorf("List() error = %v, want %v", err, errList)
	}
}

type stubTaskPriorityRepository struct {
	priorities []TaskPriority
	err        error
	listCalled bool
}

func (r *stubTaskPriorityRepository) List(ctx context.Context) ([]TaskPriority, error) {
	r.listCalled = true
	return r.priorities, r.err
}

func mustTaskPriorityForService(t *testing.T, value string) TaskPriority {
	t.Helper()
	priority, err := NewTaskPriority(value)
	if err != nil {
		t.Fatalf("NewTaskPriority() error = %v", err)
	}
	return priority
}

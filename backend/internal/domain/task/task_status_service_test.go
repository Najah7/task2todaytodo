package task

import (
	"context"
	"errors"
	"testing"
)

func TestTaskStatusServiceList(t *testing.T) {
	want := []TaskStatus{mustTaskStatusForService(t, "open")}
	repo := &stubTaskStatusRepository{statuses: want}

	got, err := NewTaskStatusService(repo).List(context.Background())
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

func TestTaskStatusServiceListError(t *testing.T) {
	errList := errors.New("list task statuses")
	_, err := NewTaskStatusService(&stubTaskStatusRepository{err: errList}).List(context.Background())
	if !errors.Is(err, errList) {
		t.Errorf("List() error = %v, want %v", err, errList)
	}
}

type stubTaskStatusRepository struct {
	statuses   []TaskStatus
	err        error
	listCalled bool
}

func (r *stubTaskStatusRepository) List(ctx context.Context) ([]TaskStatus, error) {
	r.listCalled = true
	return r.statuses, r.err
}

func mustTaskStatusForService(t *testing.T, value string) TaskStatus {
	t.Helper()
	status, err := NewTaskStatus(value)
	if err != nil {
		t.Fatalf("NewTaskStatus() error = %v", err)
	}
	return status
}

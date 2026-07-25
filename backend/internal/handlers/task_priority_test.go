package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
)

func TestTaskPriorityHandlerList(t *testing.T) {
	handler := NewTaskPriorityHandler(domain.NewTaskPriorityService(&stubTaskPriorityHandlerRepository{
		priorities: []domain.TaskPriority{mustTaskPriorityForHandler(t, "urgent")},
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tasks/priorities", nil)

	handler.List(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var got TaskPriorityListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	want := []TaskPriorityResponse{{Priority: "urgent", Label: "Urgent", LabelJp: "緊急", Weight: 100}}
	if len(got.Data) != len(want) || got.Data[0] != want[0] {
		t.Errorf("data = %+v, want %+v", got.Data, want)
	}
}

type stubTaskPriorityHandlerRepository struct {
	priorities []domain.TaskPriority
	err        error
}

func (r *stubTaskPriorityHandlerRepository) List(ctx context.Context) ([]domain.TaskPriority, error) {
	return r.priorities, r.err
}

func mustTaskPriorityForHandler(t *testing.T, value string) domain.TaskPriority {
	t.Helper()
	priority, err := domain.NewTaskPriority(value)
	if err != nil {
		t.Fatalf("NewTaskPriority() error = %v", err)
	}
	return priority
}

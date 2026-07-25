package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/Najah7/task2schedule/internal/domain/task"
)

func TestTaskStatusHandlerList(t *testing.T) {
	handler := NewTaskStatusHandler(domain.NewTaskStatusService(&stubTaskStatusHandlerRepository{
		statuses: []domain.TaskStatus{mustTaskStatusForHandler(t, "open")},
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tasks/statuses", nil)

	handler.List(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var got TaskStatusListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	want := []TaskStatusResponse{{Status: "open", Label: "Open", LabelJp: "オープン"}}
	if len(got.Data) != len(want) || got.Data[0] != want[0] {
		t.Errorf("data = %+v, want %+v", got.Data, want)
	}
}

type stubTaskStatusHandlerRepository struct {
	statuses []domain.TaskStatus
	err      error
}

func (r *stubTaskStatusHandlerRepository) List(ctx context.Context) ([]domain.TaskStatus, error) {
	return r.statuses, r.err
}

func mustTaskStatusForHandler(t *testing.T, value string) domain.TaskStatus {
	t.Helper()
	status, err := domain.NewTaskStatus(value)
	if err != nil {
		t.Fatalf("NewTaskStatus() error = %v", err)
	}
	return status
}

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
)

func TestTaskFrequencyHandlerList(t *testing.T) {
	handler := NewTaskFrequencyHandler(domain.NewTaskFrequencyService(&stubTaskFrequencyHandlerRepository{
		frequencies: []domain.TaskFrequency{mustTaskFrequencyForHandler(t, "mon")},
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tasks/frequencies", nil)

	handler.List(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var got TaskFrequencyListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	want := []TaskFrequencyResponse{{Frequency: "mon", Label: "Monday", LabelJp: "月曜日"}}
	if len(got.Data) != len(want) || got.Data[0] != want[0] {
		t.Errorf("data = %+v, want %+v", got.Data, want)
	}
}

type stubTaskFrequencyHandlerRepository struct {
	frequencies []domain.TaskFrequency
	err         error
}

func (r *stubTaskFrequencyHandlerRepository) List(ctx context.Context) ([]domain.TaskFrequency, error) {
	return r.frequencies, r.err
}

func mustTaskFrequencyForHandler(t *testing.T, value string) domain.TaskFrequency {
	t.Helper()
	frequency, err := domain.NewTaskFrequency(value)
	if err != nil {
		t.Fatalf("NewTaskFrequency() error = %v", err)
	}
	return frequency
}

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/task/usecase"
)

func TestProjectTypeHandlerList(t *testing.T) {
	handler := NewProjectTypeHandler(taskusecase.NewProjectTypeService(&stubProjectTypeHandlerRepository{
		projectTypes: []domain.ProjectType{mustProjectTypeForHandler(t, "work")},
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/projects/types", nil)

	handler.List(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var got ProjectTypeListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	want := []ProjectTypeResponse{{Type: "work", Label: "Work", LabelJp: "仕事"}}
	if len(got.Data) != len(want) || got.Data[0] != want[0] {
		t.Errorf("data = %+v, want %+v", got.Data, want)
	}
}

func TestProjectTypeHandlerListError(t *testing.T) {
	errList := errors.New("list project types")
	handler := NewProjectTypeHandler(taskusecase.NewProjectTypeService(&stubProjectTypeHandlerRepository{err: errList}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/projects/types", nil)

	handler.List(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	var got sharedhandlers.ErrResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Error.Code != sharedhandlers.ErrSpecProjectTypesListFailed.Code {
		t.Errorf("code = %q, want %q", got.Error.Code, sharedhandlers.ErrSpecProjectTypesListFailed.Code)
	}
}

type stubProjectTypeHandlerRepository struct {
	projectTypes []domain.ProjectType
	err          error
}

func (r *stubProjectTypeHandlerRepository) List(ctx context.Context) ([]domain.ProjectType, error) {
	return r.projectTypes, r.err
}

func mustProjectTypeForHandler(t *testing.T, value string) domain.ProjectType {
	t.Helper()
	projectType, err := domain.NewProjectType(value)
	if err != nil {
		t.Fatalf("NewProjectType() error = %v", err)
	}
	return projectType
}

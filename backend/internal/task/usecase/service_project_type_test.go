package usecase

import (
	"context"
	"errors"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	"testing"
)

func TestProjectTypeServiceList(t *testing.T) {
	want := []domain.ProjectType{mustProjectTypeForService(t, "work")}
	repo := &stubProjectTypeRepository{projectTypes: want}

	got, err := NewProjectTypeService(repo).List(context.Background())
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

func TestProjectTypeServiceListError(t *testing.T) {
	errList := errors.New("list project types")
	_, err := NewProjectTypeService(&stubProjectTypeRepository{err: errList}).List(context.Background())
	if !errors.Is(err, errList) {
		t.Errorf("List() error = %v, want %v", err, errList)
	}
}

type stubProjectTypeRepository struct {
	projectTypes []domain.ProjectType
	err          error
	listCalled   bool
}

func (r *stubProjectTypeRepository) List(ctx context.Context) ([]domain.ProjectType, error) {
	r.listCalled = true
	return r.projectTypes, r.err
}

func mustProjectTypeForService(t *testing.T, value string) domain.ProjectType {
	t.Helper()
	projectType, err := domain.NewProjectType(value)
	if err != nil {
		t.Fatalf("domain.NewProjectType() error = %v", err)
	}
	return projectType
}

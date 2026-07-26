package usecase

import (
	"context"
	"errors"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	"testing"
)

func TestTaskFrequencyServiceList(t *testing.T) {
	want := []domain.TaskFrequency{mustTaskFrequencyForService(t, "mon")}
	repo := &stubTaskFrequencyRepository{frequencies: want}

	got, err := NewTaskFrequencyService(repo).List(context.Background())
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

func TestTaskFrequencyServiceListError(t *testing.T) {
	errList := errors.New("list task frequencies")
	_, err := NewTaskFrequencyService(&stubTaskFrequencyRepository{err: errList}).List(context.Background())
	if !errors.Is(err, errList) {
		t.Errorf("List() error = %v, want %v", err, errList)
	}
}

type stubTaskFrequencyRepository struct {
	frequencies []domain.TaskFrequency
	err         error
	listCalled  bool
}

func (r *stubTaskFrequencyRepository) List(ctx context.Context) ([]domain.TaskFrequency, error) {
	r.listCalled = true
	return r.frequencies, r.err
}

func mustTaskFrequencyForService(t *testing.T, value string) domain.TaskFrequency {
	t.Helper()
	frequency, err := domain.NewTaskFrequency(value)
	if err != nil {
		t.Fatalf("domain.NewTaskFrequency() error = %v", err)
	}
	return frequency
}

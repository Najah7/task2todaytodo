package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type createProjectRepositoryFake struct {
	project domain.Project
	result  dao.Project
	err     error
	calls   int
}

func (repo *createProjectRepositoryFake) Create(_ context.Context, project domain.Project) (dao.Project, error) {
	repo.calls++
	repo.project = project
	return repo.result, repo.err
}

func validCreateProjectInput() CreateProjectInput {
	return CreateProjectInput{
		ID:          domain.ProjectID("project-1"),
		UserID:      domain.UserID("user-1"),
		Title:       "Project title",
		Goal:        "Project goal",
		Description: "Project description",
	}
}

func TestCreateProjectUseCaseExecuteCreatesProjectWithDefaultsAndOwner(t *testing.T) {
	want := dao.Project{ID: "project-1", UserID: "user-1", Title: "Project title"}
	repo := &createProjectRepositoryFake{result: want}

	got, err := NewCreateProjectUseCase(repo, nil).Execute(context.Background(), validCreateProjectInput())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got != want {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if repo.calls != 1 {
		t.Fatalf("Create() calls = %d, want 1", repo.calls)
	}
	if repo.project.ID != domain.ProjectID("project-1") || repo.project.UserID != domain.UserID("user-1") {
		t.Errorf("created project identity = (%q, %q), want (%q, %q)", repo.project.ID, repo.project.UserID, "project-1", "user-1")
	}
	if repo.project.Type.String() != "other" || repo.project.Priority.String() != "low" || repo.project.Progress != 0 {
		t.Errorf("created defaults = type %q, priority %q, progress %d; want other, low, 0", repo.project.Type.String(), repo.project.Priority.String(), repo.project.Progress)
	}
}

func TestCreateProjectUseCaseExecuteAcceptsOneSidedDatesAndNormalizesToUTCDate(t *testing.T) {
	start := time.Date(2026, time.April, 5, 19, 20, 0, 0, time.FixedZone("UTC+9", 9*60*60))
	input := validCreateProjectInput()
	input.StartDate = &start
	repo := &createProjectRepositoryFake{}

	_, err := NewCreateProjectUseCase(repo, nil).Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	got := repo.project.Schedule.StartDate
	if got == nil {
		t.Fatal("created project start date = nil, want date")
	}
	if got.Location() != time.UTC || got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 || got.Nanosecond() != 0 || got.Day() != 5 {
		t.Errorf("normalized start date = %v, want UTC midnight on 2026-04-05", got)
	}
	if repo.project.Schedule.EndDate != nil {
		t.Errorf("created project end date = %v, want nil", repo.project.Schedule.EndDate)
	}
}

func TestCreateProjectUseCaseExecuteAcceptsSameStartAndEndDate(t *testing.T) {
	day := time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC)
	input := validCreateProjectInput()
	input.StartDate = &day
	input.EndDate = &day
	repo := &createProjectRepositoryFake{}

	_, err := NewCreateProjectUseCase(repo, nil).Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.calls != 1 {
		t.Errorf("Create() calls = %d, want 1", repo.calls)
	}
}

func TestCreateProjectUseCaseExecuteRejectsEndDateBeforeStartDate(t *testing.T) {
	start := time.Date(2026, time.April, 6, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC)
	input := validCreateProjectInput()
	input.StartDate = &start
	input.EndDate = &end
	repo := &createProjectRepositoryFake{}

	_, err := NewCreateProjectUseCase(repo, nil).Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrProjectEndDateBeforeStartDate) {
		t.Errorf("Execute() error = %v, want %v", err, domain.ErrProjectEndDateBeforeStartDate)
	}
	if repo.calls != 0 {
		t.Errorf("Create() calls = %d, want 0", repo.calls)
	}
}

func TestCreateProjectUseCaseExecuteRejectsEmptyTitle(t *testing.T) {
	input := validCreateProjectInput()
	input.Title = " \t "
	repo := &createProjectRepositoryFake{}

	_, err := NewCreateProjectUseCase(repo, nil).Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrProjectTitleEmpty) {
		t.Errorf("Execute() error = %v, want %v", err, domain.ErrProjectTitleEmpty)
	}
	if repo.calls != 0 {
		t.Errorf("Create() calls = %d, want 0", repo.calls)
	}
}

func TestCreateProjectUseCaseExecutePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &createProjectRepositoryFake{err: wantErr}

	_, err := NewCreateProjectUseCase(repo, nil).Execute(context.Background(), validCreateProjectInput())
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if repo.calls != 1 {
		t.Errorf("Create() calls = %d, want 1", repo.calls)
	}
}

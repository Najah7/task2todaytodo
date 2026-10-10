package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
)

type listProjectsRepository struct {
	projectsByUserID map[domain.UserID][]dao.Project
	err              error
	gotUserID        domain.UserID
}

func (repo *listProjectsRepository) ListByUserID(_ context.Context, userID domain.UserID) ([]dao.Project, error) {
	repo.gotUserID = userID
	if repo.err != nil {
		return nil, repo.err
	}
	return repo.projectsByUserID[userID], nil
}

func TestListProjectsUseCaseExecute(t *testing.T) {
	userID := domain.UserID("user-1")
	want := []dao.Project{
		{ID: "project-1", UserID: string(userID), Title: "Owned project", StartDate: stringPointer("2026-10-01")},
	}
	want[0].Progress = 75
	progress := &projectProgressSourceFake{sources: dao.ProjectProgressSources{
		Tasks: []dao.ProjectTaskProgress{
			{ProjectID: "project-1", TaskID: "task-1", Progress: 50},
			{ProjectID: "project-1", TaskID: "task-2", Done: true, Progress: 100},
		},
	}}
	repo := &listProjectsRepository{projectsByUserID: map[domain.UserID][]dao.Project{
		userID:                      want,
		domain.UserID("other-user"): {{ID: "project-2", UserID: "other-user", Title: "Other project"}},
	}}

	got, err := NewListProjectsUseCase(repo, progress, nil).Execute(context.Background(), userID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.gotUserID != userID {
		t.Errorf("repository user ID = %q, want authenticated user ID %q", repo.gotUserID, userID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if progress.calls != 1 || !reflect.DeepEqual(progress.ids, []string{"project-1"}) {
		t.Errorf("progress source calls=%d project IDs=%v, want one read for project-1", progress.calls, progress.ids)
	}
}

func TestListProjectsUseCaseCombinesTaskAndScheduleProgressIncludingEmptyTaskSets(t *testing.T) {
	projects := []dao.Project{{ID: "mixed"}, {ID: "schedule-only"}, {ID: "empty"}, {ID: "task-only"}}
	progress := &projectProgressSourceFake{sources: dao.ProjectProgressSources{
		Tasks: []dao.ProjectTaskProgress{
			{ProjectID: "mixed", TaskID: "task-1", Progress: 50},
			{ProjectID: "task-only", TaskID: "task-2", Progress: 25},
		},
		Schedules: []dao.ProjectScheduleProgress{
			{ProjectID: "mixed", Total: 2, Completed: 1},
			{ProjectID: "schedule-only", Total: 2, Completed: 1},
		},
	}}
	repo := &listProjectsRepository{projectsByUserID: map[domain.UserID][]dao.Project{"user-1": projects}}

	got, err := NewListProjectsUseCase(repo, progress, nil).Execute(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	want := map[string]int{"mixed": 50, "schedule-only": 50, "empty": 0, "task-only": 25}
	for _, project := range got {
		if project.Progress != want[project.ID] {
			t.Errorf("project %q progress = %d, want %d", project.ID, project.Progress, want[project.ID])
		}
	}
	if progress.calls != 1 || !reflect.DeepEqual(progress.ids, []string{"mixed", "schedule-only", "empty", "task-only"}) {
		t.Errorf("progress read ids=%v calls=%d", progress.ids, progress.calls)
	}
}

func TestListProjectsUseCaseReturnsEmptyList(t *testing.T) {
	userID := domain.UserID("user-without-projects")
	repo := &listProjectsRepository{projectsByUserID: map[domain.UserID][]dao.Project{
		userID: {},
	}}

	got, err := NewListProjectsUseCase(repo, &projectProgressSourceFake{}, nil).Execute(context.Background(), userID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("Execute() = %#v, want empty list", got)
	}
	if repo.gotUserID != userID {
		t.Errorf("repository user ID = %q, want authenticated user ID %q", repo.gotUserID, userID)
	}
}

func TestListProjectsUseCasePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &listProjectsRepository{err: wantErr}

	got, err := NewListProjectsUseCase(repo, &projectProgressSourceFake{}, nil).Execute(context.Background(), domain.UserID("user-1"))
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("Execute() = %#v, want nil list on error", got)
	}
}

func stringPointer(value string) *string {
	return &value
}

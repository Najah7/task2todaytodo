package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type getProjectRepositoryFake struct {
	projects map[getProjectKey]dao.Project
	err      error
	userID   domain.UserID
	id       domain.ProjectID
	calls    int
}

type getProjectKey struct {
	userID    domain.UserID
	projectID domain.ProjectID
}

func (repo *getProjectRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, id domain.ProjectID) (dao.Project, error) {
	repo.calls++
	repo.userID = userID
	repo.id = id
	if repo.err != nil {
		return dao.Project{}, repo.err
	}
	project, ok := repo.projects[getProjectKey{userID: userID, projectID: id}]
	if !ok {
		return dao.Project{}, domain.ErrProjectNotFound
	}
	return project, nil
}

func (*getProjectRepositoryFake) HasPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (bool, error) {
	return true, nil
}

func TestGetProjectUseCaseExecuteReturnsOwnedProject(t *testing.T) {
	userID := domain.UserID("user-1")
	projectID := domain.ProjectID("project-1")
	want := dao.Project{
		ID:          string(projectID),
		UserID:      string(userID),
		Title:       "Owned project",
		Goal:        "Ship the feature",
		Description: "Project details",
		Progress:    25,
		CanUpdate:   true,
		CanDelete:   true,
	}
	repo := &getProjectRepositoryFake{projects: map[getProjectKey]dao.Project{
		{userID: userID, projectID: projectID}: want,
	}}

	progress := &projectProgressSourceFake{sources: dao.ProjectProgressSources{
		Tasks: []dao.ProjectTaskProgress{{ProjectID: string(projectID), TaskID: "task-1", Total: 4, Completed: 1}},
	}}
	got, err := NewGetProjectUseCase(repo, progress, nil).Execute(context.Background(), userID, projectID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if progress.calls != 1 || got.Progress != 25 {
		t.Errorf("progress=%d source reads=%d, want 25 and one read", got.Progress, progress.calls)
	}
	if repo.calls != 1 {
		t.Errorf("repository calls = %d, want 1", repo.calls)
	}
	if repo.userID != userID || repo.id != projectID {
		t.Errorf("GetByUserID() arguments = (%q, %q), want (%q, %q)", repo.userID, repo.id, userID, projectID)
	}
}

func TestGetProjectUseCaseExecuteReturnsNotFoundForMissingOrUnownedProject(t *testing.T) {
	userID := domain.UserID("user-1")
	projectID := domain.ProjectID("project-1")
	for _, testCase := range []struct {
		name     string
		projects map[getProjectKey]dao.Project
	}{
		{name: "missing project"},
		{
			name: "project belongs to another user",
			projects: map[getProjectKey]dao.Project{
				{userID: domain.UserID("other-user"), projectID: projectID}: {
					ID: string(projectID), UserID: "other-user", Title: "Other user's project",
				},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &getProjectRepositoryFake{projects: testCase.projects}

			got, err := NewGetProjectUseCase(repo, &projectProgressSourceFake{}, nil).Execute(context.Background(), userID, projectID)
			if !errors.Is(err, domain.ErrProjectNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, domain.ErrProjectNotFound)
			}
			if got != (dao.Project{}) {
				t.Errorf("Execute() = %#v, want zero project on error", got)
			}
			if repo.calls != 1 || repo.userID != userID || repo.id != projectID {
				t.Errorf("GetByUserID() = calls %d, user %q, project %q; want 1, %q, %q", repo.calls, repo.userID, repo.id, userID, projectID)
			}
		})
	}
}

func TestGetProjectUseCaseExecutePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &getProjectRepositoryFake{err: wantErr}

	got, err := NewGetProjectUseCase(repo, &projectProgressSourceFake{}, nil).Execute(context.Background(), domain.UserID("user-1"), domain.ProjectID("project-1"))
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if got != (dao.Project{}) {
		t.Errorf("Execute() = %#v, want zero project on error", got)
	}
	if repo.calls != 1 {
		t.Errorf("repository calls = %d, want 1", repo.calls)
	}
}

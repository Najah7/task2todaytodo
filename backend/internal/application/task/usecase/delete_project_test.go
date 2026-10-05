package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type deleteProjectRepositoryFake struct {
	ProjectRepository
	project         dao.Project
	lockErr         error
	err             error
	userID          domain.UserID
	projectID       domain.ProjectID
	expected        int32
	lockCalls       int
	activeLockCalls int
	deleteCalls     int
}

func (repo *deleteProjectRepositoryFake) LockByUserIDWithPermission(_ context.Context, userID domain.UserID, projectID domain.ProjectID, permission shared.Capability) (dao.Project, error) {
	repo.lockCalls++
	repo.userID, repo.projectID = userID, projectID
	if permission != shared.ProjectDelete() {
		return dao.Project{}, errors.New("unexpected project permission")
	}
	if repo.lockErr != nil {
		return dao.Project{}, repo.lockErr
	}
	return repo.project, nil
}

func (repo *deleteProjectRepositoryFake) LockActiveTasksForDeletion(context.Context, domain.ProjectID) error {
	repo.activeLockCalls++
	return nil
}

func (repo *deleteProjectRepositoryFake) DeleteByUserID(_ context.Context, userID domain.UserID, projectID domain.ProjectID, expectedRevision int32) error {
	repo.deleteCalls++
	repo.userID, repo.projectID, repo.expected = userID, projectID, expectedRevision
	return repo.err
}

type deleteProjectRepositoriesFake struct {
	taskProgressTestRepositories
	projects *deleteProjectRepositoryFake
}

func (repos deleteProjectRepositoriesFake) Projects() ProjectRepository { return repos.projects }

type deleteProjectUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *deleteProjectUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

func newDeleteProjectFixture(repoErr, lockErr error) (*deleteProjectUOWFake, *deleteProjectRepositoryFake) {
	repo := &deleteProjectRepositoryFake{
		project: dao.Project{ID: "project-1", UserID: "user-1", Revision: 4},
		err:     repoErr, lockErr: lockErr,
	}
	return &deleteProjectUOWFake{repos: deleteProjectRepositoriesFake{projects: repo}}, repo
}

func TestDeleteProjectUseCaseExecuteLocksAndDeletesCurrentProjectRevision(t *testing.T) {
	uow, repo := newDeleteProjectFixture(nil, nil)
	err := NewDeleteProjectUseCase(uow, nil).Execute(context.Background(), "user-1", "project-1", 4)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if uow.calls != 1 || repo.lockCalls != 1 || repo.activeLockCalls != 1 || repo.deleteCalls != 1 {
		t.Errorf("calls = UOW:%d lock:%d active-task-lock:%d delete:%d; want 1 each", uow.calls, repo.lockCalls, repo.activeLockCalls, repo.deleteCalls)
	}
	if repo.userID != "user-1" || repo.projectID != "project-1" || repo.expected != 4 {
		t.Errorf("DeleteByUserID() arguments = (%q, %q, %d), want (user-1, project-1, 4)", repo.userID, repo.projectID, repo.expected)
	}
}

func TestDeleteProjectUseCaseExecuteReturnsNotFoundBeforeCascadeLocks(t *testing.T) {
	uow, repo := newDeleteProjectFixture(nil, domain.ErrProjectNotFound)
	err := NewDeleteProjectUseCase(uow, nil).Execute(context.Background(), "user-1", "project-1", 4)
	if !errors.Is(err, domain.ErrProjectNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, domain.ErrProjectNotFound)
	}
	if repo.lockCalls != 1 || repo.activeLockCalls != 0 || repo.deleteCalls != 0 {
		t.Errorf("calls = lock:%d active-task-lock:%d delete:%d; want lock only", repo.lockCalls, repo.activeLockCalls, repo.deleteCalls)
	}
}

func TestDeleteProjectUseCaseExecutePropagatesRepositoryAndUOWErrors(t *testing.T) {
	wantErr := errors.New("database unavailable")
	uow, repo := newDeleteProjectFixture(wantErr, nil)
	if err := NewDeleteProjectUseCase(uow, nil).Execute(context.Background(), "user-1", "project-1", 4); !errors.Is(err, wantErr) {
		t.Errorf("repository error = %v, want %v", err, wantErr)
	}
	if repo.lockCalls != 1 || repo.activeLockCalls != 1 || repo.deleteCalls != 1 {
		t.Errorf("repository calls = lock:%d active-task-lock:%d delete:%d; want each once", repo.lockCalls, repo.activeLockCalls, repo.deleteCalls)
	}

	uow, repo = newDeleteProjectFixture(nil, nil)
	uow.err = wantErr
	if err := NewDeleteProjectUseCase(uow, nil).Execute(context.Background(), "user-1", "project-1", 4); !errors.Is(err, wantErr) {
		t.Errorf("UOW error = %v, want %v", err, wantErr)
	}
	if uow.calls != 1 || repo.lockCalls != 0 {
		t.Errorf("UOW calls = %d and project locks = %d; want one UOW call without callback", uow.calls, repo.lockCalls)
	}
}

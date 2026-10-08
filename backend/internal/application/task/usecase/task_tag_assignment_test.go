package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	taskdao "github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type taskTagAssignmentRepo struct {
	TaskTagRepository
	added, removed    string
	addErr, removeErr error
}

func (repo *taskTagAssignmentRepo) ListByTaskAndUserID(context.Context, domain.UserID, domain.TaskID) ([]taskdao.TaskTag, error) {
	return nil, nil
}
func (repo *taskTagAssignmentRepo) AddToTask(_ context.Context, _ domain.UserID, _ domain.TaskID, tagID string) error {
	repo.added = tagID
	return repo.addErr
}
func (repo *taskTagAssignmentRepo) RemoveFromTask(_ context.Context, _ domain.UserID, _ domain.TaskID, tagID string) error {
	repo.removed = tagID
	return repo.removeErr
}

type taskTagAssignmentTaskRepo struct {
	TaskRepository
	task       taskdao.Task
	err        error
	capability shared.Capability
}

func (repo *taskTagAssignmentTaskRepo) GetByUserIDWithPermission(_ context.Context, _ domain.UserID, _ domain.TaskID, capability shared.Capability) (taskdao.Task, error) {
	repo.capability = capability
	return repo.task, repo.err
}

func (repo *taskTagAssignmentTaskRepo) LockByUserIDWithPermission(_ context.Context, _ domain.UserID, _ domain.TaskID, capability shared.Capability) (taskdao.Task, error) {
	repo.capability = capability
	return repo.task, repo.err
}

type taskTagAssignmentRepos struct {
	Repositories
	tasks TaskRepository
	tags  TaskTagRepository
}

func (repos taskTagAssignmentRepos) Tasks() TaskRepository       { return repos.tasks }
func (repos taskTagAssignmentRepos) TaskTags() TaskTagRepository { return repos.tags }

type taskTagAssignmentUOW struct{ repos Repositories }

func (uow taskTagAssignmentUOW) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, uow.repos)
}

func taskTagAssignmentFixture() (*taskTagAssignmentUOW, *taskTagAssignmentTaskRepo, *taskTagAssignmentRepo) {
	tasks := &taskTagAssignmentTaskRepo{task: taskdao.Task{ID: "task-1", UserID: "project-owner"}}
	tags := &taskTagAssignmentRepo{}
	repos := taskTagAssignmentRepos{tasks: tasks, tags: tags}
	return &taskTagAssignmentUOW{repos: repos}, tasks, tags
}

func TestTaskTagAssignmentUsesUpdatePermissionAndMapsRepositoryFailure(t *testing.T) {
	uow, tasks, tags := taskTagAssignmentFixture()
	err := NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), "editor", "task-1", "tag-1")
	if err != nil || tags.added != "tag-1" {
		t.Fatalf("add tag err/called = %v/%q", err, tags.added)
	}
	if tasks.capability.Action != shared.ActionUpdate {
		t.Fatalf("task capability = %+v, want update", tasks.capability)
	}

	wantErr := ErrTaskTagAssignmentNotOwned
	uow, _, tags = taskTagAssignmentFixture()
	tags.addErr = wantErr
	err = NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), "editor", "task-1", "foreign-tag")
	if !errors.Is(err, wantErr) || tags.added != "foreign-tag" {
		t.Fatalf("foreign tag err/add = %v/%q", err, tags.added)
	}
}

func TestTaskTagAssignmentPropagatesErrorsAndRemovesOwnedTag(t *testing.T) {
	uow, _, tags := taskTagAssignmentFixture()
	if err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), "editor", "task-1", "tag-2"); err != nil || tags.removed != "tag-2" {
		t.Fatalf("remove err/called = %v/%q", err, tags.removed)
	}

	wantErr := errors.New("task lookup failed")
	uow, tasks, _ := taskTagAssignmentFixture()
	tasks.err = wantErr
	if err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), "editor", "task-1", "tag-3"); !errors.Is(err, wantErr) {
		t.Fatalf("lookup error = %v, want %v", err, wantErr)
	}
}

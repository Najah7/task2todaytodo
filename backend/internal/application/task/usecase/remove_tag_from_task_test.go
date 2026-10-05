package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type removeTagFromTaskUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *removeTagFromTaskUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type removeTagFromTaskRepositoriesFake struct {
	taskProgressTestRepositories
	tasks    TaskRepository
	tags     TaskTagRepository
	accesses *[]string
}

func (repos removeTagFromTaskRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

func (repos removeTagFromTaskRepositoriesFake) TaskTags() TaskTagRepository {
	*repos.accesses = append(*repos.accesses, "task-tags")
	return repos.tags
}

type removeTagFromTaskTaskRepositoryFake struct {
	taskProgressTestRepository
	task     dao.Task
	err      error
	userID   domain.UserID
	taskID   domain.TaskID
	calls    int
	accesses *[]string
}

func (repo *removeTagFromTaskTaskRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "get-task")
	return repo.task, repo.err
}

func (repo *removeTagFromTaskTaskRepositoryFake) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}

type removeTagFromTaskTagRepositoryFake struct {
	TaskTagRepository
	tag          dao.TaskTag
	getErr       error
	removeErr    error
	getUserID    domain.UserID
	getTagID     domain.TaskTagID
	removeUserID domain.UserID
	removeTaskID domain.TaskID
	removeTagID  domain.TaskTagID
	getCalls     int
	removeCalls  int
	assignments  map[string]bool
	accesses     *[]string
}

func (repo *removeTagFromTaskTagRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, tagID domain.TaskTagID) (dao.TaskTag, error) {
	repo.getCalls++
	repo.getUserID, repo.getTagID = userID, tagID
	*repo.accesses = append(*repo.accesses, "get-tag")
	return repo.tag, repo.getErr
}

func (repo *removeTagFromTaskTagRepositoryFake) RemoveFromTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, tagID domain.TaskTagID) error {
	repo.removeCalls++
	repo.removeUserID, repo.removeTaskID, repo.removeTagID = userID, taskID, tagID
	*repo.accesses = append(*repo.accesses, "remove-tag")
	if repo.removeErr != nil {
		return repo.removeErr
	}
	delete(repo.assignments, string(taskID)+":"+string(tagID))
	return nil
}

func newRemoveTagFromTaskFixture(task dao.Task, taskErr error, tag dao.TaskTag, tagErr error) (*removeTagFromTaskUOWFake, *removeTagFromTaskTaskRepositoryFake, *removeTagFromTaskTagRepositoryFake, *[]string) {
	var accesses []string
	taskRepo := &removeTagFromTaskTaskRepositoryFake{task: task, err: taskErr, accesses: &accesses}
	tagRepo := &removeTagFromTaskTagRepositoryFake{
		tag: tag, getErr: tagErr,
		assignments: map[string]bool{"task-1:tag-1": true}, accesses: &accesses,
	}
	uow := &removeTagFromTaskUOWFake{repos: removeTagFromTaskRepositoriesFake{
		tasks: taskRepo, tags: tagRepo, accesses: &accesses,
	}}
	return uow, taskRepo, tagRepo, &accesses
}

func TestRemoveTagFromTaskUseCaseExecuteRemovesOwnersTagFromSharedTask(t *testing.T) {
	actorID, ownerID, taskID, tagID := domain.UserID("editor-1"), domain.UserID("owner-1"), domain.TaskID("task-1"), domain.TaskTagID("tag-1")
	uow, taskRepo, tagRepo, accesses := newRemoveTagFromTaskFixture(
		dao.Task{ID: string(taskID), UserID: string(ownerID)}, nil,
		dao.TaskTag{ID: string(tagID), UserID: string(ownerID), Name: "work"}, nil,
	)

	if err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), actorID, taskID, tagID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if uow.calls != 1 || taskRepo.calls != 1 || tagRepo.getCalls != 1 || tagRepo.removeCalls != 1 {
		t.Errorf("calls = UOW:%d task:%d tag lookup:%d removal:%d, want 1 each", uow.calls, taskRepo.calls, tagRepo.getCalls, tagRepo.removeCalls)
	}
	if taskRepo.userID != actorID || taskRepo.taskID != taskID || tagRepo.getUserID != ownerID || tagRepo.getTagID != tagID || tagRepo.removeUserID != actorID || tagRepo.removeTaskID != taskID || tagRepo.removeTagID != tagID {
		t.Errorf("repository arguments actor=%q task=%q tag owner=%q tag=%q removal actor=%q task=%q tag=%q; want actor-scoped task/write and owner-scoped tag lookup", taskRepo.userID, taskRepo.taskID, tagRepo.getUserID, tagRepo.getTagID, tagRepo.removeUserID, tagRepo.removeTaskID, tagRepo.removeTagID)
	}
	if !reflect.DeepEqual(*accesses, []string{"tasks", "get-task", "task-tags", "get-tag", "remove-tag"}) {
		t.Errorf("repository access order = %v, want ownership checks before assignment removal", *accesses)
	}
	if tagRepo.assignments["task-1:tag-1"] {
		t.Error("tag assignment remains, want removed")
	}
	if tagRepo.tag.ID != string(tagID) || tagRepo.tag.Name != "work" {
		t.Errorf("tag = %+v, want tag itself retained", tagRepo.tag)
	}
}

func TestRemoveTagFromTaskUseCaseExecuteAllowsUnassignedTag(t *testing.T) {
	uow, _, tagRepo, _ := newRemoveTagFromTaskFixture(
		dao.Task{ID: "task-1", UserID: "user-1"}, nil,
		dao.TaskTag{ID: "tag-1", UserID: "user-1"}, nil,
	)
	tagRepo.assignments = map[string]bool{}

	if err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1"); err != nil {
		t.Fatalf("Execute() error = %v, want idempotent success", err)
	}
	if tagRepo.removeCalls != 1 || len(tagRepo.assignments) != 0 {
		t.Errorf("removal calls = %d, assignments = %v; want idempotent no-op", tagRepo.removeCalls, tagRepo.assignments)
	}
}

func TestRemoveTagFromTaskUseCaseExecuteRejectsInvisibleTask(t *testing.T) {
	uow, _, tagRepo, _ := newRemoveTagFromTaskFixture(dao.Task{}, ErrTaskNotFound, dao.TaskTag{ID: "tag-1", UserID: "user-1"}, nil)
	err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), "editor-1", "task-1", "tag-1")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
	}
	if tagRepo.getCalls != 0 || tagRepo.removeCalls != 0 {
		t.Errorf("tag reads = %d, removals = %d, want no tag access for invisible task", tagRepo.getCalls, tagRepo.removeCalls)
	}
}

func TestRemoveTagFromTaskUseCaseExecuteRejectsMissingOrUnownedTag(t *testing.T) {
	for _, testCase := range []struct {
		name string
		tag  dao.TaskTag
		err  error
	}{
		{name: "missing tag", err: ErrTaskTagNotFound},
		{name: "tag owned by another user", tag: dao.TaskTag{ID: "tag-1", UserID: "other-user"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			uow, _, tagRepo, _ := newRemoveTagFromTaskFixture(dao.Task{ID: "task-1", UserID: "user-1"}, nil, testCase.tag, testCase.err)
			err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1")
			if !errors.Is(err, ErrTaskTagNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskTagNotFound)
			}
			if tagRepo.removeCalls != 0 {
				t.Errorf("RemoveFromTask() calls = %d, want 0 for missing or unowned tag", tagRepo.removeCalls)
			}
		})
	}
}

func TestRemoveTagFromTaskUseCaseExecutePropagatesAndRetainsAssignmentOnFailure(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &removeTagFromTaskUOWFake{err: wantErr}
	if err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1"); !errors.Is(err, wantErr) {
		t.Errorf("UOW error = %v, want %v", err, wantErr)
	}

	removeErr := errors.New("assignment removal failed")
	uow, _, tagRepo, _ := newRemoveTagFromTaskFixture(
		dao.Task{ID: "task-1", UserID: "user-1"}, nil,
		dao.TaskTag{ID: "tag-1", UserID: "user-1"}, nil,
	)
	tagRepo.removeErr = removeErr
	if err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1"); !errors.Is(err, removeErr) {
		t.Errorf("removal error = %v, want %v", err, removeErr)
	}
	if !tagRepo.assignments["task-1:tag-1"] {
		t.Error("failed removal changed assignment, want transaction rollback to retain it")
	}

	taskErr := errors.New("task lookup failed")
	uow, _, _, _ = newRemoveTagFromTaskFixture(dao.Task{}, taskErr, dao.TaskTag{}, nil)
	if err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1"); !errors.Is(err, taskErr) {
		t.Errorf("task lookup error = %v, want %v", err, taskErr)
	}

	tagErr := errors.New("tag lookup failed")
	uow, _, _, _ = newRemoveTagFromTaskFixture(dao.Task{ID: "task-1", UserID: "user-1"}, nil, dao.TaskTag{}, tagErr)
	if err := NewRemoveTagFromTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1"); !errors.Is(err, tagErr) {
		t.Errorf("tag lookup error = %v, want %v", err, tagErr)
	}
}

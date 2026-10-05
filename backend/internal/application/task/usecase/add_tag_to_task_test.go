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

type addTagToTaskUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *addTagToTaskUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type addTagToTaskRepositoriesFake struct {
	taskProgressTestRepositories
	tasks    TaskRepository
	tags     TaskTagRepository
	accesses *[]string
}

func (repos addTagToTaskRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

func (repos addTagToTaskRepositoriesFake) TaskTags() TaskTagRepository {
	*repos.accesses = append(*repos.accesses, "task-tags")
	return repos.tags
}

type addTagToTaskTaskRepositoryFake struct {
	taskProgressTestRepository
	task     dao.Task
	err      error
	userID   domain.UserID
	taskID   domain.TaskID
	calls    int
	accesses *[]string
}

func (repo *addTagToTaskTaskRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "get-task")
	if repo.err != nil {
		return dao.Task{}, repo.err
	}
	return repo.task, nil
}

func (repo *addTagToTaskTaskRepositoryFake) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}

type addTagToTaskTagRepositoryFake struct {
	TaskTagRepository
	tag         dao.TaskTag
	getErr      error
	addErr      error
	getUserID   domain.UserID
	getTagID    domain.TaskTagID
	addUserID   domain.UserID
	addTaskID   domain.TaskID
	addTagID    domain.TaskTagID
	getCalls    int
	addCalls    int
	assignments map[string]bool
	accesses    *[]string
}

func (repo *addTagToTaskTagRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, tagID domain.TaskTagID) (dao.TaskTag, error) {
	repo.getCalls++
	repo.getUserID, repo.getTagID = userID, tagID
	*repo.accesses = append(*repo.accesses, "get-tag")
	if repo.getErr != nil {
		return dao.TaskTag{}, repo.getErr
	}
	return repo.tag, nil
}

func (repo *addTagToTaskTagRepositoryFake) AddToTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, tagID domain.TaskTagID) error {
	repo.addCalls++
	repo.addUserID, repo.addTaskID, repo.addTagID = userID, taskID, tagID
	*repo.accesses = append(*repo.accesses, "add-tag")
	if repo.addErr != nil {
		return repo.addErr
	}
	if repo.assignments == nil {
		repo.assignments = make(map[string]bool)
	}
	// The production repository also makes this insert idempotent.
	repo.assignments[string(taskID)+":"+string(tagID)] = true
	return nil
}

func newAddTagToTaskFixture(task dao.Task, taskErr error, tag dao.TaskTag, tagErr error) (*addTagToTaskUOWFake, *addTagToTaskTaskRepositoryFake, *addTagToTaskTagRepositoryFake, *[]string) {
	var accesses []string
	taskRepo := &addTagToTaskTaskRepositoryFake{task: task, err: taskErr, accesses: &accesses}
	tagRepo := &addTagToTaskTagRepositoryFake{tag: tag, getErr: tagErr, accesses: &accesses}
	uow := &addTagToTaskUOWFake{repos: addTagToTaskRepositoriesFake{
		tasks:    taskRepo,
		tags:     tagRepo,
		accesses: &accesses,
	}}
	return uow, taskRepo, tagRepo, &accesses
}

func TestAddTagToTaskUseCaseExecuteAddsOwnersTagToSharedTask(t *testing.T) {
	actorID, ownerID, taskID, tagID := domain.UserID("editor-1"), domain.UserID("owner-1"), domain.TaskID("task-1"), domain.TaskTagID("tag-1")
	uow, taskRepo, tagRepo, accesses := newAddTagToTaskFixture(
		dao.Task{ID: string(taskID), UserID: string(ownerID)}, nil,
		dao.TaskTag{ID: string(tagID), UserID: string(ownerID)}, nil,
	)

	if err := NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), actorID, taskID, tagID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if uow.calls != 1 || taskRepo.calls != 1 || tagRepo.getCalls != 1 || tagRepo.addCalls != 1 {
		t.Errorf("calls = UOW:%d task:%d tag lookup:%d assignment:%d, want 1 each", uow.calls, taskRepo.calls, tagRepo.getCalls, tagRepo.addCalls)
	}
	if taskRepo.userID != actorID || taskRepo.taskID != taskID || tagRepo.getUserID != ownerID || tagRepo.getTagID != tagID || tagRepo.addUserID != actorID || tagRepo.addTaskID != taskID || tagRepo.addTagID != tagID {
		t.Errorf("repository arguments actor=%q task=%q tag owner=%q tag=%q assignment actor=%q task=%q tag=%q; want actor-scoped task/write and owner-scoped tag lookup", taskRepo.userID, taskRepo.taskID, tagRepo.getUserID, tagRepo.getTagID, tagRepo.addUserID, tagRepo.addTaskID, tagRepo.addTagID)
	}
	if !reflect.DeepEqual(*accesses, []string{"tasks", "get-task", "task-tags", "get-tag", "add-tag"}) {
		t.Errorf("repository access order = %v, want task and tag ownership checks before assignment", *accesses)
	}
}

func TestAddTagToTaskUseCaseExecuteAllowsDuplicateAssignment(t *testing.T) {
	userID, taskID, tagID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TaskTagID("tag-1")
	uow, _, tagRepo, _ := newAddTagToTaskFixture(
		dao.Task{ID: string(taskID), UserID: string(userID)}, nil,
		dao.TaskTag{ID: string(tagID), UserID: string(userID)}, nil,
	)
	tagRepo.assignments = map[string]bool{string(taskID) + ":" + string(tagID): true}

	if err := NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), userID, taskID, tagID); err != nil {
		t.Fatalf("Execute() error = %v, want nil for duplicate", err)
	}
	if tagRepo.addCalls != 1 || len(tagRepo.assignments) != 1 {
		t.Errorf("duplicate assignment calls = %d and stored assignments = %v, want idempotent success", tagRepo.addCalls, tagRepo.assignments)
	}
}

func TestAddTagToTaskUseCaseExecuteRejectsInvisibleTask(t *testing.T) {
	uow, _, tagRepo, _ := newAddTagToTaskFixture(dao.Task{}, ErrTaskNotFound, dao.TaskTag{ID: "tag-1", UserID: "user-1"}, nil)
	err := NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), "editor-1", "task-1", "tag-1")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
	}
	if tagRepo.getCalls != 0 || tagRepo.addCalls != 0 {
		t.Errorf("tag reads = %d, assignments = %d, want no tag access for invisible task", tagRepo.getCalls, tagRepo.addCalls)
	}
}

func TestAddTagToTaskUseCaseExecuteRejectsMissingOrUnownedTag(t *testing.T) {
	for _, testCase := range []struct {
		name string
		tag  dao.TaskTag
		err  error
	}{
		{name: "missing tag", err: ErrTaskTagNotFound},
		{name: "tag owned by another user", tag: dao.TaskTag{ID: "tag-1", UserID: "other-user"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			uow, _, tagRepo, _ := newAddTagToTaskFixture(dao.Task{ID: "task-1", UserID: "user-1"}, nil, testCase.tag, testCase.err)
			err := NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1")
			if !errors.Is(err, ErrTaskTagNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskTagNotFound)
			}
			if tagRepo.addCalls != 0 {
				t.Errorf("AddToTask() calls = %d, want 0 for missing or unowned tag", tagRepo.addCalls)
			}
		})
	}
}

func TestAddTagToTaskUseCaseExecutePropagatesUOWAndRepositoryErrors(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &addTagToTaskUOWFake{err: wantErr}
	if err := NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1"); !errors.Is(err, wantErr) {
		t.Errorf("UOW error = %v, want %v", err, wantErr)
	}

	taskErr := errors.New("task lookup failed")
	uow, _, _, _ = newAddTagToTaskFixture(dao.Task{}, taskErr, dao.TaskTag{}, nil)
	if err := NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1"); !errors.Is(err, taskErr) {
		t.Errorf("task lookup error = %v, want %v", err, taskErr)
	}

	tagErr := errors.New("tag lookup failed")
	uow, _, _, _ = newAddTagToTaskFixture(dao.Task{ID: "task-1", UserID: "user-1"}, nil, dao.TaskTag{}, tagErr)
	if err := NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1"); !errors.Is(err, tagErr) {
		t.Errorf("tag lookup error = %v, want %v", err, tagErr)
	}

	addErr := errors.New("assignment failed")
	uow, _, tagRepo, _ := newAddTagToTaskFixture(dao.Task{ID: "task-1", UserID: "user-1"}, nil, dao.TaskTag{ID: "tag-1", UserID: "user-1"}, nil)
	tagRepo.addErr = addErr
	if err := NewAddTagToTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "tag-1"); !errors.Is(err, addErr) {
		t.Errorf("assignment error = %v, want %v", err, addErr)
	}
}

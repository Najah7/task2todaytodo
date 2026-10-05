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

type deleteTaskUOWFake struct {
	repos       Repositories
	err         error
	calls       int
	callbackErr error
}

func (uow *deleteTaskUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	uow.callbackErr = fn(ctx, uow.repos)
	return uow.callbackErr
}

type deleteTaskRepositoriesFake struct {
	taskProgressTestRepositories
	tasks    TaskRepository
	accesses *[]string
}

func (repos deleteTaskRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

type deleteTaskTaskRepositoryFake struct {
	taskProgressTestRepository
	err              error
	lockErr          error
	userID           domain.UserID
	taskID           domain.TaskID
	lockCalls        int
	calls            int
	expectedRevision int32
	accesses         *[]string
}

func (repo *deleteTaskTaskRepositoryFake) LockByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, permission shared.Capability) (dao.Task, error) {
	repo.lockCalls++
	*repo.accesses = append(*repo.accesses, "lock-task")
	if permission != shared.TaskDelete() {
		return dao.Task{}, errors.New("unexpected task permission")
	}
	if repo.lockErr != nil {
		return dao.Task{}, repo.lockErr
	}
	return dao.Task{ID: string(taskID), UserID: string(userID), Revision: 1}, nil
}

func (repo *deleteTaskTaskRepositoryFake) DeleteByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32) error {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	repo.expectedRevision = expectedRevision
	*repo.accesses = append(*repo.accesses, "delete-task")
	return repo.err
}

func newDeleteTaskFixture(repoErr error) (*deleteTaskUOWFake, *deleteTaskTaskRepositoryFake, *[]string) {
	var accesses []string
	taskRepo := &deleteTaskTaskRepositoryFake{err: repoErr, accesses: &accesses}
	uow := &deleteTaskUOWFake{repos: deleteTaskRepositoriesFake{tasks: taskRepo, accesses: &accesses}}
	return uow, taskRepo, &accesses
}

func TestDeleteTaskUseCaseExecuteDeletesOwnedTaskInUOW(t *testing.T) {
	userID, taskID := domain.UserID("user-1"), domain.TaskID("task-1")
	uow, taskRepo, accesses := newDeleteTaskFixture(nil)

	err := NewDeleteTaskUseCase(uow, nil).Execute(context.Background(), userID, taskID, 1)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if uow.calls != 1 || taskRepo.lockCalls != 1 || taskRepo.calls != 1 {
		t.Fatalf("calls = UOW:%d lock:%d delete:%d, want 1 each", uow.calls, taskRepo.lockCalls, taskRepo.calls)
	}
	if taskRepo.userID != userID || taskRepo.taskID != taskID || taskRepo.expectedRevision != 1 {
		t.Errorf("DeleteByUserID() arguments = (%q, %q, %d), want (%q, %q, 1)", taskRepo.userID, taskRepo.taskID, taskRepo.expectedRevision, userID, taskID)
	}
	if !reflect.DeepEqual(*accesses, []string{"tasks", "lock-task", "tasks", "delete-task"}) {
		t.Errorf("repository access order = %v, want task lock then revision-checked delete", *accesses)
	}
}

func TestDeleteTaskUseCaseExecuteReturnsNotFoundForMissingOrUnownedTask(t *testing.T) {
	for _, name := range []string{"missing task", "task owned by another user"} {
		t.Run(name, func(t *testing.T) {
			uow, taskRepo, _ := newDeleteTaskFixture(nil)
			taskRepo.lockErr = ErrTaskNotFound

			err := NewDeleteTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", 1)
			if !errors.Is(err, ErrTaskNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
			}
			if uow.calls != 1 || taskRepo.lockCalls != 1 || taskRepo.calls != 0 {
				t.Errorf("calls = UOW:%d lock:%d delete:%d, want lock only", uow.calls, taskRepo.lockCalls, taskRepo.calls)
			}
		})
	}
}

func TestDeleteTaskUseCaseExecutePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	uow, taskRepo, _ := newDeleteTaskFixture(wantErr)

	err := NewDeleteTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", 1)
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if !errors.Is(uow.callbackErr, wantErr) {
		t.Errorf("unit of work callback error = %v, want %v", uow.callbackErr, wantErr)
	}
	if taskRepo.calls != 1 {
		t.Errorf("DeleteByUserID() calls = %d, want 1", taskRepo.calls)
	}
}

func TestDeleteTaskUseCaseExecutePropagatesUOWError(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &deleteTaskUOWFake{err: wantErr}

	err := NewDeleteTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", 1)
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if uow.calls != 1 || uow.callbackErr != nil {
		t.Errorf("UOW calls = %d and callback error = %v, want one call without callback", uow.calls, uow.callbackErr)
	}
}

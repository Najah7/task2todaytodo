package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type completeTaskUOWFake struct {
	repos       Repositories
	err         error
	callbackErr error
	calls       int
	committed   bool
	rolledBack  bool
}

func (uow *completeTaskUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	uow.callbackErr = fn(ctx, uow.repos)
	if uow.callbackErr != nil {
		uow.rolledBack = true
	} else {
		uow.committed = true
	}
	return uow.callbackErr
}

type completeTaskRepositoriesFake struct {
	taskProgressTestRepositories
	tasks     TaskRepository
	todoItems TodoItemRepository
	schedules TaskScheduleRepository
	accesses  *[]string
}

func (repos completeTaskRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

func (repos completeTaskRepositoriesFake) TodoItems() TodoItemRepository {
	*repos.accesses = append(*repos.accesses, "todo-items")
	return repos.todoItems
}

func (repos completeTaskRepositoriesFake) TaskSchedules() TaskScheduleRepository {
	*repos.accesses = append(*repos.accesses, "task-schedules")
	return repos.schedules
}

type completeTaskTaskRepositoryFake struct {
	taskProgressTestRepository
	task        dao.Task
	getErr      error
	updateErr   error
	getCalls    int
	updateCalls int
	userID      domain.UserID
	taskID      domain.TaskID
	updated     domain.Task
	accesses    *[]string
}

func (repo *completeTaskTaskRepositoryFake) LockByUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}

func (repo *completeTaskTaskRepositoryFake) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}

func (repo *completeTaskTaskRepositoryFake) SetStatusByUserID(_ context.Context, _ domain.UserID, _ domain.TaskID, status domain.TaskStatus) error {
	repo.updateCalls++
	*repo.accesses = append(*repo.accesses, "set-status")
	if repo.updateErr != nil {
		return repo.updateErr
	}
	repo.task.Status = dao.TaskStatus{Value: status.Value}
	repo.task.Revision++
	repo.updated.Status = status
	return nil
}

func (repo *completeTaskTaskRepositoryFake) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, expectedRevision int32, _ shared.Capability) error {
	if repo.task.Revision != expectedRevision {
		return ErrRevisionConflict
	}
	return repo.SetStatusByUserID(ctx, userID, taskID, status)
}

func (repo *completeTaskTaskRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.getCalls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "get-task")
	if repo.getErr != nil {
		return dao.Task{}, repo.getErr
	}
	if repo.task.ID == "" || repo.task.ID != string(taskID) || repo.task.UserID != string(userID) {
		return dao.Task{}, ErrTaskNotFound
	}
	return repo.task, nil
}

func (repo *completeTaskTaskRepositoryFake) UpdateByUserID(_ context.Context, userID domain.UserID, task domain.Task, expectedRevision int32) (dao.Task, error) {
	repo.updateCalls++
	repo.updated = task
	*repo.accesses = append(*repo.accesses, "update-task")
	if repo.updateErr != nil {
		return dao.Task{}, repo.updateErr
	}
	return dao.Task{ID: string(task.ID), UserID: string(task.UserID), Status: dao.TaskStatus{Value: task.Status.String()}, Revision: expectedRevision + 1}, nil
}

type completeTaskTodoItemRepositoryFake struct {
	TodoItemRepository
	err      error
	calls    int
	userID   domain.UserID
	taskID   domain.TaskID
	fromAt   time.Time
	accesses *[]string
}

func (repo *completeTaskTodoItemRepositoryFake) DeleteUneditedFutureByTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, fromAt time.Time) (int64, error) {
	repo.calls++
	repo.userID, repo.taskID, repo.fromAt = userID, taskID, fromAt
	*repo.accesses = append(*repo.accesses, "delete-future-todo-items")
	return 0, repo.err
}

type completeTaskScheduleRepositoryFake struct {
	TaskScheduleRepository
	err      error
	calls    int
	userID   domain.UserID
	taskID   domain.TaskID
	fromAt   time.Time
	accesses *[]string
}

func (repo *completeTaskScheduleRepositoryFake) DeleteUneditedFutureByTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, fromAt time.Time) error {
	repo.calls++
	repo.userID, repo.taskID, repo.fromAt = userID, taskID, fromAt
	*repo.accesses = append(*repo.accesses, "delete-future-schedules")
	return repo.err
}

func newCompleteTaskFixture(status string) (*CompleteTaskUseCase, *completeTaskUOWFake, *completeTaskTaskRepositoryFake, *completeTaskTodoItemRepositoryFake, *completeTaskScheduleRepositoryFake, *[]string) {
	var accesses []string
	taskRepo := &completeTaskTaskRepositoryFake{
		task: dao.Task{
			ID: "task-1", UserID: "user-1", AssigneeID: "user-1", Title: "Task", Progress: 37,
			Priority: dao.Priority{Value: "high"}, Status: dao.TaskStatus{Value: status},
			CreatedAt: 100, UpdatedAt: 200, Revision: 1,
		},
		accesses: &accesses,
	}
	todoRepo := &completeTaskTodoItemRepositoryFake{accesses: &accesses}
	scheduleRepo := &completeTaskScheduleRepositoryFake{accesses: &accesses}
	uow := &completeTaskUOWFake{repos: completeTaskRepositoriesFake{
		tasks: taskRepo, todoItems: todoRepo, schedules: scheduleRepo, accesses: &accesses,
	}}
	fixedNow := time.Date(2026, time.October, 3, 0, 30, 0, 0, time.UTC)
	uc := NewCompleteTaskUseCase(uow, func() time.Time { return fixedNow }, nil)
	return uc, uow, taskRepo, todoRepo, scheduleRepo, &accesses
}

func TestCompleteTaskUseCaseCompletesTaskAndListUsesTaskStatus(t *testing.T) {
	for _, status := range []string{"open", "in_progress", "pending", "waiting_on_others", "done"} {
		t.Run(status, func(t *testing.T) {
			uc, uow, taskRepo, todoRepo, scheduleRepo, accesses := newCompleteTaskFixture(status)
			userID, taskID := domain.UserID("user-1"), domain.TaskID("task-1")

			got, err := uc.Execute(context.Background(), userID, taskID, 1)
			if err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}
			if got.Revision != 2 || got.Status.Value != "done" {
				t.Errorf("Execute() = revision %d, status %q; want revision 2, done", got.Revision, got.Status.Value)
			}
			if uow.calls != 1 || taskRepo.getCalls != 2 || taskRepo.updateCalls != 1 || todoRepo.calls != 0 || scheduleRepo.calls != 0 {
				t.Fatalf("calls = UOW:%d get:%d update:%d todo:%d schedule:%d, want status update and persisted readback", uow.calls, taskRepo.getCalls, taskRepo.updateCalls, todoRepo.calls, scheduleRepo.calls)
			}
			if taskRepo.updated.Status.String() != "done" || taskRepo.task.Status.Value != "done" {
				t.Errorf("updated status = %q/%q, want done", taskRepo.updated.Status.String(), taskRepo.task.Status.Value)
			}
			wantCalls := []string{"tasks", "get-task", "set-status", "get-task"}
			if !reflect.DeepEqual(*accesses, wantCalls) {
				t.Errorf("transaction repository call order = %v", *accesses)
			}
		})
	}
}

func TestCompleteTaskUseCaseReturnsOwnershipErrorBeforeMutation(t *testing.T) {
	t.Run("missing or unowned task", func(t *testing.T) {
		uc, uow, taskRepo, todoRepo, scheduleRepo, _ := newCompleteTaskFixture("open")
		taskRepo.task.UserID = "other-user"
		if _, err := uc.Execute(context.Background(), "user-1", "task-1", 1); !errors.Is(err, ErrTaskNotFound) {
			t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
		}
		if uow.calls != 1 || taskRepo.getCalls != 1 || taskRepo.updateCalls != 0 || todoRepo.calls != 0 || scheduleRepo.calls != 0 {
			t.Errorf("calls = UOW:%d get:%d update:%d todo:%d schedule:%d, want only owner-scoped load", uow.calls, taskRepo.getCalls, taskRepo.updateCalls, todoRepo.calls, scheduleRepo.calls)
		}
	})
}

func TestCompleteTaskUseCasePropagatesTaskAndUnitOfWorkFailures(t *testing.T) {
	t.Run("task update", func(t *testing.T) {
		uc, uow, taskRepo, _, _, accesses := newCompleteTaskFixture("open")
		wantErr := errors.New("task update failed")
		taskRepo.updateErr = wantErr
		_, err := uc.Execute(context.Background(), "user-1", "task-1", 1)
		if !errors.Is(err, wantErr) || !uow.rolledBack || !reflect.DeepEqual(*accesses, []string{"tasks", "get-task", "set-status"}) {
			t.Errorf("Execute() error=%v rollback=%t calls=%v", err, uow.rolledBack, *accesses)
		}
	})
	t.Run("unit of work", func(t *testing.T) {
		uc, uow, taskRepo, _, _, _ := newCompleteTaskFixture("open")
		wantErr := errors.New("transaction failed")
		uow.err = wantErr
		if _, err := uc.Execute(context.Background(), "user-1", "task-1", 1); !errors.Is(err, wantErr) {
			t.Errorf("Execute() error=%v", err)
		}
		if uow.calls != 1 || uow.callbackErr != nil || taskRepo.getCalls != 0 {
			t.Errorf("unexpected calls UOW:%d callback:%v get:%d", uow.calls, uow.callbackErr, taskRepo.getCalls)
		}
	})
}

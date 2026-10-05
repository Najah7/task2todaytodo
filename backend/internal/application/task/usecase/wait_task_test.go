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

type waitTaskUOWFake struct {
	repos       Repositories
	err         error
	calls       int
	committed   bool
	rolledBack  bool
	callbackErr error
}

func (uow *waitTaskUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	uow.callbackErr = fn(ctx, uow.repos)
	if uow.callbackErr != nil {
		uow.rolledBack = true
		return uow.callbackErr
	}
	uow.committed = true
	return nil
}

type waitTaskRepositoriesFake struct {
	taskProgressTestRepositories
	tasks    TaskRepository
	accesses *[]string
}

func (repos *waitTaskRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

type waitTaskTaskRepositoryFake struct {
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

func (repo *waitTaskTaskRepositoryFake) LockByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.getCalls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "get-task")
	if repo.getErr != nil || repo.task.ID != string(taskID) || repo.task.UserID != string(userID) {
		return dao.Task{}, ErrTaskNotFound
	}
	return repo.task, nil
}

func (repo *waitTaskTaskRepositoryFake) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.LockByUserID(ctx, userID, taskID)
}

func (repo *waitTaskTaskRepositoryFake) SetStatusByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus) error {
	repo.updateCalls++
	*repo.accesses = append(*repo.accesses, "set-status")
	if repo.updateErr != nil {
		return repo.updateErr
	}
	updated, err := taskFromDAO(repo.task)
	if err != nil {
		return err
	}
	updated.Status = status
	repo.updated = updated
	repo.task.Status = dao.TaskStatus{Value: status.Value}
	repo.task.Revision++
	return nil
}

func (repo *waitTaskTaskRepositoryFake) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, expectedRevision int32, _ shared.Capability) error {
	if repo.task.Revision != expectedRevision {
		return ErrRevisionConflict
	}
	return repo.SetStatusByUserID(ctx, userID, taskID, status)
}

func (repo *waitTaskTaskRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.getCalls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "get-task")
	return repo.task, repo.getErr
}

func (repo *waitTaskTaskRepositoryFake) UpdateByUserID(_ context.Context, userID domain.UserID, task domain.Task, expectedRevision int32) (dao.Task, error) {
	repo.updateCalls++
	repo.updated = task
	*repo.accesses = append(*repo.accesses, "update-task")
	if repo.updateErr != nil {
		return dao.Task{}, repo.updateErr
	}
	return dao.Task{ID: string(task.ID), UserID: string(task.UserID), Status: dao.TaskStatus{Value: task.Status.String()}, Revision: expectedRevision + 1}, nil
}

func newWaitTaskFixture(status string) (*WaitTaskUseCase, *waitTaskUOWFake, *waitTaskTaskRepositoryFake, *waitTaskRepositoriesFake, *[]string) {
	var accesses []string
	minutes := 42
	tasks := &waitTaskTaskRepositoryFake{
		task: dao.Task{
			ID: "task-1", UserID: "user-1", AssigneeID: "user-1", ProjectID: "project-7", Title: "Ship release",
			Description: "Finalize notes", DueDate: 1_800_000_000, EstimatedMinutes: &minutes,
			ActualMinutes: nil, Progress: 37, Priority: dao.Priority{Value: "high"},
			Status: dao.TaskStatus{Value: status}, CreatedAt: 100, UpdatedAt: 200, Revision: 1,
		},
		accesses: &accesses,
	}
	repos := &waitTaskRepositoriesFake{tasks: tasks, accesses: &accesses}
	uow := &waitTaskUOWFake{repos: repos}
	uc := NewWaitTaskUseCase(uow, nil)
	return uc, uow, tasks, repos, &accesses
}

func TestWaitTaskUseCaseTransitions(t *testing.T) {
	for _, status := range []string{"open", "in_progress", "pending", "waiting_on_others", "done"} {
		t.Run(status, func(t *testing.T) {
			uc, uow, tasks, _, accesses := newWaitTaskFixture(status)
			userID, taskID := domain.UserID("user-1"), domain.TaskID("task-1")
			result, err := uc.Execute(context.Background(), userID, taskID, 1)
			if err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}
			if result.Revision != 2 || result.Status.Value != "waiting_on_others" {
				t.Fatalf("Execute() = revision %d status %q, want revision 2 waiting_on_others", result.Revision, result.Status.Value)
			}
			if uow.calls != 1 || !uow.committed || uow.rolledBack {
				t.Errorf("UOW = calls:%d committed:%t rolledBack:%t, want one commit", uow.calls, uow.committed, uow.rolledBack)
			}
			if tasks.getCalls != 2 || tasks.updateCalls != 1 || tasks.userID != userID || tasks.taskID != taskID {
				t.Errorf("task repository calls/scope = %d/%d user:%q task:%q, want locked read, update, and persisted readback", tasks.getCalls, tasks.updateCalls, tasks.userID, tasks.taskID)
			}
			if tasks.updated.Status.String() != "waiting_on_others" || tasks.updated.ID != taskID || tasks.updated.UserID != userID || tasks.updated.Title != "Ship release" || tasks.updated.Progress != 37 || tasks.updated.Priority.Value != "high" || tasks.updated.EstimatedMinutes == nil || *tasks.updated.EstimatedMinutes != 42 {
				t.Errorf("updated task = %#v, want waiting_on_others with existing fields preserved", tasks.updated)
			}

			wantAccesses := []string{"tasks", "get-task", "set-status", "get-task"}
			if !reflect.DeepEqual(*accesses, wantAccesses) {
				t.Errorf("operation order = %v, want %v", *accesses, wantAccesses)
			}
		})
	}
}

func TestWaitTaskUseCaseDoesNotMutateMissingOrUnownedTask(t *testing.T) {
	for _, test := range []struct {
		name   string
		task   dao.Task
		getErr error
	}{
		{name: "not found", getErr: ErrTaskNotFound},
		{name: "different owner", task: dao.Task{ID: "task-1", UserID: "someone-else", Title: "Task", Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "done"}}},
		{name: "different task", task: dao.Task{ID: "other-task", UserID: "user-1", Title: "Task", Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "done"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			uc, uow, tasks, _, _ := newWaitTaskFixture("done")
			tasks.task, tasks.getErr = test.task, test.getErr
			_, err := uc.Execute(context.Background(), "user-1", "task-1", 1)
			if !errors.Is(err, ErrTaskNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
			}
			if uow.calls != 1 || tasks.getCalls != 1 || tasks.updateCalls != 0 || !uow.rolledBack {
				t.Errorf("calls = UOW:%d get:%d update:%d rollback:%t, want owner-scoped read only", uow.calls, tasks.getCalls, tasks.updateCalls, uow.rolledBack)
			}
		})
	}
}

func TestWaitTaskUseCasePropagatesUOWError(t *testing.T) {
	uc, uow, tasks, _, _ := newWaitTaskFixture("done")
	wantErr := errors.New("transaction failed")
	uow.err = wantErr
	_, err := uc.Execute(context.Background(), "user-1", "task-1", 1)
	if !errors.Is(err, wantErr) || uow.calls != 1 || uow.callbackErr != nil || tasks.getCalls != 0 {
		t.Errorf("Execute() error=%v UOW=%d callback=%v get=%d, want original UOW error before callback", err, uow.calls, uow.callbackErr, tasks.getCalls)
	}
}

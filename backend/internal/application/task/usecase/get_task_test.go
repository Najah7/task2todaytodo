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

type getTaskUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *getTaskUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type getTaskRepositoriesFake struct {
	taskProgressTestRepositories
	tasks TaskRepository
	items TaskListProjectionReader
}

func (repos getTaskRepositoriesFake) ActionItems() ActionItemRepository {
	if items, ok := repos.items.(ActionItemRepository); ok {
		return items
	}
	return taskProgressTestActionItems{}
}

func (repos getTaskRepositoriesFake) Tasks() TaskRepository { return repos.tasks }

type getTaskTaskRepositoryFake struct {
	taskProgressTestRepository
	taskProgressSourceFake
	task            dao.Task
	err             error
	permission      bool
	permissionErr   error
	permissionCheck shared.Capability
	permissionCalls int
	userID          domain.UserID
	taskID          domain.TaskID
	calls           int
}

func (repo *getTaskTaskRepositoryFake) ReadTaskProgressSources(ctx context.Context, taskIDs []string, asOf time.Time) (dao.TaskProgressSources, error) {
	return repo.taskProgressSourceFake.ReadTaskProgressSources(ctx, taskIDs, asOf)
}

func (repo *getTaskTaskRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	if repo.err != nil {
		return dao.Task{}, repo.err
	}
	if repo.task.ID == "" || repo.task.UserID != string(userID) {
		return dao.Task{}, ErrTaskNotFound
	}
	return repo.task, nil
}

func (repo *getTaskTaskRepositoryFake) HasPermission(_ context.Context, _ domain.UserID, _ domain.TaskID, capability shared.Capability) (bool, error) {
	repo.permissionCalls++
	repo.permissionCheck = capability
	return repo.permission, repo.permissionErr
}

type getTaskProjectionFake struct{ rows []dao.ActionItem }

type getTaskActionItemsFake struct {
	taskProgressTestActionItems
	getTaskProjectionFake
}

func (reader getTaskActionItemsFake) ReadTaskListProjection(ctx context.Context, userID domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	return reader.getTaskProjectionFake.ReadTaskListProjection(ctx, userID, taskIDs)
}

func (reader getTaskProjectionFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	canCreate := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = reader.rows
		canCreate[id] = true
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items, CanCreateActionItems: canCreate}, nil
}

func TestGetTaskUseCaseExecuteReturnsOwnedTaskWithCapabilitiesAndProjection(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	wantTask := dao.Task{ID: string(taskID), UserID: string(userID), Title: "Owned task", Progress: 66}
	taskRepo := &getTaskTaskRepositoryFake{
		task: wantTask, permission: true,
		taskProgressSourceFake: taskProgressSourceFake{sources: dao.TaskProgressSources{
			Counts: map[string]dao.TaskProgressCounts{string(taskID): {Total: 3, Completed: 2}},
		}},
	}
	rows := []dao.ActionItem{
		{ID: "item-1", TaskID: string(taskID), Completed: true},
		{ID: "item-2", TaskID: string(taskID), Completed: true},
		{ID: "item-3", TaskID: string(taskID)},
	}
	wantTask.Progress, wantTask.ActionItemCount, wantTask.ActionItemCompletedCount = 66, 3, 2
	wantTask.EstimateSource = "action_items"
	wantTask.CanUpdate = true
	wantTask.EstimateEditable = true
	wantTask.CanAddActionItem = true
	uow := &getTaskUOWFake{repos: getTaskRepositoriesFake{
		tasks: taskRepo,
		items: getTaskActionItemsFake{getTaskProjectionFake: getTaskProjectionFake{rows: rows}},
	}}

	got, err := NewGetTaskUseCase(uow, nil).Execute(context.Background(), userID, taskID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, wantTask) {
		t.Errorf("Execute() = %#v, want %#v", got, wantTask)
	}
	if uow.calls != 1 || taskRepo.calls != 1 || taskRepo.permissionCalls != 1 {
		t.Errorf("calls = UOW:%d task:%d permission:%d, want 1 each", uow.calls, taskRepo.calls, taskRepo.permissionCalls)
	}
	if taskRepo.userID != userID || taskRepo.taskID != taskID || taskRepo.permissionCheck != shared.TaskUpdate() {
		t.Errorf("repository scope/capability = (%q,%q)/%+v, want (%q,%q)/task update", taskRepo.userID, taskRepo.taskID, taskRepo.permissionCheck, userID, taskID)
	}
}

func TestGetTaskUseCaseExecuteReturnsNotFoundForMissingOrUnownedTask(t *testing.T) {
	userID := domain.UserID("requesting-user")
	taskID := domain.TaskID("task-1")
	for _, testCase := range []struct {
		name string
		task dao.Task
		err  error
	}{
		{name: "missing task"},
		{name: "task owned by another user", task: dao.Task{ID: string(taskID), UserID: "other-user"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			taskRepo := &getTaskTaskRepositoryFake{task: testCase.task, err: testCase.err}
			uow := &getTaskUOWFake{repos: getTaskRepositoriesFake{tasks: taskRepo}}

			got, err := NewGetTaskUseCase(uow, nil).Execute(context.Background(), userID, taskID)
			if !errors.Is(err, ErrTaskNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
			}
			if !reflect.DeepEqual(got, dao.Task{}) {
				t.Errorf("Execute() = %#v, want zero result on error", got)
			}
			if taskRepo.calls != 1 || taskRepo.permissionCalls != 0 || taskRepo.userID != userID || taskRepo.taskID != taskID {
				t.Errorf("repository calls/scope = %d/%d (%q,%q); want one lookup before authorization", taskRepo.calls, taskRepo.permissionCalls, taskRepo.userID, taskRepo.taskID)
			}
		})
	}
}

func TestGetTaskUseCaseExecutePropagatesUOWError(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &getTaskUOWFake{err: wantErr}

	got, err := NewGetTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(got, dao.Task{}) {
		t.Errorf("Execute() = %#v, want zero result on error", got)
	}
	if uow.calls != 1 {
		t.Errorf("UOW calls = %d, want 1", uow.calls)
	}
}

func TestGetTaskUseCaseExecutePropagatesTaskRepositoryError(t *testing.T) {
	wantErr := errors.New("task query failed")
	taskRepo := &getTaskTaskRepositoryFake{err: wantErr}
	uow := &getTaskUOWFake{repos: getTaskRepositoriesFake{tasks: taskRepo}}

	got, err := NewGetTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(got, dao.Task{}) {
		t.Errorf("Execute() = %#v, want zero result on error", got)
	}
	if taskRepo.calls != 1 || taskRepo.permissionCalls != 0 {
		t.Errorf("task calls = lookup:%d permission:%d; want lookup only", taskRepo.calls, taskRepo.permissionCalls)
	}
}

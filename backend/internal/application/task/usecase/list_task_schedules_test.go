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

type listTaskSchedulesUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *listTaskSchedulesUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type listTaskSchedulesRepositoriesFake struct {
	taskProgressTestRepositories
	tasks     TaskRepository
	schedules TaskScheduleRepository
	accesses  *[]string
}

func (repos listTaskSchedulesRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

func (repos listTaskSchedulesRepositoriesFake) TaskSchedules() TaskScheduleRepository {
	*repos.accesses = append(*repos.accesses, "task-schedules")
	return repos.schedules
}

type listTaskSchedulesTaskRepositoryFake struct {
	taskProgressTestRepository
	task       dao.Task
	err        error
	userID     domain.UserID
	taskID     domain.TaskID
	permission shared.Capability
	calls      int
	accesses   *[]string
}

func (repo *listTaskSchedulesTaskRepositoryFake) GetByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, permission shared.Capability) (dao.Task, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	repo.permission = permission
	*repo.accesses = append(*repo.accesses, "get-task")
	if repo.err != nil {
		return dao.Task{}, repo.err
	}
	return repo.task, nil
}

type listTaskSchedulesRepositoryFake struct {
	TaskScheduleRepository
	schedules []dao.TaskSchedule
	err       error
	userID    domain.UserID
	taskID    domain.TaskID
	calls     int
	accesses  *[]string
}

func (repo *listTaskSchedulesRepositoryFake) ListByTaskAndUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskSchedule, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "list-task-schedules")
	return repo.schedules, repo.err
}

func TestListTaskSchedulesUseCaseExecuteReturnsReadableSharedTaskSchedulesInRepositoryOrder(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	wantSchedules := []dao.TaskSchedule{
		{ID: "schedule-2", TaskID: string(taskID), StartAt: 200, SeriesID: "schedule-2"},
		{ID: "schedule-1", TaskID: string(taskID), StartAt: 200, SeriesID: "schedule-1"},
		{ID: "schedule-3", TaskID: string(taskID), StartAt: 300, SeriesID: "schedule-1"},
	}
	var accesses []string
	taskRepo := &listTaskSchedulesTaskRepositoryFake{task: dao.Task{ID: string(taskID), UserID: "owner-1"}, accesses: &accesses}
	schedulesRepo := &listTaskSchedulesRepositoryFake{schedules: wantSchedules, accesses: &accesses}
	uow := &listTaskSchedulesUOWFake{repos: listTaskSchedulesRepositoriesFake{
		tasks: taskRepo, schedules: schedulesRepo, accesses: &accesses,
	}}

	got, err := NewListTaskSchedulesUseCase(uow, nil).Execute(context.Background(), userID, taskID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, wantSchedules) {
		t.Errorf("Execute() = %#v, want repository order %#v", got, wantSchedules)
	}
	if uow.calls != 1 || taskRepo.calls != 1 || schedulesRepo.calls != 1 {
		t.Errorf("calls = UOW:%d task:%d schedules:%d, want 1 each", uow.calls, taskRepo.calls, schedulesRepo.calls)
	}
	if taskRepo.userID != userID || taskRepo.taskID != taskID || taskRepo.permission != shared.TaskScheduleRead() || schedulesRepo.userID != userID || schedulesRepo.taskID != taskID {
		t.Errorf("repository scope = task(%q,%q), schedules(%q,%q), want user %q and task %q", taskRepo.userID, taskRepo.taskID, schedulesRepo.userID, schedulesRepo.taskID, userID, taskID)
	}
	if !reflect.DeepEqual(accesses, []string{"tasks", "get-task", "task-schedules", "list-task-schedules"}) {
		t.Errorf("repository access order = %v, want task permission check before schedule listing", accesses)
	}
}

func TestListTaskSchedulesUseCaseExecuteReturnsNonNilEmptyList(t *testing.T) {
	var accesses []string
	taskRepo := &listTaskSchedulesTaskRepositoryFake{task: dao.Task{UserID: "user-1"}, accesses: &accesses}
	schedulesRepo := &listTaskSchedulesRepositoryFake{accesses: &accesses}
	uow := &listTaskSchedulesUOWFake{repos: listTaskSchedulesRepositoriesFake{
		tasks: taskRepo, schedules: schedulesRepo, accesses: &accesses,
	}}

	got, err := NewListTaskSchedulesUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("Execute() = %#v, want non-nil empty list", got)
	}
}

func TestListTaskSchedulesUseCaseExecuteStopsWhenTaskScheduleReadIsDenied(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	var accesses []string
	taskRepo := &listTaskSchedulesTaskRepositoryFake{err: ErrTaskNotFound, accesses: &accesses}
	schedulesRepo := &listTaskSchedulesRepositoryFake{schedules: []dao.TaskSchedule{{ID: "must-not-be-read"}}, accesses: &accesses}
	uow := &listTaskSchedulesUOWFake{repos: listTaskSchedulesRepositoriesFake{
		tasks: taskRepo, schedules: schedulesRepo, accesses: &accesses,
	}}

	got, err := NewListTaskSchedulesUseCase(uow, nil).Execute(context.Background(), userID, taskID)
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
	}
	if got != nil {
		t.Errorf("Execute() = %#v, want nil result on error", got)
	}
	if schedulesRepo.calls != 0 {
		t.Errorf("ListByTaskAndUserID() calls = %d, want 0 when task_schedule/read is denied", schedulesRepo.calls)
	}
}

func TestListTaskSchedulesUseCaseExecutePropagatesRepositoryErrors(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	wantErr := errors.New("database unavailable")
	t.Run("task repository", func(t *testing.T) {
		var accesses []string
		taskRepo := &listTaskSchedulesTaskRepositoryFake{err: wantErr, accesses: &accesses}
		schedulesRepo := &listTaskSchedulesRepositoryFake{accesses: &accesses}
		uow := &listTaskSchedulesUOWFake{repos: listTaskSchedulesRepositoriesFake{
			tasks: taskRepo, schedules: schedulesRepo, accesses: &accesses,
		}}
		got, err := NewListTaskSchedulesUseCase(uow, nil).Execute(context.Background(), userID, taskID)
		if !errors.Is(err, wantErr) || got != nil || schedulesRepo.calls != 0 {
			t.Errorf("Execute() = (%#v, %v), schedule calls %d; want nil result, original error, 0", got, err, schedulesRepo.calls)
		}
	})
	t.Run("task schedule repository", func(t *testing.T) {
		var accesses []string
		taskRepo := &listTaskSchedulesTaskRepositoryFake{task: dao.Task{UserID: string(userID)}, accesses: &accesses}
		schedulesRepo := &listTaskSchedulesRepositoryFake{err: wantErr, accesses: &accesses}
		uow := &listTaskSchedulesUOWFake{repos: listTaskSchedulesRepositoriesFake{
			tasks: taskRepo, schedules: schedulesRepo, accesses: &accesses,
		}}
		got, err := NewListTaskSchedulesUseCase(uow, nil).Execute(context.Background(), userID, taskID)
		if !errors.Is(err, wantErr) || got != nil {
			t.Errorf("Execute() = (%#v, %v), want nil result and original error", got, err)
		}
	})
}

func TestListTaskSchedulesUseCaseExecutePropagatesUOWError(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &listTaskSchedulesUOWFake{err: wantErr}

	got, err := NewListTaskSchedulesUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("Execute() = %#v, want nil result", got)
	}
	if uow.calls != 1 {
		t.Errorf("UOW calls = %d, want 1", uow.calls)
	}
}

package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type deleteTaskScheduleUOWFake struct {
	repos       Repositories
	err         error
	calls       int
	callbackErr error
}

func (uow *deleteTaskScheduleUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	uow.callbackErr = fn(ctx, uow.repos)
	return uow.callbackErr
}

type deleteTaskScheduleRepositoriesFake struct {
	taskProgressTestRepositories
	repo     TaskScheduleRepository
	accesses *[]string
}

func (repos deleteTaskScheduleRepositoriesFake) TaskSchedules() TaskScheduleRepository {
	*repos.accesses = append(*repos.accesses, "task-schedules")
	return repos.repo
}

type deleteTaskScheduleRepositoryFake struct {
	TaskScheduleRepository
	schedule dao.TaskSchedule

	getErr          error
	tombstoneErr    error
	deleteFutureErr error
	userID          domain.UserID
	taskID          domain.TaskID
	scheduleID      domain.TaskScheduleID
	seriesID        domain.TaskScheduleID
	fromAt          time.Time
	getCalls        int
	tombstoneCalls  int
	deleteFuture    int
	accesses        *[]string
}

func (repo *deleteTaskScheduleRepositoryFake) GetByTaskAndUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, scheduleID domain.TaskScheduleID) (dao.TaskSchedule, error) {
	repo.getCalls++
	repo.userID, repo.taskID, repo.scheduleID = userID, taskID, scheduleID
	*repo.accesses = append(*repo.accesses, "get-owned-schedule")
	if repo.getErr != nil {
		return dao.TaskSchedule{}, repo.getErr
	}
	return repo.schedule, nil
}

func (repo *deleteTaskScheduleRepositoryFake) TombstoneByTaskAndUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, scheduleID domain.TaskScheduleID) error {
	repo.tombstoneCalls++
	repo.userID, repo.taskID, repo.scheduleID = userID, taskID, scheduleID
	*repo.accesses = append(*repo.accesses, "tombstone-schedule")
	return repo.tombstoneErr
}

func (repo *deleteTaskScheduleRepositoryFake) DeleteUneditedFutureBySeries(_ context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, fromAt time.Time) error {
	repo.deleteFuture++
	repo.userID, repo.taskID, repo.seriesID, repo.fromAt = userID, taskID, seriesID, fromAt
	*repo.accesses = append(*repo.accesses, "delete-unedited-future")
	return repo.deleteFutureErr
}

func newDeleteTaskScheduleFixture(schedule dao.TaskSchedule) (*deleteTaskScheduleUOWFake, *deleteTaskScheduleRepositoryFake, *[]string) {
	var accesses []string
	repo := &deleteTaskScheduleRepositoryFake{schedule: schedule, accesses: &accesses}
	uow := &deleteTaskScheduleUOWFake{repos: deleteTaskScheduleRepositoriesFake{repo: repo, accesses: &accesses}}
	return uow, repo, &accesses
}

func TestDeleteTaskScheduleUseCaseTombstonesRecurringRootAndKeepsHistory(t *testing.T) {
	userID, taskID, scheduleID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TaskScheduleID("series-root")
	uow, repo, accesses := newDeleteTaskScheduleFixture(dao.TaskSchedule{
		ID: string(scheduleID), TaskID: string(taskID), IntervalWeeks: 1,
		SeriesID: string(scheduleID), Timezone: "Asia/Tokyo",
	})
	err := NewDeleteTaskScheduleUseCase(uow, nil).Execute(context.Background(), userID, taskID, scheduleID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if uow.calls != 1 || repo.getCalls != 1 || repo.tombstoneCalls != 1 || repo.deleteFuture != 0 {
		t.Errorf("calls = UOW:%d get:%d tombstone:%d delete future:%d", uow.calls, repo.getCalls, repo.tombstoneCalls, repo.deleteFuture)
	}
	if repo.userID != userID || repo.taskID != taskID || repo.scheduleID != scheduleID {
		t.Errorf("repository scope = user:%q task:%q schedule:%q, want %q/%q/%q", repo.userID, repo.taskID, repo.scheduleID, userID, taskID, scheduleID)
	}
	if !reflect.DeepEqual(*accesses, []string{"task-schedules", "get-owned-schedule", "tombstone-schedule"}) {
		t.Errorf("repository access order = %v, want load and tombstone root", *accesses)
	}
}

func TestDeleteTaskScheduleUseCaseTombstonesOneOffScheduleWithoutFutureCleanup(t *testing.T) {
	userID, taskID, scheduleID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TaskScheduleID("single")
	uow, repo, accesses := newDeleteTaskScheduleFixture(dao.TaskSchedule{
		ID: string(scheduleID), TaskID: string(taskID), IntervalWeeks: 0,
		SeriesID: string(scheduleID), Timezone: "Asia/Tokyo",
	})

	if err := NewDeleteTaskScheduleUseCase(uow, nil).Execute(context.Background(), userID, taskID, scheduleID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.getCalls != 1 || repo.tombstoneCalls != 1 || repo.deleteFuture != 0 {
		t.Errorf("calls = get:%d tombstone:%d delete future:%d, want 1, 1, 0", repo.getCalls, repo.tombstoneCalls, repo.deleteFuture)
	}
	if !reflect.DeepEqual(*accesses, []string{"task-schedules", "get-owned-schedule", "tombstone-schedule"}) {
		t.Errorf("repository access order = %v, want only load and tombstone one-off schedule", *accesses)
	}
}

func TestDeleteTaskScheduleUseCaseRejectsCompletedOneOff(t *testing.T) {
	uow, repo, _ := newDeleteTaskScheduleFixture(dao.TaskSchedule{ID: "one-off", SeriesID: "one-off", Completed: true, RepeatState: repeatStateOneOff, Timezone: "UTC"})
	err := NewDeleteTaskScheduleUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "one-off")
	if !errors.Is(err, ErrOccurrenceCompleted) || repo.tombstoneCalls != 0 {
		t.Fatalf("Execute() error=%v tombstones=%d; want completed conflict and no mutation", err, repo.tombstoneCalls)
	}
}

func TestDeleteTaskScheduleUseCaseTombstonesGeneratedOccurrenceWithoutDeletingSeries(t *testing.T) {
	userID, taskID, scheduleID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TaskScheduleID("occurrence-2026-10-04")
	uow, repo, accesses := newDeleteTaskScheduleFixture(dao.TaskSchedule{
		ID: string(scheduleID), TaskID: string(taskID), IntervalWeeks: 1,
		SeriesID: "series-root", Timezone: "Asia/Tokyo",
	})

	if err := NewDeleteTaskScheduleUseCase(uow, nil).Execute(context.Background(), userID, taskID, scheduleID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.getCalls != 1 || repo.tombstoneCalls != 1 || repo.deleteFuture != 0 {
		t.Errorf("calls = get:%d tombstone:%d delete future:%d, want 1, 1, 0", repo.getCalls, repo.tombstoneCalls, repo.deleteFuture)
	}
	if !reflect.DeepEqual(*accesses, []string{"task-schedules", "get-owned-schedule", "tombstone-schedule"}) {
		t.Errorf("repository access order = %v, want only load and tombstone this occurrence", *accesses)
	}
}

func TestDeleteTaskScheduleUseCaseReturnsNotFoundForMissingOrUnownedSchedule(t *testing.T) {
	wantErr := domain.ErrTaskScheduleNotFound
	uow, repo, _ := newDeleteTaskScheduleFixture(dao.TaskSchedule{ID: "schedule-1", SeriesID: "schedule-1", IntervalWeeks: 1, Timezone: "UTC"})
	repo.getErr = wantErr

	err := NewDeleteTaskScheduleUseCase(uow, nil).Execute(context.Background(), "other-user", "task-1", "schedule-1")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if repo.getCalls != 1 || repo.tombstoneCalls != 0 || repo.deleteFuture != 0 {
		t.Errorf("calls = get:%d tombstone:%d delete future:%d, want 1, 0, 0", repo.getCalls, repo.tombstoneCalls, repo.deleteFuture)
	}
	if repo.userID != "other-user" {
		t.Errorf("GetByTaskAndUserID() user ID = %q, want caller %q", repo.userID, "other-user")
	}
}

func TestDeleteTaskScheduleUseCaseStopsWhenTombstoneFails(t *testing.T) {
	wantErr := errors.New("database unavailable")
	uow, repo, _ := newDeleteTaskScheduleFixture(dao.TaskSchedule{ID: "root", SeriesID: "root", IntervalWeeks: 1, Timezone: "UTC"})
	repo.tombstoneErr = wantErr

	err := NewDeleteTaskScheduleUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "root")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if repo.deleteFuture != 0 {
		t.Errorf("DeleteUneditedFutureBySeries() calls = %d, want 0 after tombstone failure", repo.deleteFuture)
	}
}

func TestDeleteTaskScheduleUseCasePropagatesUOWErrors(t *testing.T) {
	t.Run("unit of work", func(t *testing.T) {
		wantErr := errors.New("transaction failed")
		uow := &deleteTaskScheduleUOWFake{err: wantErr}

		err := NewDeleteTaskScheduleUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "schedule-1")
		if !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
		if uow.calls != 1 || uow.callbackErr != nil {
			t.Errorf("UOW calls = %d and callback error = %v, want 1 call without callback", uow.calls, uow.callbackErr)
		}
	})
}

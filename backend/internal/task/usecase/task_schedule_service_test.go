package usecase

import (
	"context"
	"errors"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	"testing"
	"time"
)

func TestTaskScheduleServiceCreateDefaultsIntervalAndScopesByUser(t *testing.T) {
	repo := &fakeTaskScheduleRepository{}
	service := NewTaskScheduleService(repo)
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)

	got, err := service.Create(context.Background(), fixedTaskIDGen("schedule-1"), TaskScheduleCreateParams{
		UserID:          "user-1",
		TaskID:          "task-1",
		Title:           "Focus block",
		Description:     "Deep work",
		Location:        "Home",
		FrequencyValues: []string{"mon", "fri"},
		StartAt:         startAt,
		EndAt:           endAt,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got.ID != "schedule-1" || got.TaskID != "task-1" || got.IntervalWeeks != domain.OnceIntervalWeeks {
		t.Fatalf("task schedule = %+v, want generated ID and one-off default", got)
	}
	if len(got.Frequencies) != 2 || got.Frequencies[0].String() != "mon" || got.Frequencies[1].String() != "fri" {
		t.Fatalf("frequencies = %+v, want parsed frequency values", got.Frequencies)
	}
	if repo.createUserID != "user-1" || repo.createCalls != 1 {
		t.Fatalf("repo create user=%q calls=%d, want owned-task create", repo.createUserID, repo.createCalls)
	}
}

func TestTaskScheduleServiceCreatePropagatesTaskNotFound(t *testing.T) {
	repo := &fakeTaskScheduleRepository{createErr: domain.ErrTaskScheduleTaskNotFound}
	service := NewTaskScheduleService(repo)
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)

	_, err := service.Create(context.Background(), fixedTaskIDGen("schedule-1"), TaskScheduleCreateParams{
		UserID:  "user-1",
		TaskID:  "task-1",
		Title:   "Focus block",
		StartAt: startAt,
		EndAt:   startAt.Add(time.Hour),
	})
	if !errors.Is(err, domain.ErrTaskScheduleTaskNotFound) {
		t.Fatalf("Create() error = %v, want domain.ErrTaskScheduleTaskNotFound", err)
	}
}

func TestTaskScheduleServiceUpdateMergesPartialFields(t *testing.T) {
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)
	repo := &fakeTaskScheduleRepository{
		scheduleByTaskAndUser: mustExistingTaskScheduleForService(t, "schedule-1", "task-1", "Old title", "Old description", "Office", 4, []string{"mon"}, startAt, endAt),
	}
	service := NewTaskScheduleService(repo)
	title := "New title"
	location := ""
	frequencies := []string{"sat"}

	got, err := service.Update(context.Background(), TaskScheduleUpdateParams{
		UserID:          "user-1",
		TaskID:          "task-1",
		ID:              "schedule-1",
		Title:           &title,
		Location:        &location,
		FrequencyValues: &frequencies,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if got.Title != "New title" || got.Location != "" {
		t.Fatalf("task schedule = %+v, want requested fields updated", got)
	}
	if got.Description != "Old description" || got.IntervalWeeks != 4 || got.StartAt != startAt || got.EndAt != endAt {
		t.Fatalf("task schedule = %+v, want omitted fields preserved", got)
	}
	if len(got.Frequencies) != 1 || got.Frequencies[0].String() != "sat" {
		t.Fatalf("frequencies = %+v, want updated frequency values", got.Frequencies)
	}
	if repo.getUserID != "user-1" || repo.getTaskID != "task-1" || repo.getScheduleID != "schedule-1" || repo.updateUserID != "user-1" {
		t.Fatalf("repo calls = %+v, want user/task-scoped update", repo)
	}
}

func TestTaskScheduleServiceUpdateRejectsInvalidFrequency(t *testing.T) {
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	repo := &fakeTaskScheduleRepository{
		scheduleByTaskAndUser: mustExistingTaskScheduleForService(t, "schedule-1", "task-1", "Schedule", "", "", 1, nil, startAt, startAt.Add(time.Hour)),
	}
	service := NewTaskScheduleService(repo)
	frequencies := []string{"daily"}

	_, err := service.Update(context.Background(), TaskScheduleUpdateParams{
		UserID:          "user-1",
		TaskID:          "task-1",
		ID:              "schedule-1",
		FrequencyValues: &frequencies,
	})
	if !errors.Is(err, domain.ErrTaskFrequencyInvalid) {
		t.Fatalf("Update() error = %v, want domain.ErrTaskFrequencyInvalid", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("updateCalls = %d, want repository update not called", repo.updateCalls)
	}
}

func TestTaskScheduleServiceDeleteScopesByUser(t *testing.T) {
	repo := &fakeTaskScheduleRepository{}
	service := NewTaskScheduleService(repo)

	if err := service.Delete(context.Background(), "user-1", "task-1", "schedule-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if repo.deleteUserID != "user-1" || repo.deleteTaskID != "task-1" || repo.deleteScheduleID != "schedule-1" {
		t.Fatalf("delete scope = %+v, want user/task-scoped delete", repo)
	}
}

func TestTaskScheduleServiceDeletePropagatesRepositoryError(t *testing.T) {
	repo := &fakeTaskScheduleRepository{deleteErr: domain.ErrTaskScheduleNotFound}
	service := NewTaskScheduleService(repo)

	err := service.Delete(context.Background(), "user-1", "task-1", "schedule-1")
	if !errors.Is(err, domain.ErrTaskScheduleNotFound) {
		t.Fatalf("Delete() error = %v, want domain.ErrTaskScheduleNotFound", err)
	}
}

type fakeTaskScheduleRepository struct {
	TaskScheduleRepository

	createCalls           int
	createUserID          domain.UserID
	created               domain.TaskSchedule
	createErr             error
	scheduleByTaskAndUser domain.TaskSchedule
	getUserID             domain.UserID
	getTaskID             domain.TaskID
	getScheduleID         domain.TaskScheduleID
	getErr                error
	updateCalls           int
	updateUserID          domain.UserID
	updated               domain.TaskSchedule
	updateErr             error
	deleteUserID          domain.UserID
	deleteTaskID          domain.TaskID
	deleteScheduleID      domain.TaskScheduleID
	deleteErr             error
}

func (r *fakeTaskScheduleRepository) CreateByTaskAndUser(_ context.Context, userID domain.UserID, schedule domain.TaskSchedule) (domain.TaskSchedule, error) {
	r.createCalls++
	r.createUserID = userID
	r.created = schedule
	if r.createErr != nil {
		return domain.NewZeroTaskSchedule(), r.createErr
	}
	return schedule, nil
}

func (r *fakeTaskScheduleRepository) GetByTaskAndUser(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) (domain.TaskSchedule, error) {
	r.getUserID = userID
	r.getTaskID = taskID
	r.getScheduleID = id
	if r.getErr != nil {
		return domain.NewZeroTaskSchedule(), r.getErr
	}
	return r.scheduleByTaskAndUser, nil
}

func (r *fakeTaskScheduleRepository) UpdateByTaskAndUser(_ context.Context, userID domain.UserID, schedule domain.TaskSchedule) (domain.TaskSchedule, error) {
	r.updateCalls++
	r.updateUserID = userID
	r.updated = schedule
	if r.updateErr != nil {
		return domain.NewZeroTaskSchedule(), r.updateErr
	}
	return schedule, nil
}

func (r *fakeTaskScheduleRepository) DeleteByTaskAndUser(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) error {
	r.deleteUserID = userID
	r.deleteTaskID = taskID
	r.deleteScheduleID = id
	return r.deleteErr
}

func mustExistingTaskScheduleForService(
	t *testing.T,
	id domain.TaskScheduleID,
	taskID domain.TaskID,
	title string,
	description string,
	location string,
	intervalWeeks int,
	frequencyValues []string,
	startAt time.Time,
	endAt time.Time,
) domain.TaskSchedule {
	t.Helper()
	frequencies := make(domain.TaskFrequencies, 0, len(frequencyValues))
	for _, value := range frequencyValues {
		frequencies = append(frequencies, mustTaskFrequencyForService(t, value))
	}
	schedule, err := domain.NewExistingTaskSchedule(
		id,
		taskID,
		title,
		description,
		location,
		intervalWeeks,
		frequencies,
		startAt,
		endAt,
		startAt.Add(-time.Hour),
		startAt.Add(-time.Minute),
	)
	if err != nil {
		t.Fatalf("domain.NewExistingTaskSchedule() error = %v", err)
	}
	return schedule
}

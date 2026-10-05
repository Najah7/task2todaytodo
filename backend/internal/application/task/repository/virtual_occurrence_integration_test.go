package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type occurrenceTestID struct{}

func (occurrenceTestID) Generate() string { return ulid.Make().String() }

type occurrenceTestRepositories struct {
	usecase.Repositories
	task     usecase.TaskRepository
	todo     usecase.TodoItemRepository
	schedule usecase.TaskScheduleRepository
}

func (r occurrenceTestRepositories) Tasks() usecase.TaskRepository                 { return r.task }
func (r occurrenceTestRepositories) TodoItems() usecase.TodoItemRepository         { return r.todo }
func (r occurrenceTestRepositories) TaskSchedules() usecase.TaskScheduleRepository { return r.schedule }

type occurrenceTestUOW struct{ pool *pgxpool.Pool }

func (u occurrenceTestUOW) Do(ctx context.Context, fn func(context.Context, usecase.Repositories) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repos := occurrenceTestRepositories{
		task: NewTaskRepository(tx), todo: NewTodoItemRepository(tx), schedule: NewTaskScheduleRepository(tx),
	}
	if err := fn(ctx, repos); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestVirtualOccurrenceMutationsPersistRealULIDs(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	uow := occurrenceTestUOW{pool: pool}
	id := occurrenceTestID{}
	monday, err := domain.NewTaskFrequency("mon")
	if err != nil {
		t.Fatal(err)
	}
	rootDate := recurrenceDate("2026-10-05")

	todoID := domain.TodoItemID(id.Generate())
	todo, err := domain.NewTodoItemWithDetails(todoID, taskID, "Weekly work", "", rootDate, false, 0, 1, domain.TaskFrequencies{monday})
	if err != nil {
		t.Fatal(err)
	}
	todo, err = todo.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(todoID), OccurrenceDate: rootDate, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTodoItemRepository(pool).CreateForOwnedTask(ctx, userID, todo, false); err != nil {
		t.Fatal(err)
	}

	title := "Edited weekly work"
	updated, err := usecase.NewUpdateTodoItemUseCase(uow, nil, id).ExecuteOccurrence(ctx, userID, taskID, todoID, "2026-10-12", "current", usecase.PatchField[string]{Present: true, Value: &title}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{})
	if err != nil {
		t.Fatalf("update virtual todo: %v", err)
	}
	assertRealOccurrenceID(t, updated.ID)
	if err := usecase.NewCompleteTodoItemUseCase(uow, nil, id).ExecuteOccurrence(ctx, userID, taskID, todoID, "2026-10-19"); err != nil {
		t.Fatalf("complete virtual todo: %v", err)
	}

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, location)
	scheduleID := domain.TaskScheduleID(id.Generate())
	schedule, err := domain.NewTaskScheduleWithDetails(scheduleID, taskID, "Weekly meeting", "", "", 1, domain.TaskFrequencies{monday}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(scheduleID), OccurrenceDate: rootDate, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTaskScheduleRepository(pool).CreateByTaskAndUserID(ctx, userID, schedule); err != nil {
		t.Fatal(err)
	}

	scheduleTitle := "Edited weekly meeting"
	updatedSchedule, err := usecase.NewUpdateTaskScheduleUseCase(uow, nil, id).ExecuteOccurrence(ctx, userID, taskID, scheduleID, "2026-10-12", "current", usecase.PatchField[string]{Present: true, Value: &scheduleTitle}, usecase.PatchField[string]{}, usecase.PatchField[string]{})
	if err != nil {
		t.Fatalf("update virtual schedule: %v", err)
	}
	assertRealOccurrenceID(t, updatedSchedule.ID)
	if err := usecase.NewCompleteTaskScheduleUseCase(uow, nil, id).ExecuteOccurrence(ctx, userID, taskID, scheduleID, "2026-10-19"); err != nil {
		t.Fatalf("complete virtual schedule: %v", err)
	}

	todoRows, err := NewTodoItemRepository(pool).ListByTask(ctx, userID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	assertSavedOccurrence(t, todoRows, "2026-10-12", false)
	assertSavedOccurrence(t, todoRows, "2026-10-19", true)
	scheduleRows, err := NewTaskScheduleRepository(pool).ListByTaskAndUserID(ctx, userID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	for _, occurrenceDate := range []string{"2026-10-12", "2026-10-19"} {
		found := false
		for _, row := range scheduleRows {
			if row.OccurrenceDate == occurrenceDate {
				assertRealOccurrenceID(t, row.ID)
				if row.Completed != (occurrenceDate == "2026-10-19") {
					t.Errorf("schedule %s completed = %v", occurrenceDate, row.Completed)
				}
				found = true
			}
		}
		if !found {
			t.Errorf("schedule %s was not saved", occurrenceDate)
		}
	}
}

func assertRealOccurrenceID(t *testing.T, value string) {
	t.Helper()
	if value == usecase.VirtualOccurrenceID {
		t.Fatalf("virtual response ID was persisted: %s", value)
	}
	if _, err := ulid.ParseStrict(value); err != nil {
		t.Fatalf("occurrence ID %q is not a ULID: %v", value, err)
	}
}

func assertSavedOccurrence(t *testing.T, rows []dao.TodoItem, date string, completed bool) {
	t.Helper()
	for _, row := range rows {
		if row.OccurrenceDate == date {
			assertRealOccurrenceID(t, row.ID)
			if row.Completed != completed {
				t.Errorf("todo %s completed = %v, want %v", date, row.Completed, completed)
			}
			return
		}
	}
	t.Errorf("todo %s was not saved", date)
}

func TestRootTemplateUpdatesPreserveFirstOccurrenceSnapshot(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	uow := occurrenceTestUOW{pool: pool}
	id := occurrenceTestID{}
	monday, err := domain.NewTaskFrequency("mon")
	if err != nil {
		t.Fatal(err)
	}
	first := recurrenceDate("2026-10-05")
	todoID := domain.TodoItemID(id.Generate())
	root, err := domain.NewTodoItemWithDetails(todoID, taskID, "Original", "", first, false, 0, 1, domain.TaskFrequencies{monday})
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(todoID), OccurrenceDate: first, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTodoItemRepository(pool).CreateForOwnedTask(ctx, userID, root, false); err != nil {
		t.Fatal(err)
	}
	firstTitle, laterTitle := "First only", "Later"
	update := usecase.NewUpdateTodoItemUseCase(uow, nil, id)
	if _, err := update.ExecuteOccurrence(ctx, userID, taskID, todoID, "2026-10-05", "current", usecase.PatchField[string]{Present: true, Value: &firstTitle}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{}); err != nil {
		t.Fatalf("edit first occurrence: %v", err)
	}
	if _, err := update.ExecuteOccurrence(ctx, userID, taskID, todoID, "2026-10-19", "future", usecase.PatchField[string]{Present: true, Value: &laterTitle}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{}); err != nil {
		t.Fatalf("revise later occurrences: %v", err)
	}
	todoPage, err := usecase.NewListTodoItemsUseCase(uow, nil).ExecutePage(ctx, userID, taskID, usecase.CursorPageRequest{Size: 4, FromDate: "2026-10-05"})
	if err != nil {
		t.Fatal(err)
	}
	for index, expected := range []string{"First only", "Later", "Later", "Later"} {
		if len(todoPage.Items) <= index || todoPage.Items[index].Title != expected {
			t.Fatalf("todo titles = %+v, index %d want %q", todoPage.Items, index, expected)
		}
	}
	storedRoot, err := NewTodoItemRepository(pool).GetForOwnedTask(ctx, userID, taskID, todoID)
	if err != nil || storedRoot.Title != "Later" || storedRoot.RepeatState != "active" || storedRoot.FrequencyAnchorDate != first.Unix() {
		t.Fatalf("current todo root = %+v, error = %v", storedRoot, err)
	}

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, location)
	scheduleID := domain.TaskScheduleID(id.Generate())
	schedule, err := domain.NewTaskScheduleWithDetails(scheduleID, taskID, "Weekly", "", "", 1, domain.TaskFrequencies{monday}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(scheduleID), OccurrenceDate: first, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTaskScheduleRepository(pool).CreateByTaskAndUserID(ctx, userID, schedule); err != nil {
		t.Fatal(err)
	}
	reschedule := usecase.NewRescheduleTaskScheduleUseCase(uow, nil, id)
	firstStart := time.Date(2026, 10, 5, 10, 0, 0, 0, location)
	if _, err := reschedule.Execute(ctx, usecase.RescheduleTaskScheduleInput{UserID: userID, TaskID: taskID, TaskScheduleID: scheduleID, OccurrenceDate: "2026-10-05", StartAt: firstStart, EndAt: firstStart.Add(time.Hour), Scope: "current"}); err != nil {
		t.Fatalf("reschedule first occurrence: %v", err)
	}
	laterStart := time.Date(2026, 10, 19, 11, 0, 0, 0, location)
	if _, err := reschedule.Execute(ctx, usecase.RescheduleTaskScheduleInput{UserID: userID, TaskID: taskID, TaskScheduleID: scheduleID, OccurrenceDate: "2026-10-19", StartAt: laterStart, EndAt: laterStart.Add(time.Hour), Scope: "future"}); err != nil {
		t.Fatalf("revise later schedule times: %v", err)
	}
	schedulePage, err := usecase.NewListTaskSchedulesUseCase(uow, nil).ExecutePage(ctx, userID, taskID, usecase.CursorPageRequest{Size: 4, FromDate: "2026-10-05"})
	if err != nil {
		t.Fatal(err)
	}
	for index, hour := range []int{10, 11, 11, 11} {
		if len(schedulePage.Items) <= index || time.Unix(schedulePage.Items[index].StartAt, 0).In(location).Hour() != hour {
			t.Fatalf("schedule starts = %+v, index %d want %d:00", schedulePage.Items, index, hour)
		}
	}
	wrongDate := time.Date(2026, 10, 27, 11, 0, 0, 0, location)
	_, err = reschedule.Execute(ctx, usecase.RescheduleTaskScheduleInput{UserID: userID, TaskID: taskID, TaskScheduleID: scheduleID, OccurrenceDate: "2026-10-26", StartAt: wrongDate, EndAt: wrongDate.Add(time.Hour), Scope: "future"})
	if !errors.Is(err, usecase.ErrRescheduleDateMismatch) {
		t.Fatalf("different local date error = %v, want ErrRescheduleDateMismatch", err)
	}
}

func TestOneOffScheduleRescheduleWithoutOccurrenceDate(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	uow := occurrenceTestUOW{pool: pool}
	id := occurrenceTestID{}
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 6, 9, 0, 0, 0, location)
	scheduleID := domain.TaskScheduleID(id.Generate())
	schedule, err := domain.NewTaskScheduleWithDetails(scheduleID, taskID, "One-off", "", "", 0, nil, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(scheduleID), OccurrenceDate: recurrenceDate("2026-10-06"), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTaskScheduleRepository(pool).CreateByTaskAndUserID(ctx, userID, schedule); err != nil {
		t.Fatal(err)
	}

	newStart := start.Add(2 * time.Hour)
	updated, err := usecase.NewRescheduleTaskScheduleUseCase(uow, nil, id).Execute(ctx, usecase.RescheduleTaskScheduleInput{
		UserID: userID, TaskID: taskID, TaskScheduleID: scheduleID,
		StartAt: newStart, EndAt: newStart.Add(time.Hour), Scope: "future",
	})
	if err != nil {
		t.Fatalf("reschedule one-off without occurrence date: %v", err)
	}
	if updated.ID != string(scheduleID) || updated.StartAt != newStart.Unix() {
		t.Fatalf("updated schedule = %+v", updated)
	}
	stored, err := NewTaskScheduleRepository(pool).GetByTaskAndUserID(ctx, userID, taskID, scheduleID)
	if err != nil || stored.RepeatState != "one_off" || stored.IntervalWeeks != 0 {
		t.Fatalf("one-off schedule root = %+v, error = %v", stored, err)
	}
}

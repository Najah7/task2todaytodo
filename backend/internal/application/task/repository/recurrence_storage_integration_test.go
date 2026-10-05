package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func TestInlineRecurrenceStorageAndTodoSkippedChild(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	todoRepo := NewTodoItemRepository(pool)
	todoID := domain.TodoItemID(ulid.Make().String())
	rootDate := recurrenceDate("2026-10-05")
	monday := domain.TaskFrequencies{{Value: "mon"}}
	todo, err := domain.NewTodoItemWithDetails(todoID, taskID, "Weekly review", "notes", rootDate, false, 1, 1, monday)
	if err != nil {
		t.Fatal(err)
	}
	todo, err = todo.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(todoID), OccurrenceDate: rootDate, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := todoRepo.CreateForOwnedTask(ctx, userID, todo, true); err != nil {
		t.Fatalf("create todo root: %v", err)
	}
	root, err := todoRepo.GetForOwnedTask(ctx, userID, taskID, todoID)
	if err != nil || root.RepeatState != "active" || root.FrequencyAnchorDate != rootDate.Unix() {
		t.Fatalf("initial todo root = %+v, error %v", root, err)
	}
	newAnchor := recurrenceDate("2026-10-19")
	if err := todoRepo.SetTodoItemRecurrence(ctx, userID, taskID, todoID, newAnchor, 2, []dao.TaskFrequency{{Value: "wed"}}); err != nil {
		t.Fatalf("replace todo recurrence: %v", err)
	}
	root, err = todoRepo.GetForOwnedTask(ctx, userID, taskID, todoID)
	if err != nil || root.FrequencyAnchorDate != newAnchor.Unix() || root.IntervalWeeks != 2 || len(root.Frequencies) != 1 || root.Frequencies[0].Value != "wed" {
		t.Fatalf("replaced todo root = %+v, error %v", root, err)
	}
	if err := todoRepo.SetTodoItemSkippedOccurrence(ctx, userID, taskID, todoID, rootDate, true); err != nil {
		t.Fatalf("skip todo root occurrence: %v", err)
	}
	var rootDeleted, rootDateChildDeleted, rootDateChildSkipped bool
	if err := pool.QueryRow(ctx, `SELECT root.deleted_at IS NOT NULL, child.deleted_at IS NOT NULL, child.skipped_at IS NOT NULL FROM todo_items root JOIN todo_items child ON child.series_id = root.id AND child.occurrence_date = root.occurrence_date AND child.id <> child.series_id WHERE root.id = $1 AND root.id = root.series_id`, string(todoID)).Scan(&rootDeleted, &rootDateChildDeleted, &rootDateChildSkipped); err != nil {
		t.Fatalf("read skipped first todo occurrence: %v", err)
	}
	if rootDeleted || rootDateChildDeleted || !rootDateChildSkipped {
		t.Fatalf("skipping first todo occurrence set root deleted=%v, child deleted=%v, child skipped=%v", rootDeleted, rootDateChildDeleted, rootDateChildSkipped)
	}
	if err := todoRepo.SetTodoItemSkippedOccurrence(ctx, userID, taskID, todoID, rootDate, false); err != nil {
		t.Fatalf("restore todo root occurrence: %v", err)
	}
	var firstTodoChildCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM todo_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(todoID), timeToPgDate(rootDate)).Scan(&firstTodoChildCount); err != nil {
		t.Fatal(err)
	}
	if firstTodoChildCount != 0 {
		t.Fatalf("restored first todo skip marker count = %d, want 0", firstTodoChildCount)
	}

	updated := todo
	updated.Title = "New weekly review"
	updated.Description = "future template"
	updated.DueDate = recurrenceDate("2026-10-19")
	firstSnapshotID := domain.TodoItemID(ulid.Make().String())
	if _, err := todoRepo.UpdateTodoItemSeriesTemplate(ctx, userID, taskID, todoID, firstSnapshotID, updated); err != nil {
		t.Fatalf("update todo root template: %v", err)
	}
	updated.Title = "Latest template"
	secondSnapshotID := domain.TodoItemID(ulid.Make().String())
	if _, err := todoRepo.UpdateTodoItemSeriesTemplate(ctx, userID, taskID, todoID, secondSnapshotID, updated); err != nil {
		t.Fatalf("update todo root template again: %v", err)
	}
	var snapshotID, snapshotTitle, rootTitle string
	if err := pool.QueryRow(ctx, `SELECT child.id, child.title, root.title FROM todo_items root JOIN todo_items child ON child.series_id = root.id AND child.occurrence_date = root.occurrence_date AND child.id <> child.series_id WHERE root.id = $1 AND root.id = root.series_id`, string(todoID)).Scan(&snapshotID, &snapshotTitle, &rootTitle); err != nil {
		t.Fatalf("read first todo snapshot: %v", err)
	}
	if snapshotID != string(firstSnapshotID) || snapshotTitle != "Weekly review" || rootTitle != "Latest template" {
		t.Fatalf("todo root/snapshot = %q/%q/%q; snapshot should stay original while root template changes", snapshotID, snapshotTitle, rootTitle)
	}

	skipDate := recurrenceDate("2026-10-26")
	if err := todoRepo.SetTodoItemSkippedOccurrence(ctx, userID, taskID, todoID, skipDate, true); err != nil {
		t.Fatalf("skip todo occurrence: %v", err)
	}
	assertTodoSkipped(t, ctx, pool, todoRepo, userID, taskID, todoID, skipDate, true)
	var skippedException bool
	if err := pool.QueryRow(ctx, `SELECT is_exception FROM todo_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(todoID), timeToPgDate(skipDate)).Scan(&skippedException); err != nil {
		t.Fatalf("read pure todo skip child: %v", err)
	}
	if skippedException {
		t.Fatal("pure todo skip marker is_exception = true, want false")
	}
	if err := todoRepo.SetTodoItemSkippedOccurrence(ctx, userID, taskID, todoID, skipDate, false); err != nil {
		t.Fatalf("restore todo occurrence: %v", err)
	}
	assertTodoSkipped(t, ctx, pool, todoRepo, userID, taskID, todoID, skipDate, false)
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM todo_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(todoID), timeToPgDate(skipDate)).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("restored pure todo marker count = %d, want 0", remaining)
	}

	editedDate := recurrenceDate("2026-11-02")
	editedID := domain.TodoItemID(ulid.Make().String())
	edited, err := domain.NewTodoItemWithDetails(editedID, taskID, "Saved edit", "keep me", editedDate, false, 3, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	edited, err = edited.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(todoID), OccurrenceDate: editedDate, Timezone: "Asia/Tokyo", IsException: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := todoRepo.UpsertTodoItemOverride(ctx, userID, edited); err != nil {
		t.Fatalf("save edited todo occurrence: %v", err)
	}
	if err := todoRepo.SetTodoItemSkippedOccurrence(ctx, userID, taskID, todoID, editedDate, true); err != nil {
		t.Fatalf("skip edited todo occurrence: %v", err)
	}
	if err := todoRepo.SetTodoItemSkippedOccurrence(ctx, userID, taskID, todoID, editedDate, false); err != nil {
		t.Fatalf("restore edited todo occurrence: %v", err)
	}
	var restoredTitle string
	var restoredDeleted, restoredSkipped bool
	if err := pool.QueryRow(ctx, `SELECT title, deleted_at IS NOT NULL, skipped_at IS NOT NULL FROM todo_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(todoID), timeToPgDate(editedDate)).Scan(&restoredTitle, &restoredDeleted, &restoredSkipped); err != nil {
		t.Fatalf("read restored edited todo: %v", err)
	}
	if restoredTitle != "Saved edit" || restoredDeleted || restoredSkipped {
		t.Fatalf("restored edited todo = %q deleted=%v skipped=%v, want saved content visible", restoredTitle, restoredDeleted, restoredSkipped)
	}
}

func TestInlineScheduleRecurrenceAndSkippedChild(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	scheduleRepo := NewTaskScheduleRepository(pool)
	scheduleID := domain.TaskScheduleID(ulid.Make().String())
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	rootDate := recurrenceDate("2026-10-05")
	startAt := time.Date(2026, 10, 5, 9, 30, 0, 0, location)
	schedule, err := domain.NewTaskScheduleWithDetails(scheduleID, taskID, "Weekly meeting", "agenda", "Room A", 1, domain.TaskFrequencies{{Value: "mon"}}, startAt, startAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(scheduleID), OccurrenceDate: rootDate, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scheduleRepo.CreateByTaskAndUserID(ctx, userID, schedule); err != nil {
		t.Fatalf("create schedule root: %v", err)
	}
	newAnchor := recurrenceDate("2026-10-19")
	if err := scheduleRepo.SetTaskScheduleRecurrence(ctx, userID, taskID, scheduleID, newAnchor, 2, []dao.TaskFrequency{{Value: "wed"}}); err != nil {
		t.Fatalf("replace schedule recurrence: %v", err)
	}
	root, err := scheduleRepo.GetByTaskAndUserID(ctx, userID, taskID, scheduleID)
	if err != nil || root.RepeatState != "active" || root.FrequencyAnchorDate != newAnchor.Unix() || root.IntervalWeeks != 2 || len(root.Frequencies) != 1 {
		t.Fatalf("updated schedule root = %+v, error %v", root, err)
	}
	if err := scheduleRepo.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, scheduleID, rootDate, true); err != nil {
		t.Fatalf("skip schedule root occurrence: %v", err)
	}
	var scheduleRootDeleted, scheduleRootDateChildDeleted, scheduleRootDateChildSkipped bool
	if err := pool.QueryRow(ctx, `SELECT root.deleted_at IS NOT NULL, child.deleted_at IS NOT NULL, child.skipped_at IS NOT NULL FROM task_schedules root JOIN task_schedules child ON child.series_id = root.id AND child.occurrence_date = root.occurrence_date AND child.id <> child.series_id WHERE root.id = $1 AND root.id = root.series_id`, string(scheduleID)).Scan(&scheduleRootDeleted, &scheduleRootDateChildDeleted, &scheduleRootDateChildSkipped); err != nil {
		t.Fatalf("read skipped first schedule occurrence: %v", err)
	}
	if scheduleRootDeleted || scheduleRootDateChildDeleted || !scheduleRootDateChildSkipped {
		t.Fatalf("skipping first schedule occurrence set root deleted=%v, child deleted=%v, child skipped=%v", scheduleRootDeleted, scheduleRootDateChildDeleted, scheduleRootDateChildSkipped)
	}
	if err := scheduleRepo.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, scheduleID, rootDate, false); err != nil {
		t.Fatalf("restore schedule root occurrence: %v", err)
	}
	var firstScheduleChildCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM task_schedules WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(scheduleID), timeToPgDate(rootDate)).Scan(&firstScheduleChildCount); err != nil {
		t.Fatal(err)
	}
	if firstScheduleChildCount != 0 {
		t.Fatalf("restored first schedule marker count = %d, want 0", firstScheduleChildCount)
	}

	updated := schedule
	updated.Title = "New meeting template"
	updated.Location = "Room B"
	updated.StartAt = time.Date(2026, 10, 5, 10, 0, 0, 0, location)
	updated.EndAt = updated.StartAt.Add(time.Hour)
	snapshotID := domain.TaskScheduleID(ulid.Make().String())
	if _, err := scheduleRepo.UpdateTaskScheduleSeriesTemplate(ctx, userID, taskID, scheduleID, snapshotID, updated); err != nil {
		t.Fatalf("update schedule template: %v", err)
	}
	var snapshotTitle, rootTitle string
	var snapshotStart, rootStart time.Time
	if err := pool.QueryRow(ctx, `SELECT child.title, root.title, child.start_at, root.start_at FROM task_schedules root JOIN task_schedules child ON child.series_id = root.id AND child.occurrence_date = root.occurrence_date AND child.id <> child.series_id WHERE root.id = $1 AND root.id = root.series_id`, string(scheduleID)).Scan(&snapshotTitle, &rootTitle, &snapshotStart, &rootStart); err != nil {
		t.Fatalf("read schedule root and snapshot: %v", err)
	}
	if snapshotTitle != "Weekly meeting" || rootTitle != "New meeting template" || !snapshotStart.Equal(startAt.UTC()) || !rootStart.Equal(updated.StartAt.UTC()) {
		t.Fatalf("schedule snapshot/root = %q/%q %v/%v", snapshotTitle, rootTitle, snapshotStart, rootStart)
	}

	skipDate := recurrenceDate("2026-10-26")
	if err := scheduleRepo.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, scheduleID, skipDate, true); err != nil {
		t.Fatalf("skip schedule occurrence: %v", err)
	}
	dates, err := scheduleRepo.ListTaskScheduleSkippedOccurrences(ctx, userID, taskID, scheduleID)
	if err != nil || len(dates) != 1 || dates[0] != skipDate.Unix() {
		t.Fatalf("schedule skipped dates = %#v, error %v", dates, err)
	}
	if err := scheduleRepo.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, scheduleID, skipDate, false); err != nil {
		t.Fatalf("restore schedule occurrence: %v", err)
	}
	dates, err = scheduleRepo.ListTaskScheduleSkippedOccurrences(ctx, userID, taskID, scheduleID)
	if err != nil || len(dates) != 0 {
		t.Fatalf("restored schedule skipped dates = %#v, error %v", dates, err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM task_schedules WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(scheduleID), timeToPgDate(skipDate)).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("restored schedule marker count = %d, want 0", remaining)
	}
	editedDate := recurrenceDate("2026-11-02")
	editedScheduleID := domain.TaskScheduleID(ulid.Make().String())
	editedStart := time.Date(2026, 11, 2, 12, 0, 0, 0, location)
	editedSchedule, err := domain.NewTaskScheduleWithDetails(editedScheduleID, taskID, "Saved schedule edit", "keep this", "Room C", 0, nil, editedStart, editedStart.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	editedSchedule, err = editedSchedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(scheduleID), OccurrenceDate: editedDate, Timezone: "Asia/Tokyo", IsException: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scheduleRepo.UpsertTaskScheduleOverride(ctx, userID, editedSchedule); err != nil {
		t.Fatalf("save edited schedule occurrence: %v", err)
	}
	if err := scheduleRepo.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, scheduleID, editedDate, true); err != nil {
		t.Fatalf("skip edited schedule occurrence: %v", err)
	}
	if err := scheduleRepo.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, scheduleID, editedDate, false); err != nil {
		t.Fatalf("restore edited schedule occurrence: %v", err)
	}
	var restoredScheduleTitle string
	var restoredScheduleDeleted, restoredScheduleSkipped bool
	if err := pool.QueryRow(ctx, `SELECT title, deleted_at IS NOT NULL, skipped_at IS NOT NULL FROM task_schedules WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(scheduleID), timeToPgDate(editedDate)).Scan(&restoredScheduleTitle, &restoredScheduleDeleted, &restoredScheduleSkipped); err != nil {
		t.Fatalf("read restored edited schedule: %v", err)
	}
	if restoredScheduleTitle != "Saved schedule edit" || restoredScheduleDeleted || restoredScheduleSkipped {
		t.Fatalf("restored edited schedule = %q deleted=%v skipped=%v, want saved content visible", restoredScheduleTitle, restoredScheduleDeleted, restoredScheduleSkipped)
	}

	if err := scheduleRepo.StopTaskScheduleRecurrence(ctx, userID, taskID, scheduleID); err != nil {
		t.Fatalf("stop schedule recurrence: %v", err)
	}
	root, err = scheduleRepo.GetByTaskAndUserID(ctx, userID, taskID, scheduleID)
	if err != nil || root.RepeatState != "stopped" || root.IntervalWeeks != 0 || root.FrequencyAnchorDate != newAnchor.Unix() || len(root.Frequencies) != 0 {
		t.Fatalf("stopped schedule root = %+v, error %v", root, err)
	}
}

func TestRestoreCannotResurrectDeletedOccurrenceOrTask(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := localDate(time.Now(), location)
	rootDate := today.AddDate(0, 0, -7)
	futureDate := today.AddDate(0, 0, 7)
	todoID := domain.TodoItemID(ulid.Make().String())
	createWeeklyTodoRoot(t, ctx, pool, userID, taskID, todoID, rootDate, location)
	todoRepo := NewTodoItemRepository(pool)
	if err := todoRepo.SetTodoItemSkippedOccurrence(ctx, userID, taskID, todoID, futureDate, true); err != nil {
		t.Fatalf("skip todo occurrence before deletion: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE todo_items SET deleted_at = now() WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(todoID), timeToPgDate(futureDate)); err != nil {
		t.Fatalf("delete skipped todo occurrence: %v", err)
	}
	if err := todoRepo.SetTodoItemSkippedOccurrence(ctx, userID, taskID, todoID, futureDate, false); err != nil {
		t.Fatalf("restore deleted todo occurrence: %v", err)
	}
	var todoDeleted, todoSkipped bool
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL, skipped_at IS NOT NULL FROM todo_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(todoID), timeToPgDate(futureDate)).Scan(&todoDeleted, &todoSkipped); err != nil {
		t.Fatalf("read deleted todo occurrence after restore: %v", err)
	}
	if !todoDeleted || !todoSkipped {
		t.Fatalf("deleted todo occurrence after restore: deleted=%v skipped=%v, want both markers retained", todoDeleted, todoSkipped)
	}
	if dates, err := todoRepo.ListTodoItemSkippedOccurrences(ctx, userID, taskID, todoID); err != nil || len(dates) != 0 {
		t.Fatalf("list skipped todo dates after deletion: dates=%v error=%v, want no active skips", dates, err)
	}

	frequencyName := strings.ToLower(rootDate.Weekday().String()[:3])
	frequency, err := domain.NewTaskFrequency(frequencyName)
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := domain.TaskScheduleID(ulid.Make().String())
	startAt := time.Date(rootDate.Year(), rootDate.Month(), rootDate.Day(), 9, 0, 0, 0, location)
	schedule, err := domain.NewTaskScheduleWithDetails(scheduleID, taskID, "Weekly schedule", "", "", 1, domain.TaskFrequencies{frequency}, startAt, startAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(scheduleID), OccurrenceDate: rootDate, Timezone: location.String()})
	if err != nil {
		t.Fatal(err)
	}
	scheduleRepo := NewTaskScheduleRepository(pool)
	if _, err := scheduleRepo.CreateByTaskAndUserID(ctx, userID, schedule); err != nil {
		t.Fatalf("create schedule root: %v", err)
	}
	if err := scheduleRepo.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, scheduleID, futureDate, true); err != nil {
		t.Fatalf("skip schedule occurrence before deletion: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE task_schedules SET deleted_at = now() WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(scheduleID), timeToPgDate(futureDate)); err != nil {
		t.Fatalf("delete skipped schedule occurrence: %v", err)
	}
	if err := scheduleRepo.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, scheduleID, futureDate, false); err != nil {
		t.Fatalf("restore deleted schedule occurrence: %v", err)
	}
	var scheduleDeleted, scheduleSkipped bool
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL, skipped_at IS NOT NULL FROM task_schedules WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(scheduleID), timeToPgDate(futureDate)).Scan(&scheduleDeleted, &scheduleSkipped); err != nil {
		t.Fatalf("read deleted schedule occurrence after restore: %v", err)
	}
	if !scheduleDeleted || !scheduleSkipped {
		t.Fatalf("deleted schedule occurrence after restore: deleted=%v skipped=%v, want both markers retained", scheduleDeleted, scheduleSkipped)
	}
	if dates, err := scheduleRepo.ListTaskScheduleSkippedOccurrences(ctx, userID, taskID, scheduleID); err != nil || len(dates) != 0 {
		t.Fatalf("list skipped schedule dates after deletion: dates=%v error=%v, want no active skips", dates, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE tasks SET deleted_at = now() WHERE id = $1`, string(taskID)); err != nil {
		t.Fatalf("delete task ancestor: %v", err)
	}
	nextDate := futureDate.AddDate(0, 0, 7)
	if err := todoRepo.SetTodoItemSkippedOccurrence(ctx, userID, taskID, todoID, nextDate, true); err == nil {
		t.Fatal("skip todo occurrence on deleted task succeeded")
	}
	if err := scheduleRepo.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, scheduleID, nextDate, true); err == nil {
		t.Fatal("skip schedule occurrence on deleted task succeeded")
	}
}

func assertTodoSkipped(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *TodoItemRepository, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, date time.Time, want bool) {
	t.Helper()
	dates, err := repo.ListTodoItemSkippedOccurrences(ctx, userID, taskID, seriesID)
	if err != nil {
		t.Fatalf("list skipped todo dates: %v", err)
	}
	found := false
	for _, value := range dates {
		if value == date.Unix() {
			found = true
			break
		}
	}
	if found != want {
		t.Fatalf("skipped todo dates = %#v, contains %s = %v, want %v", dates, date.Format("2006-01-02"), found, want)
	}
	var rowCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM todo_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id AND deleted_at IS NULL AND skipped_at IS NOT NULL`, string(seriesID), timeToPgDate(date)).Scan(&rowCount); err != nil {
		t.Fatalf("count todo skip marker: %v", err)
	}
	if (rowCount > 0) != want {
		t.Fatalf("todo active skipped child rows = %d, want skipped=%v", rowCount, want)
	}
}

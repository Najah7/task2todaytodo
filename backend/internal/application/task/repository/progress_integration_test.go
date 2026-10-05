package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTaskProgressCountsSavedOccurrencesAndCurrentVirtualOccurrenceOnce(t *testing.T) {
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
	rootID := domain.TodoItemID(occurrenceTestID{}.Generate())
	createWeeklyTodoRoot(t, ctx, pool, userID, taskID, rootID, rootDate, location)

	// Persist a future occurrence of the same series. The root, this saved
	// occurrence, and today's still-virtual occurrence are distinct work.
	if err := usecase.NewCompleteTodoItemUseCase(occurrenceTestUOW{pool: pool}, nil, occurrenceTestID{}).ExecuteOccurrence(
		ctx, userID, taskID, rootID, futureDate.Format("2006-01-02"),
	); err != nil {
		t.Fatalf("save completed future occurrence: %v", err)
	}

	taskRepo := NewTaskRepository(pool)
	current, err := taskRepo.ReadTaskProgressSources(ctx, []string{string(taskID)}, nil, time.Now())
	if err != nil {
		t.Fatalf("read current progress sources: %v", err)
	}
	if got := current.Counts[string(taskID)]; got != (dao.TaskProgressCounts{Total: 2, Completed: 1}) {
		t.Errorf("saved progress counts = %+v, want root + saved future, with one completed", got)
	}
	if len(current.TodoItemRoots) != 1 || current.TodoItemRoots[0].OccurrenceSavedToday {
		t.Fatalf("recurrence roots = %+v, want one active root with today's occurrence still virtual", current.TodoItemRoots)
	}
	page, err := usecase.NewListTasksUseCase(taskRepo, nil).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil {
		t.Fatalf("project current progress: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Progress != 33 {
		t.Errorf("current task page = %+v, want three total occurrences and 33%% progress", page.Items)
	}
	if err := usecase.NewCompleteTodoItemUseCase(occurrenceTestUOW{pool: pool}, nil, occurrenceTestID{}).ExecuteOccurrence(
		ctx, userID, taskID, rootID, today.Format("2006-01-02"),
	); err != nil {
		t.Fatalf("materialize and complete today's virtual occurrence: %v", err)
	}
	materialized, err := taskRepo.ReadTaskProgressSources(ctx, []string{string(taskID)}, nil, time.Now())
	if err != nil {
		t.Fatalf("read materialized occurrence progress: %v", err)
	}
	if got := materialized.Counts[string(taskID)]; got != (dao.TaskProgressCounts{Total: 3, Completed: 2}) {
		t.Errorf("materialized progress counts = %+v, want three saved occurrences counted once", got)
	}
	if len(materialized.TodoItemRoots) != 1 || !materialized.TodoItemRoots[0].OccurrenceSavedToday {
		t.Errorf("materialized recurrence roots = %+v, want saved-today marker suppressing virtual duplicate", materialized.TodoItemRoots)
	}
	page, err = usecase.NewListTasksUseCase(taskRepo, nil).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].Progress != 66 {
		t.Errorf("materialized task page = %+v, error=%v, want progress 66%% after one deduplicated occurrence", page.Items, err)
	}

	rolledOver, err := taskRepo.ReadTaskProgressSources(ctx, []string{string(taskID)}, nil, today.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("read next-day progress sources: %v", err)
	}
	if got := rolledOver.Counts[string(taskID)]; got != (dao.TaskProgressCounts{Total: 3, Completed: 2}) {
		t.Errorf("next-day progress counts = %+v, want all three saved occurrences", got)
	}
	status, err := taskRepo.GetByUserID(ctx, userID, taskID)
	if err != nil {
		t.Fatalf("read task after progress reads: %v", err)
	}
	if status.Status.Value != "open" {
		t.Errorf("task status after read and date rollover = %q, want open", status.Status.Value)
	}
}

func TestTaskScheduleProgressUsesScheduleTimezoneAndDate(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	now := time.Now()
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	if sameLocalDate(now, tokyo, location) {
		location, err = time.LoadLocation("Pacific/Kiritimati")
		if err != nil {
			t.Fatal(err)
		}
	}
	if sameLocalDate(now, tokyo, location) {
		t.Fatalf("could not select a schedule timezone with a different local date from the task timezone")
	}
	today := localDate(now, location)
	rootDate := today.AddDate(0, 0, -7)
	frequency, err := domain.NewTaskFrequency(strings.ToLower(today.Weekday().String()[:3]))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(rootDate.Year(), rootDate.Month(), rootDate.Day(), 9, 0, 0, 0, location)
	id := domain.TaskScheduleID(occurrenceTestID{}.Generate())
	schedule, err := domain.NewTaskScheduleWithDetails(id, taskID, "Weekly schedule", "", "", 1, domain.TaskFrequencies{frequency}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(id), OccurrenceDate: rootDate, Timezone: location.String()})
	if err != nil {
		t.Fatal(err)
	}
	schedules := NewTaskScheduleRepository(pool)
	if _, err := schedules.CreateByTaskAndUserID(ctx, userID, schedule); err != nil {
		t.Fatalf("create weekly schedule root: %v", err)
	}
	if err := schedules.SetCompletedForOwnedTask(ctx, userID, taskID, id, true); err != nil {
		t.Fatalf("complete saved schedule occurrence: %v", err)
	}

	sources, err := NewTaskRepository(pool).ReadTaskProgressSources(ctx, []string{string(taskID)}, nil, now)
	if err != nil {
		t.Fatalf("read schedule progress sources: %v", err)
	}
	if len(sources.TaskScheduleRoots) != 1 || sources.TaskScheduleRoots[0].Timezone != location.String() || sources.TaskScheduleRoots[0].OccurrenceSavedToday {
		t.Fatalf("schedule roots = %+v, want one unsaved-today root in %s", sources.TaskScheduleRoots, location)
	}
	page, err := usecase.NewListTasksUseCase(NewTaskRepository(pool), nil).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil {
		t.Fatalf("project schedule progress: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Progress != 50 {
		t.Fatalf("task page = %+v, want saved completed schedule plus current virtual schedule at 50%%", page.Items)
	}
}

func TestProjectProgressAveragesEveryLinkedTaskAfterPerTaskFloor(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, firstTaskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	projectID := domain.ProjectID(occurrenceTestID{}.Generate())
	projectType, err := domain.NewProjectType("other")
	if err != nil {
		t.Fatal(err)
	}
	priority, err := domain.NewTaskPriority("low")
	if err != nil {
		t.Fatal(err)
	}
	project, err := domain.NewProject(projectID, userID, projectType, priority, "Progress project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewProjectRepository(pool).Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, string(projectID))
	})
	if _, err := pool.Exec(ctx, `UPDATE tasks SET project_id = $2 WHERE id = $1`, string(firstTaskID), string(projectID)); err != nil {
		t.Fatalf("link first task to project: %v", err)
	}
	otherIDs := []domain.TaskID{domain.TaskID(occurrenceTestID{}.Generate()), domain.TaskID(occurrenceTestID{}.Generate())}
	for index, id := range otherIDs {
		if _, err := pool.Exec(ctx, `INSERT INTO tasks (id, user_id, project_id, title) VALUES ($1, $2, $3, $4)`, string(id), string(userID), string(projectID), "Project task"); err != nil {
			t.Fatalf("create project task %d: %v", index, err)
		}
	}
	taskIDs := []domain.TaskID{firstTaskID, otherIDs[0], otherIDs[1]}
	completedCounts := []int{1, 2, 0}
	totalCounts := []int{3, 3, 1}
	date := localDate(time.Now(), time.UTC).Format("2006-01-02")
	for taskIndex, id := range taskIDs {
		for occurrenceIndex := 0; occurrenceIndex < totalCounts[taskIndex]; occurrenceIndex++ {
			itemID := occurrenceTestID{}.Generate()
			completed := occurrenceIndex < completedCounts[taskIndex]
			if _, err := pool.Exec(ctx, `
				INSERT INTO todo_items (id, task_id, title, completed, position, series_id, occurrence_date, timezone)
				VALUES ($1, $2, 'Project progress item', $3, $4, $1, $5::date, 'UTC')
			`, itemID, string(id), completed, occurrenceIndex, date); err != nil {
				t.Fatalf("create task %s progress item %d: %v", id, occurrenceIndex, err)
			}
		}
	}

	projectRepo := NewProjectRepository(pool)
	got, err := usecase.NewGetProjectUseCase(projectRepo, NewTaskRepository(pool), nil).Execute(ctx, userID, projectID)
	if err != nil {
		t.Fatalf("get project progress: %v", err)
	}
	if got.Progress != 33 { // floor(100/3) + floor(200/3) + 0, then average.
		t.Errorf("project progress = %d, want 33 from all three task percentages (33, 66, 0)", got.Progress)
	}
	page, err := usecase.NewListProjectTasksUseCase(projectRepo, NewTaskRepository(pool), nil).Execute(ctx, userID, projectID, usecase.CursorPageRequest{Size: 1})
	if err != nil {
		t.Fatalf("list partial project task page: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("project task page has %d items, want one while project progress still includes all three", len(page.Items))
	}
}

func TestExplicitCompleteAndReopenProjectProgressProjection(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := localDate(time.Now(), location)
	repo := NewTodoItemRepository(pool)
	itemIDs := make([]domain.TodoItemID, 0, 2)
	for index := 0; index < 2; index++ {
		id := domain.TodoItemID(occurrenceTestID{}.Generate())
		itemIDs = append(itemIDs, id)
		item, err := domain.NewTodoItemWithDetails(id, taskID, "Explicit status item", "", today, false, index, domain.OnceIntervalWeeks, nil)
		if err != nil {
			t.Fatal(err)
		}
		item, err = item.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(id), OccurrenceDate: today, Timezone: location.String()})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateForOwnedTask(ctx, userID, item, false); err != nil {
			t.Fatalf("create explicit status item %d: %v", index, err)
		}
	}
	tasks := NewTaskRepository(pool)
	currentTask, err := tasks.GetByUserID(ctx, userID, taskID)
	if err != nil {
		t.Fatalf("load task before explicit completion: %v", err)
	}
	if _, err := usecase.NewCompleteTaskUseCase(occurrenceTestUOW{pool: pool}, nil, nil).Execute(ctx, userID, taskID, currentTask.Revision); err != nil {
		t.Fatalf("explicitly complete task: %v", err)
	}
	page, err := usecase.NewListTasksUseCase(tasks, nil).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].Status.Value != "done" || page.Items[0].Progress != 100 {
		t.Fatalf("explicitly completed task page = %+v, error=%v, want done at 100%%", page.Items, err)
	}
	currentTask, err = tasks.GetByUserID(ctx, userID, taskID)
	if err != nil {
		t.Fatalf("load task before explicit reopen: %v", err)
	}
	if _, err := usecase.NewReopenTaskUseCase(occurrenceTestUOW{pool: pool}, nil).Execute(ctx, userID, taskID, currentTask.Revision); err != nil {
		t.Fatalf("explicitly reopen task: %v", err)
	}
	page, err = usecase.NewListTasksUseCase(tasks, nil).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].Status.Value != "open" || page.Items[0].Progress != 0 {
		t.Fatalf("explicitly reopened task page = %+v, error=%v, want open at 0%% while children remain unfinished", page.Items, err)
	}
	uow := occurrenceTestUOW{pool: pool}
	for _, id := range itemIDs {
		if err := usecase.NewCompleteTodoItemUseCase(uow, nil).Execute(ctx, userID, taskID, id); err != nil {
			t.Fatalf("complete child %s: %v", id, err)
		}
	}
	assertProgressTaskStatus(t, ctx, pool, userID, taskID, "done")
	currentTask, err = tasks.GetByUserID(ctx, userID, taskID)
	if err != nil {
		t.Fatalf("load task before final reopen: %v", err)
	}
	if _, err := usecase.NewReopenTaskUseCase(uow, nil).Execute(ctx, userID, taskID, currentTask.Revision); err != nil {
		t.Fatalf("explicitly reopen fully completed task: %v", err)
	}
	page, err = usecase.NewListTasksUseCase(tasks, nil).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].Status.Value != "open" || page.Items[0].Progress != 100 {
		t.Fatalf("fully completed reopened task page = %+v, error=%v, want open at 100%%", page.Items, err)
	}
}

func TestStatusRecomputationFailureRollsBackChildMutation(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := localDate(time.Now(), location)
	rootID := domain.TodoItemID(occurrenceTestID{}.Generate())
	createWeeklyTodoRoot(t, ctx, pool, userID, taskID, rootID, today.AddDate(0, 0, -7), location)
	wantErr := errors.New("progress query failed")
	uow := progressFailureUOW{base: occurrenceTestUOW{pool: pool}, failAt: 2, err: wantErr}
	err = usecase.NewSkipTodoItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, today.Format("2006-01-02"))
	if !errors.Is(err, wantErr) {
		t.Fatalf("skip error = %v, want progress query failure", err)
	}
	skipped, err := NewTodoItemRepository(pool).ListTodoItemSkippedOccurrences(ctx, userID, taskID, rootID)
	if err != nil {
		t.Fatalf("read skipped occurrences after rollback: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped dates after rollback = %v, want none", skipped)
	}
	assertProgressTaskStatus(t, ctx, pool, userID, taskID, "open")
}

func TestSkipAutoCompletesAndRestoreReopensWithVirtualOccurrence(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := localDate(time.Now(), location)
	rootDate := today.AddDate(0, 0, -7)
	rootID := domain.TodoItemID(occurrenceTestID{}.Generate())
	createWeeklyTodoRoot(t, ctx, pool, userID, taskID, rootID, rootDate, location)
	uow := occurrenceTestUOW{pool: pool}

	if err := usecase.NewCompleteTodoItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID); err != nil {
		t.Fatalf("complete saved root occurrence: %v", err)
	}
	if err := usecase.NewSkipTodoItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, today.Format("2006-01-02")); err != nil {
		t.Fatalf("skip today's virtual occurrence: %v", err)
	}
	assertProgressTaskStatus(t, ctx, pool, userID, taskID, "done")

	if err := usecase.NewRestoreTodoItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, today.Format("2006-01-02")); err != nil {
		t.Fatalf("restore today's virtual occurrence: %v", err)
	}
	assertProgressTaskStatus(t, ctx, pool, userID, taskID, "open")

	page, err := usecase.NewListTasksUseCase(NewTaskRepository(pool), nil).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil {
		t.Fatalf("read task progress after restore: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Progress != 50 {
		t.Fatalf("restored task page = %+v, want one reopened task at 50%%", page.Items)
	}
}

func TestRestoreFutureSkipLeavesListAndProgressUnchanged(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	asOf := time.Now()
	today := localDate(asOf, location)
	rootID := domain.TodoItemID(occurrenceTestID{}.Generate())
	createWeeklyTodoRoot(t, ctx, pool, userID, taskID, rootID, today.AddDate(0, 0, -7), location)
	futureDate := today.AddDate(0, 0, 7)
	uow := occurrenceTestUOW{pool: pool}
	list := func() []dao.TodoItem {
		t.Helper()
		page, err := usecase.NewListTodoItemsUseCase(uow, nil, func() time.Time { return asOf }).ExecutePage(ctx, userID, taskID, usecase.CursorPageRequest{
			Size: 10, FromDate: futureDate.Format("2006-01-02"), AsOf: asOf,
		})
		if err != nil {
			t.Fatalf("list future todo occurrences: %v", err)
		}
		return page.Items
	}
	progress := func() (int, string) {
		t.Helper()
		page, err := usecase.NewListTasksUseCase(NewTaskRepository(pool), nil).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
		if err != nil {
			t.Fatalf("read task progress: %v", err)
		}
		if len(page.Items) != 1 {
			t.Fatalf("task page has %d items, want one", len(page.Items))
		}
		return page.Items[0].Progress, page.Items[0].Status.Value
	}

	beforeList := list()
	containsFutureDate := func(rows []dao.TodoItem) bool {
		for _, row := range rows {
			if row.OccurrenceDate == futureDate.Format("2006-01-02") {
				return true
			}
		}
		return false
	}
	if len(beforeList) == 0 || !containsFutureDate(beforeList) {
		t.Fatalf("future list before skip = %+v, want occurrence on %s among page results", beforeList, futureDate.Format("2006-01-02"))
	}
	beforeProgress, beforeStatus := progress()
	beforeSources, err := NewTaskRepository(pool).ReadTaskProgressSources(ctx, []string{string(taskID)}, nil, asOf)
	if err != nil {
		t.Fatalf("read progress sources before skip: %v", err)
	}

	if err := usecase.NewSkipTodoItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, futureDate.Format("2006-01-02")); err != nil {
		t.Fatalf("skip future occurrence: %v", err)
	}
	if skipped := list(); containsFutureDate(skipped) {
		t.Fatalf("future list while skipped still includes %s: %+v", futureDate.Format("2006-01-02"), skipped)
	}
	if gotProgress, gotStatus := progress(); gotProgress != beforeProgress || gotStatus != beforeStatus {
		t.Fatalf("progress while skipping future date = %d/%s, before was %d/%s", gotProgress, gotStatus, beforeProgress, beforeStatus)
	}

	if err := usecase.NewRestoreTodoItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, futureDate.Format("2006-01-02")); err != nil {
		t.Fatalf("restore future occurrence: %v", err)
	}
	afterList := list()
	if !reflect.DeepEqual(afterList, beforeList) {
		t.Fatalf("future list after restore = %+v, before skip was %+v", afterList, beforeList)
	}
	if gotProgress, gotStatus := progress(); gotProgress != beforeProgress || gotStatus != beforeStatus {
		t.Fatalf("progress after restore = %d/%s, before was %d/%s", gotProgress, gotStatus, beforeProgress, beforeStatus)
	}
	afterSources, err := NewTaskRepository(pool).ReadTaskProgressSources(ctx, []string{string(taskID)}, nil, asOf)
	if err != nil {
		t.Fatalf("read progress sources after restore: %v", err)
	}
	if beforeSources.Counts[string(taskID)] != afterSources.Counts[string(taskID)] || !reflect.DeepEqual(beforeSources.TodoItemRoots, afterSources.TodoItemRoots) {
		t.Fatalf("progress sources changed after future skip restore: before=%+v after=%+v", beforeSources, afterSources)
	}
}

func TestConcurrentFinalTodoCompletionsSetTaskDone(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := localDate(time.Now(), location)
	repo := NewTodoItemRepository(pool)
	ids := []domain.TodoItemID{domain.TodoItemID(occurrenceTestID{}.Generate()), domain.TodoItemID(occurrenceTestID{}.Generate())}
	for index, id := range ids {
		item, err := domain.NewTodoItemWithDetails(id, taskID, "Concurrent item", "", today, false, index, domain.OnceIntervalWeeks, nil)
		if err != nil {
			t.Fatal(err)
		}
		item, err = item.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(id), OccurrenceDate: today, Timezone: "Asia/Tokyo"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateForOwnedTask(ctx, userID, item, false); err != nil {
			t.Fatalf("create one-off item %d: %v", index, err)
		}
	}

	uow := occurrenceTestUOW{pool: pool}
	start := make(chan struct{})
	errCh := make(chan error, len(ids))
	var wg sync.WaitGroup
	for _, id := range ids {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errCh <- usecase.NewCompleteTodoItemUseCase(uow, nil).Execute(ctx, userID, taskID, id)
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Errorf("complete concurrent todo item: %v", err)
		}
	}
	assertProgressTaskStatus(t, ctx, pool, userID, taskID, "done")
	page, err := usecase.NewListTasksUseCase(NewTaskRepository(pool), nil).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil {
		t.Fatalf("read completed task progress: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Progress != 100 {
		t.Fatalf("completed task page = %+v, want one task at 100%%", page.Items)
	}
}

func createWeeklyTodoRoot(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID, rootDate time.Time, location *time.Location) {
	t.Helper()
	frequency := strings.ToLower(rootDate.Weekday().String()[:3])
	weekday, err := domain.NewTaskFrequency(frequency)
	if err != nil {
		t.Fatal(err)
	}
	item, err := domain.NewTodoItemWithDetails(id, taskID, "Weekly progress item", "", rootDate, false, 0, 1, domain.TaskFrequencies{weekday})
	if err != nil {
		t.Fatal(err)
	}
	item, err = item.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(id), OccurrenceDate: rootDate, Timezone: location.String()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTodoItemRepository(pool).CreateForOwnedTask(ctx, userID, item, false); err != nil {
		t.Fatalf("create weekly todo root: %v", err)
	}
}

func localDate(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func sameLocalDate(value time.Time, first, second *time.Location) bool {
	return localDate(value, first).Equal(localDate(value, second))
}

func assertProgressTaskStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID domain.UserID, taskID domain.TaskID, want string) {
	t.Helper()
	task, err := NewTaskRepository(pool).GetByUserID(ctx, userID, taskID)
	if err != nil {
		t.Fatalf("read task status: %v", err)
	}
	if task.Status.Value != want {
		t.Fatalf("task status = %q, want %q", task.Status.Value, want)
	}
}

type progressFailureUOW struct {
	base   occurrenceTestUOW
	failAt int
	err    error
}

func (uow progressFailureUOW) Do(ctx context.Context, fn func(context.Context, usecase.Repositories) error) error {
	return uow.base.Do(ctx, func(ctx context.Context, repos usecase.Repositories) error {
		tasks := &progressFailureTaskRepository{TaskRepository: repos.Tasks(), failAt: uow.failAt, err: uow.err}
		return fn(ctx, occurrenceTestRepositories{task: tasks, todo: repos.TodoItems(), schedule: repos.TaskSchedules()})
	})
}

type progressFailureTaskRepository struct {
	usecase.TaskRepository
	readCount int
	failAt    int
	err       error
}

func (repo *progressFailureTaskRepository) ReadTaskProgressSources(ctx context.Context, taskIDs, projectIDs []string, asOf time.Time) (dao.TaskProgressSources, error) {
	repo.readCount++
	if repo.readCount == repo.failAt {
		return dao.TaskProgressSources{}, repo.err
	}
	return repo.TaskRepository.ReadTaskProgressSources(ctx, taskIDs, projectIDs, asOf)
}

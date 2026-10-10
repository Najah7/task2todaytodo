//go:build integration

package task_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTaskProgressCountsSavedOccurrencesAndCurrentVirtualOccurrenceOnce(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	asOf := time.Now()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := localDate(time.Now(), location)
	rootDate := today.AddDate(0, 0, -7)
	futureDate := today.AddDate(0, 0, 7)
	rootID := domain.ActionItemID(occurrenceTestID{}.Generate())
	createWeeklyActionItemRoot(t, ctx, pool, userID, taskID, rootID, rootDate, location)

	// Persist a future occurrence of the same series. The root, this saved
	// occurrence, and today's still-virtual occurrence are distinct work.
	if err := usecase.NewCompleteActionItemUseCase(occurrenceTestUOW{pool: pool}, nil, occurrenceTestID{}).ExecuteOccurrence(
		ctx, userID, taskID, rootID, futureDate.Format("2006-01-02"),
	); err != nil {
		t.Fatalf("save completed future occurrence: %v", err)
	}

	taskRepo := taskrepo.NewTaskRepository(pool)
	itemRepo := taskrepo.NewActionItemRepository(pool)
	listItems := func() []dao.ActionItem {
		t.Helper()
		page, err := usecase.NewListActionItemsUseCase(occurrenceTestUOW{pool: pool}, nil, func() time.Time { return asOf }).ExecutePage(ctx, userID, taskID, usecase.CursorPageRequest{Size: 20, AsOf: asOf})
		if err != nil {
			t.Fatalf("list monthly ActionItem projection: %v", err)
		}
		return page.Items
	}
	listTask := func() usecase.TaskListPage {
		t.Helper()
		page, err := usecase.NewListTasksUseCase(taskRepo, nil, itemRepo).ExecuteFilteredPage(ctx, userID, usecase.TaskListRequest{
			Size: 10, Status: "all", DueFilter: "all", SortBy: "due_date", SortOrder: "asc", AsOf: asOf,
		})
		if err != nil {
			t.Fatalf("project monthly Task progress: %v", err)
		}
		return page
	}
	currentItems := listItems()
	current := listTask()
	if len(currentItems) != 5 || len(current.Items) != 1 || current.Items[0].ActionItemCount != len(currentItems) || current.Items[0].ActionItemCompletedCount != 1 || current.Items[0].Progress != 20 {
		t.Errorf("current Task/list projection = %d ActionItems, %+v; want five in-window rows, one completed and 20%%", len(currentItems), current.Items)
	}
	if currentItems[1].OccurrenceDate != futureDate.Format("2006-01-02") || !currentItems[1].Completed {
		t.Fatalf("saved future occurrence in projected list = %+v; want completed row at %s", currentItems[1], futureDate.Format("2006-01-02"))
	}
	if len(current.Items) != 1 {
		t.Fatalf("current Task page has %d rows; want one", len(current.Items))
	}
	if err := usecase.NewCompleteActionItemUseCase(occurrenceTestUOW{pool: pool}, nil, occurrenceTestID{}).ExecuteOccurrence(
		ctx, userID, taskID, rootID, today.Format("2006-01-02"),
	); err != nil {
		t.Fatalf("materialize and complete today's virtual occurrence: %v", err)
	}
	materializedItems := listItems()
	materialized := listTask()
	if len(materializedItems) != 5 || len(materialized.Items) != 1 || materialized.Items[0].ActionItemCount != len(materializedItems) || materialized.Items[0].ActionItemCompletedCount != 2 || materialized.Items[0].Progress != 40 {
		t.Errorf("materialized Task/list projection = %d ActionItems, %+v; want five rows, two completed and 40%%", len(materializedItems), materialized.Items)
	}

	nextAsOf := asOf.AddDate(0, 0, 1)
	rolledItems, err := usecase.NewListActionItemsUseCase(occurrenceTestUOW{pool: pool}, nil, func() time.Time { return nextAsOf }).ExecutePage(ctx, userID, taskID, usecase.CursorPageRequest{Size: 20, AsOf: nextAsOf})
	if err != nil {
		t.Fatalf("list next-day monthly ActionItem projection: %v", err)
	}
	rolledTasks, err := usecase.NewListTasksUseCase(taskRepo, nil, itemRepo).ExecuteFilteredPage(ctx, userID, usecase.TaskListRequest{
		Size: 10, Status: "all", DueFilter: "all", SortBy: "due_date", SortOrder: "asc", AsOf: nextAsOf,
	})
	if err != nil {
		t.Fatalf("project next-day Task progress: %v", err)
	}
	if len(rolledItems.Items) != 5 || len(rolledTasks.Items) != 1 || rolledTasks.Items[0].ActionItemCount != len(rolledItems.Items) || rolledTasks.Items[0].ActionItemCompletedCount != 2 || rolledTasks.Items[0].Progress != 40 {
		t.Errorf("next-day Task/list projection = %d ActionItems, %+v; want five rows and matching 2/5 progress", len(rolledItems.Items), rolledTasks.Items)
	}
	status, err := taskRepo.GetByUserID(ctx, userID, taskID)
	if err != nil {
		t.Fatalf("read task after progress reads: %v", err)
	}
	if status.Status.Value != "open" {
		t.Errorf("task status after read and date rollover = %q, want open", status.Status.Value)
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
	repo := taskrepo.NewActionItemRepository(pool)
	itemIDs := make([]domain.ActionItemID, 0, 2)
	for index := 0; index < 2; index++ {
		id := domain.ActionItemID(occurrenceTestID{}.Generate())
		itemIDs = append(itemIDs, id)
		item, err := domain.NewActionItemWithDetails(id, taskID, "Explicit status item", "", today, false, index, domain.OnceIntervalWeeks, nil)
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
	tasks := taskrepo.NewTaskRepository(pool)
	currentTask, err := tasks.GetByUserID(ctx, userID, taskID)
	if err != nil {
		t.Fatalf("load task before explicit completion: %v", err)
	}
	if _, err := usecase.NewCompleteTaskUseCase(occurrenceTestUOW{pool: pool}, nil, nil).Execute(ctx, userID, taskID, currentTask.Revision); err != nil {
		t.Fatalf("explicitly complete task: %v", err)
	}
	page, err := usecase.NewListTasksUseCase(tasks, nil, repo).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
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
	page, err = usecase.NewListTasksUseCase(tasks, nil, repo).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].Status.Value != "open" || page.Items[0].Progress != 0 {
		t.Fatalf("explicitly reopened task page = %+v, error=%v, want open at 0%% while children remain unfinished", page.Items, err)
	}
	uow := occurrenceTestUOW{pool: pool}
	for _, id := range itemIDs {
		if err := usecase.NewCompleteActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, id); err != nil {
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
	page, err = usecase.NewListTasksUseCase(tasks, nil, repo).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
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
	rootID := domain.ActionItemID(occurrenceTestID{}.Generate())
	createWeeklyActionItemRoot(t, ctx, pool, userID, taskID, rootID, today.AddDate(0, 0, -7), location)
	targetDate := today.AddDate(0, 0, 7)
	wantErr := errors.New("progress query failed")
	uow := progressFailureUOW{base: occurrenceTestUOW{pool: pool}, failAt: 2, err: wantErr}
	err = usecase.NewSkipActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, targetDate.Format("2006-01-02"))
	if !errors.Is(err, wantErr) {
		t.Fatalf("skip error = %v, want progress query failure", err)
	}
	skipped, err := taskrepo.NewActionItemRepository(pool).ListActionItemSkippedOccurrences(ctx, userID, taskID, rootID)
	if err != nil {
		t.Fatalf("read skipped occurrences after rollback: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped dates after rollback = %v, want none", skipped)
	}
	assertProgressTaskStatus(t, ctx, pool, userID, taskID, "open")
}

func TestSkipKeepsMonthlyListOpenAndRestoreKeepsProgress(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := localDate(time.Now(), location)
	rootDate := today.AddDate(0, 0, -7)
	rootID := domain.ActionItemID(occurrenceTestID{}.Generate())
	createWeeklyActionItemRoot(t, ctx, pool, userID, taskID, rootID, rootDate, location)
	uow := occurrenceTestUOW{pool: pool}
	itemsRepository := taskrepo.NewActionItemRepository(pool)

	if err := usecase.NewCompleteActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID); err != nil {
		t.Fatalf("complete saved root occurrence: %v", err)
	}
	if err := usecase.NewSkipActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, today.Format("2006-01-02")); err != nil {
		t.Fatalf("skip today's virtual occurrence: %v", err)
	}
	assertProgressTaskStatus(t, ctx, pool, userID, taskID, "open")

	if err := usecase.NewRestoreActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, today.Format("2006-01-02")); err != nil {
		t.Fatalf("restore today's virtual occurrence: %v", err)
	}
	assertProgressTaskStatus(t, ctx, pool, userID, taskID, "open")

	page, err := usecase.NewListTasksUseCase(taskrepo.NewTaskRepository(pool), nil, itemsRepository).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil {
		t.Fatalf("read task progress after restore: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ActionItemCount == 0 || page.Items[0].Progress != 100/page.Items[0].ActionItemCount {
		t.Fatalf("restored task page = %+v, want progress matching one completed saved row in the finite monthly list", page.Items)
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
	rootID := domain.ActionItemID(occurrenceTestID{}.Generate())
	createWeeklyActionItemRoot(t, ctx, pool, userID, taskID, rootID, today.AddDate(0, 0, -7), location)
	futureDate := today.AddDate(0, 0, 7)
	uow := occurrenceTestUOW{pool: pool}
	list := func() []dao.ActionItem {
		t.Helper()
		page, err := usecase.NewListActionItemsUseCase(uow, nil, func() time.Time { return asOf }).ExecutePage(ctx, userID, taskID, usecase.CursorPageRequest{
			Size: 10, FromDate: futureDate.Format("2006-01-02"), AsOf: asOf,
		})
		if err != nil {
			t.Fatalf("list future actionItem occurrences: %v", err)
		}
		return page.Items
	}
	progress := func() (int, string) {
		t.Helper()
		page, err := usecase.NewListTasksUseCase(taskrepo.NewTaskRepository(pool), nil, taskrepo.NewActionItemRepository(pool)).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
		if err != nil {
			t.Fatalf("read task progress: %v", err)
		}
		if len(page.Items) != 1 {
			t.Fatalf("task page has %d items, want one", len(page.Items))
		}
		return page.Items[0].Progress, page.Items[0].Status.Value
	}

	beforeList := list()
	containsFutureDate := func(rows []dao.ActionItem) bool {
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

	if err := usecase.NewSkipActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, futureDate.Format("2006-01-02")); err != nil {
		t.Fatalf("skip future occurrence: %v", err)
	}
	if skipped := list(); containsFutureDate(skipped) {
		t.Fatalf("future list while skipped still includes %s: %+v", futureDate.Format("2006-01-02"), skipped)
	}
	if gotProgress, gotStatus := progress(); gotProgress != beforeProgress || gotStatus != beforeStatus {
		t.Fatalf("progress while skipping future date = %d/%s, before was %d/%s", gotProgress, gotStatus, beforeProgress, beforeStatus)
	}

	if err := usecase.NewRestoreActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, rootID, futureDate.Format("2006-01-02")); err != nil {
		t.Fatalf("restore future occurrence: %v", err)
	}
	afterList := list()
	if !reflect.DeepEqual(afterList, beforeList) {
		t.Fatalf("future list after restore = %+v, before skip was %+v", afterList, beforeList)
	}
	if gotProgress, gotStatus := progress(); gotProgress != beforeProgress || gotStatus != beforeStatus {
		t.Fatalf("progress after restore = %d/%s, before was %d/%s", gotProgress, gotStatus, beforeProgress, beforeStatus)
	}
}

func TestConcurrentFinalActionItemCompletionsSetTaskDone(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := localDate(time.Now(), location)
	repo := taskrepo.NewActionItemRepository(pool)
	ids := []domain.ActionItemID{domain.ActionItemID(occurrenceTestID{}.Generate()), domain.ActionItemID(occurrenceTestID{}.Generate())}
	for index, id := range ids {
		item, err := domain.NewActionItemWithDetails(id, taskID, "Concurrent item", "", today, false, index, domain.OnceIntervalWeeks, nil)
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
			errCh <- usecase.NewCompleteActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, id)
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Errorf("complete concurrent action item: %v", err)
		}
	}
	assertProgressTaskStatus(t, ctx, pool, userID, taskID, "done")
	page, err := usecase.NewListTasksUseCase(taskrepo.NewTaskRepository(pool), nil, repo).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil {
		t.Fatalf("read completed task progress: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Progress != 100 {
		t.Fatalf("completed task page = %+v, want one task at 100%%", page.Items)
	}
}

func createWeeklyActionItemRoot(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, rootDate time.Time, location *time.Location) {
	t.Helper()
	// Persist the anchor as UTC midnight for the intended calendar date. The
	// recurrence command interprets FrequencyAnchorDate as a UTC calendar date.
	rootDate = time.Date(rootDate.Year(), rootDate.Month(), rootDate.Day(), 0, 0, 0, 0, time.UTC)
	frequency := strings.ToLower(rootDate.Weekday().String()[:3])
	weekday, err := domain.NewTaskFrequency(frequency)
	if err != nil {
		t.Fatal(err)
	}
	item, err := domain.NewActionItemWithDetails(id, taskID, "Weekly progress item", "", rootDate, false, 0, 1, domain.TaskFrequencies{weekday})
	if err != nil {
		t.Fatal(err)
	}
	item, err = item.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(id), OccurrenceDate: rootDate, Timezone: location.String()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := taskrepo.NewActionItemRepository(pool).CreateForOwnedTask(ctx, userID, item, false); err != nil {
		t.Fatalf("create weekly actionItem root: %v", err)
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
	task, err := taskrepo.NewTaskRepository(pool).GetByUserID(ctx, userID, taskID)
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
		actionItems := &progressFailureActionItemRepository{ActionItemRepository: repos.ActionItems(), failAt: uow.failAt, err: uow.err}
		return fn(ctx, occurrenceTestRepositories{task: repos.Tasks(), actionItem: actionItems})
	})
}

type progressFailureActionItemRepository struct {
	usecase.ActionItemRepository
	readCount int
	failAt    int
	err       error
}

func (repo *progressFailureActionItemRepository) ReadTaskListProjection(ctx context.Context, userID domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	repo.readCount++
	if repo.readCount == repo.failAt {
		return dao.TaskListProjectionSources{}, repo.err
	}
	return repo.ActionItemRepository.ReadTaskListProjection(ctx, userID, taskIDs)
}

func (repo *progressFailureActionItemRepository) ListByTaskForOccurrenceProjection(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.ActionItem, error) {
	return repo.ActionItemRepository.(interface {
		ListByTaskForOccurrenceProjection(context.Context, domain.UserID, domain.TaskID) ([]dao.ActionItem, error)
	}).ListByTaskForOccurrenceProjection(ctx, userID, taskID)
}

func (repo *progressFailureActionItemRepository) ListByTaskForOccurrenceCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, capability shared.Capability) ([]dao.ActionItem, error) {
	return repo.ActionItemRepository.(interface {
		ListByTaskForOccurrenceCommand(context.Context, domain.UserID, domain.TaskID, shared.Capability) ([]dao.ActionItem, error)
	}).ListByTaskForOccurrenceCommand(ctx, userID, taskID, capability)
}

func (repo *progressFailureActionItemRepository) GetForCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, capability shared.Capability) (dao.ActionItem, error) {
	return repo.ActionItemRepository.(interface {
		GetForCommand(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, shared.Capability) (dao.ActionItem, error)
	}).GetForCommand(ctx, userID, taskID, id, capability)
}

func (repo *progressFailureActionItemRepository) ListActionItemSkippedOccurrences(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID) ([]int64, error) {
	return repo.ActionItemRepository.(interface {
		ListActionItemSkippedOccurrences(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID) ([]int64, error)
	}).ListActionItemSkippedOccurrences(ctx, userID, taskID, seriesID)
}

func (repo *progressFailureActionItemRepository) ListActionItemSkippedOccurrencesForCapability(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, capability shared.Capability) ([]int64, error) {
	return repo.ActionItemRepository.(interface {
		ListActionItemSkippedOccurrencesForCapability(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, shared.Capability) ([]int64, error)
	}).ListActionItemSkippedOccurrencesForCapability(ctx, userID, taskID, seriesID, capability)
}

func (repo *progressFailureActionItemRepository) SetActionItemSkippedOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate time.Time, skipped bool) error {
	return repo.ActionItemRepository.(interface {
		SetActionItemSkippedOccurrence(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, time.Time, bool) error
	}).SetActionItemSkippedOccurrence(ctx, userID, taskID, seriesID, occurrenceDate, skipped)
}

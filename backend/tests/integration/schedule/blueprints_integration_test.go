//go:build integration

package schedule_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	schedulerepo "github.com/Najah7/task2todaytodo/internal/application/schedule/repository"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type blueprintTestUOW struct {
	pool      *pgxpool.Pool
	lifecycle shared.ProjectWorkLifecycle
}

type blueprintTestRepositories struct {
	schedules *schedulerepo.ScheduleRepository
	lifecycle shared.ProjectWorkLifecycle
}

func (r blueprintTestRepositories) Schedules() usecase.ScheduleRepository   { return r.schedules }
func (r blueprintTestRepositories) Blueprints() usecase.BlueprintRepository { return r.schedules }

func (u blueprintTestUOW) Do(ctx context.Context, fn func(context.Context, usecase.Repositories) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	lifecycle := u.lifecycle
	if lifecycle == nil {
		lifecycle = blueprintProjectLifecycle{}
	}
	repositories := blueprintTestRepositories{schedules: schedulerepo.NewScheduleRepository(tx), lifecycle: lifecycle}
	if err := fn(ctx, repositories); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (blueprintProjectLifecycle) LockParent(context.Context, string) error { return nil }

type blueprintProjectLifecycle struct{ status string }

func (l blueprintProjectLifecycle) ReadStatus(context.Context, string) (string, error) {
	if l.status != "" {
		return l.status, nil
	}
	return "open", nil
}
func (blueprintProjectLifecycle) CaptureWorkState(context.Context, string, time.Time) (shared.ProjectWorkState, error) {
	return shared.ProjectWorkState{Status: "open"}, nil
}
func (blueprintProjectLifecycle) ReconcileWorkState(context.Context, string, string, shared.ProjectWorkState, time.Time) error {
	return nil
}

func (r blueprintTestRepositories) ProjectLifecycle() shared.ProjectWorkLifecycle { return r.lifecycle }

type blueprintTestIDs struct{}

func (blueprintTestIDs) Generate() string { return ulid.Make().String() }

type blueprintTimezoneValidator struct{ queries *sqlc.Queries }

func (v blueprintTimezoneValidator) TimezoneExists(ctx context.Context, zone string) (bool, error) {
	return v.queries.IsTimezoneAllowed(ctx, zone)
}

func TestBlueprintCreateAndVirtualListDoesNotPersistRecurringRows(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	ids := blueprintTestIDs{}
	now := time.Now().UTC()
	localToday, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(localToday)
	weekStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	weekStart = weekStart.AddDate(0, 0, -((int(weekStart.Weekday()) + 6) % 7))
	clock := func() time.Time { return now }
	create := usecase.NewCreateBlueprintUseCase(uow, ids, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, clock)

	onceStart := weekStart.AddDate(0, 0, 1).Add(11 * time.Hour)
	once, err := create.Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "One-off planning session", Timezone: "Asia/Tokyo", LocalStart: onceStart,
		LocalEnd: onceStart.Add(time.Hour), Frequency: "once",
	})
	if err != nil {
		t.Fatalf("create one-off blueprint: %v", err)
	}
	recurringStart := weekStart.Add(9 * time.Hour)
	recurring, err := create.Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Daily focus block", Timezone: "Asia/Tokyo", LocalStart: recurringStart,
		LocalEnd: recurringStart.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurring blueprint: %v", err)
	}
	var onceRows, recurringRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedules WHERE blueprint_id=$1`, once.ID).Scan(&onceRows); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedules WHERE blueprint_id=$1`, recurring.ID).Scan(&recurringRows); err != nil {
		t.Fatal(err)
	}
	if onceRows != 1 || recurringRows != 0 {
		t.Fatalf("concrete rows: one-off=%d recurring=%d; want 1 and 0", onceRows, recurringRows)
	}
	var concreteScheduleID string
	if err := pool.QueryRow(ctx, `SELECT id FROM schedules WHERE blueprint_id=$1`, once.ID).Scan(&concreteScheduleID); err != nil {
		t.Fatalf("read concrete one-off schedule: %v", err)
	}
	todoListID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO todo_lists(id,user_id,list_date) VALUES($1,$2,$3)`, todoListID, userID, time.Now().UTC().Format("2006-01-02")); err != nil {
		t.Fatalf("create TodoList: %v", err)
	}
	if _, err := sqlc.New(pool).AddTodoListSchedule(ctx, sqlc.AddTodoListScheduleParams{TodoListID: todoListID, ScheduleID: concreteScheduleID}); err != nil {
		t.Fatalf("link independent concrete Schedule into TodoList: %v", err)
	}
	linked, err := sqlc.New(pool).ListTodoListSchedules(ctx, todoListID)
	if err != nil || len(linked) != 1 || linked[0].ScheduleID != concreteScheduleID {
		t.Fatalf("TodoList Schedule links=%v, err=%v; want concrete occurrence %s", linked, err, concreteScheduleID)
	}

	list := usecase.NewListOccurrencesUseCase(uow)
	from := weekStart.UTC()
	to := from.AddDate(0, 0, 4)
	page, err := list.ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 20, From: from, To: to})
	if err != nil {
		t.Fatalf("list virtual and persisted occurrences: %v", err)
	}
	if len(page.Items) != 5 {
		t.Fatalf("listed occurrence count=%d; want one persisted and four virtual: %+v", len(page.Items), page.Items)
	}
	var virtualDaily int
	for _, item := range page.Items {
		if item.BlueprintID == recurring.ID && item.ID == nil {
			virtualDaily++
		}
	}
	if virtualDaily != 4 {
		t.Fatalf("virtual daily occurrences=%d; want 4", virtualDaily)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedules WHERE blueprint_id=$1`, recurring.ID).Scan(&recurringRows); err != nil {
		t.Fatal(err)
	}
	if recurringRows != 0 {
		t.Fatalf("read-only list persisted %d recurring rows", recurringRows)
	}
}

func TestMovedOccurrenceKeepsOriginalIdentityAndAppearsOnlyAtActualTime(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(location)
	localDate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Daily stand-up", Timezone: "Asia/Tokyo", LocalStart: localDate.Add(9 * time.Hour),
		LocalEnd: localDate.Add(10 * time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurring blueprint: %v", err)
	}
	originalDate := localDate.Format("2006-01-02")
	movedStart := time.Date(localDate.Year(), localDate.Month(), localDate.Day()+1, 11, 0, 0, 0, location)
	movedEnd := movedStart.Add(time.Hour)
	moved, err := usecase.NewRescheduleOccurrenceUseCase(uow, nil, blueprintTestIDs{}).Execute(ctx, userID, blueprint.ID, originalDate, movedStart, movedEnd, 0)
	if err != nil {
		t.Fatalf("move occurrence: %v", err)
	}
	if moved.BlueprintID != blueprint.ID || moved.OccurrenceDate != originalDate || moved.ID == nil || moved.StartAt != movedStart.Unix() {
		t.Fatalf("moved occurrence lost its original identity: %+v", moved)
	}
	if _, err := usecase.NewRescheduleOccurrenceUseCase(uow, nil, blueprintTestIDs{}).Execute(ctx, userID, blueprint.ID,
		originalDate, movedStart.Add(time.Hour), movedEnd.Add(time.Hour), 0); err != usecase.ErrOccurrenceRevisionMismatch {
		t.Fatalf("stale reschedule revision error=%v, want ErrOccurrenceRevisionMismatch", err)
	}

	list := usecase.NewListOccurrencesUseCase(uow)
	dayStart := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 0, 0, 0, 0, location).UTC()
	originPage, err := list.ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 10, From: dayStart, To: dayStart.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range originPage.Items {
		if item.BlueprintID == blueprint.ID && item.OccurrenceDate == originalDate {
			t.Fatalf("moved origin still appears at original time: %+v", item)
		}
	}
	movedDay := dayStart.Add(24 * time.Hour)
	movedPage, err := list.ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 10, From: movedDay, To: movedDay.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	foundMoved := false
	for _, item := range movedPage.Items {
		if item.BlueprintID == blueprint.ID && item.OccurrenceDate == originalDate && item.StartAt == movedStart.Unix() {
			foundMoved = true
		}
	}
	if !foundMoved {
		t.Fatalf("moved occurrence not returned at its actual overlapping time: %+v", movedPage.Items)
	}
}

func TestBackfillHonorsCutoffBoundaryAndNeverOverwritesSavedOverride(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := now.In(location)
	localDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	startDate := localDate.AddDate(0, 0, -3)
	create := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now })
	makeDaily := func(title string, hour time.Duration) usecase.CreateBlueprintInput {
		start := startDate.Add(hour)
		return usecase.CreateBlueprintInput{Title: title, Timezone: "Asia/Tokyo", LocalStart: start,
			LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1}
	}
	before, err := create.Execute(ctx, userID, makeDaily("Before cutoff", 9*time.Hour))
	if err != nil {
		t.Fatalf("create before-cutoff rule: %v", err)
	}
	equal, err := create.Execute(ctx, userID, makeDaily("At cutoff", 12*time.Hour))
	if err != nil {
		t.Fatalf("create exact-cutoff rule: %v", err)
	}
	future, err := create.Execute(ctx, userID, makeDaily("After cutoff", 14*time.Hour))
	if err != nil {
		t.Fatalf("create after-cutoff rule: %v", err)
	}
	// Save an override before backfill reaches its source date. Backfill must
	// retain the moved snapshot and never recreate the origin for that key.
	overrideDate := startDate.AddDate(0, 0, 1).Format("2006-01-02")
	movedStart := time.Date(today.Year(), today.Month(), today.Day()+2, 16, 0, 0, 0, location)
	_, err = usecase.NewRescheduleOccurrenceUseCase(uow, nil, blueprintTestIDs{}).Execute(ctx, userID, before.ID, overrideDate, movedStart, movedStart.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("save override before backfill: %v", err)
	}
	backfill := usecase.NewBackfillUseCase(uow, blueprintTestIDs{})
	cutoff := time.Date(today.Year(), today.Month(), today.Day(), 12, 0, 0, 0, location).UTC()
	if _, err := backfill.Execute(ctx, userID, cutoff, nil); err != nil {
		t.Fatalf("backfill at noon cutoff: %v", err)
	}
	todayKey := localDate.Format("2006-01-02")
	countOnDate := func(blueprintID string) int {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedules WHERE blueprint_id=$1 AND occurrence_date=$2`, blueprintID, todayKey).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	if countOnDate(before.ID) != 1 || countOnDate(equal.ID) != 0 || countOnDate(future.ID) != 0 {
		t.Fatalf("noon cutoff rows: before=%d equal=%d future=%d; want 1, 0, 0", countOnDate(before.ID), countOnDate(equal.ID), countOnDate(future.ID))
	}
	// Advancing the cutoff on the same local day must revisit occurrences whose
	// start had not yet passed; a date-only checkpoint must not lose them.
	laterCutoff := time.Date(today.Year(), today.Month(), today.Day(), 15, 0, 0, 0, location).UTC()
	if _, err := backfill.Execute(ctx, userID, laterCutoff, nil); err != nil {
		t.Fatalf("backfill later on same local day: %v", err)
	}
	if countOnDate(equal.ID) != 1 || countOnDate(future.ID) != 1 {
		t.Fatalf("later cutoff rows: equal=%d future=%d; want 1 and 1", countOnDate(equal.ID), countOnDate(future.ID))
	}
	var persistedStart time.Time
	if err := pool.QueryRow(ctx, `SELECT start_at FROM schedules WHERE blueprint_id=$1 AND occurrence_date=$2`, before.ID, overrideDate).Scan(&persistedStart); err != nil {
		t.Fatalf("read saved override: %v", err)
	}
	if !persistedStart.Equal(movedStart.UTC()) {
		t.Fatalf("backfill overwrote saved override start: got %s, want %s", persistedStart, movedStart.UTC())
	}
}

func TestDeleteFollowingSuppressesVirtualTailAndSavedOverride(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	viewerID := createScheduleTestUser(t, pool)
	projectID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other','Deleted Schedule Audit project','low','open',$2)`, projectID, userID); err != nil {
		t.Fatalf("create audit project: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'viewer',$3)`, projectID, viewerID, userID); err != nil {
		t.Fatalf("add project viewer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
	})
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 14)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Recurring review", ProjectID: projectID, Timezone: "Asia/Tokyo", LocalStart: startDate.Add(9 * time.Hour),
		LocalEnd: startDate.Add(10 * time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurring blueprint: %v", err)
	}
	deletedFrom := startDate.AddDate(0, 0, 3)
	savedDate := startDate.AddDate(0, 0, 5)
	movedStart := time.Date(savedDate.Year(), savedDate.Month(), savedDate.Day()+2, 16, 0, 0, 0, location)
	_, err = usecase.NewRescheduleOccurrenceUseCase(uow, nil, blueprintTestIDs{}).Execute(ctx, userID, blueprint.ID,
		savedDate.Format("2006-01-02"), movedStart, movedStart.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("save tail override: %v", err)
	}
	if _, err := usecase.NewGetOccurrenceUseCase(uow).Execute(ctx, viewerID, blueprint.ID, savedDate.Format("2006-01-02")); err != nil {
		t.Fatalf("Project viewer cannot read saved occurrence before deletion: %v", err)
	}
	deleteFollowing := usecase.NewDeleteBlueprintFollowingUseCase(uow, blueprintTestIDs{}, func() time.Time { return now })
	if err := deleteFollowing.Execute(ctx, userID, blueprint.ID, deletedFrom.Format("2006-01-02"), blueprint.Revision); err != nil {
		t.Fatalf("delete following tail: %v", err)
	}
	from := time.Date(deletedFrom.Year(), deletedFrom.Month(), deletedFrom.Day(), 0, 0, 0, 0, location).UTC()
	page, err := usecase.NewListOccurrencesUseCase(uow).ExecutePage(ctx, userID, usecase.CursorPageRequest{
		Size: 50, From: from, To: from.AddDate(0, 0, 20),
	})
	if err != nil {
		t.Fatalf("list after deleting tail: %v", err)
	}
	for _, occurrence := range page.Items {
		if occurrence.BlueprintID == blueprint.ID && occurrence.OccurrenceDate >= deletedFrom.Format("2006-01-02") {
			t.Fatalf("deleted tail was projected or saved override resurfaced: %+v", occurrence)
		}
	}
	var deletedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT deleted_at FROM schedules WHERE blueprint_id=$1 AND occurrence_date=$2`, blueprint.ID, savedDate.Format("2006-01-02")).Scan(&deletedAt); err != nil {
		t.Fatalf("read deleted tail override: %v", err)
	}
	if deletedAt == nil {
		t.Fatal("saved override in deleted tail was not tombstoned")
	}
	history := usecase.NewListOccurrenceRevisionsUseCase(uow)
	ownerHistory, err := history.ExecutePage(ctx, userID, blueprint.ID, savedDate.Format("2006-01-02"), 20, 0)
	if err != nil || len(ownerHistory.Items) == 0 {
		t.Fatalf("owner could not read cancelled occurrence history: items=%v err=%v", ownerHistory.Items, err)
	}
	if _, err := history.ExecutePage(ctx, viewerID, blueprint.ID, savedDate.Format("2006-01-02"), 20, 0); err == nil {
		t.Fatal("Project viewer could read cancelled occurrence audit history")
	}
}

func TestDeleteFollowingAdvancesAggregateRevisionAndInvalidatesStaleEditor(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 10)
	start := startDate.Add(9 * time.Hour)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Deletion revision", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurrence: %v", err)
	}
	deletedFrom := startDate.AddDate(0, 0, 3)
	if err := usecase.NewDeleteBlueprintFollowingUseCase(uow, blueprintTestIDs{}, func() time.Time { return now }).Execute(ctx, userID, blueprint.ID, deletedFrom.Format("2006-01-02"), blueprint.Revision); err != nil {
		t.Fatalf("delete future tail: %v", err)
	}
	current, err := usecase.NewGetBlueprintUseCase(uow).Execute(ctx, userID, blueprint.ID)
	if err != nil {
		t.Fatalf("get blueprint after deletion: %v", err)
	}
	if current.Revision <= blueprint.Revision {
		t.Fatalf("aggregate revision after deletion=%d, want greater than initial %d", current.Revision, blueprint.Revision)
	}
	staleTitle := "Stale earlier edit"
	if _, err := usecase.NewUpdateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now }).Execute(
		ctx, userID, blueprint.ID, startDate.AddDate(0, 0, 1).Format("2006-01-02"), "following",
		usecase.UpdateBlueprintInput{Title: &staleTitle}, blueprint.Revision,
	); !errors.Is(err, usecase.ErrOccurrenceRevisionMismatch) {
		t.Fatalf("earlier edit with stale pre-deletion revision error=%v, want ErrOccurrenceRevisionMismatch", err)
	}
	updatedTitle := "Earlier edit after cancellation"
	if _, err := usecase.NewUpdateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now }).Execute(
		ctx, userID, blueprint.ID, startDate.AddDate(0, 0, 1).Format("2006-01-02"), "following",
		usecase.UpdateBlueprintInput{Title: &updatedTitle}, current.Revision,
	); err != nil {
		t.Fatalf("earlier edit with fresh post-deletion revision: %v", err)
	}
	from := deletedFrom.UTC()
	page, err := usecase.NewListOccurrencesUseCase(uow).ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 20, From: from, To: from.AddDate(0, 0, 7)})
	if err != nil {
		t.Fatalf("list after earlier edit following cancellation: %v", err)
	}
	for _, occurrence := range page.Items {
		if occurrence.BlueprintID == blueprint.ID && occurrence.OccurrenceDate >= deletedFrom.Format("2006-01-02") {
			t.Fatalf("earlier edit revived cancelled tail: %+v", occurrence)
		}
	}
}

func TestSkipRetryIsIdempotentAndRestoreOnlyUnskips(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, 2).Add(9 * time.Hour)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Daily task", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurring blueprint: %v", err)
	}
	date := start.Format("2006-01-02")
	get := usecase.NewGetOccurrenceUseCase(uow)
	skip := usecase.NewSetOccurrenceStateUseCase(uow, "skip", nil, blueprintTestIDs{})
	if err := skip.Execute(ctx, userID, blueprint.ID, date, 0); err != nil {
		t.Fatalf("skip virtual occurrence: %v", err)
	}
	skipped, err := get.Execute(ctx, userID, blueprint.ID, date)
	if err != nil || !skipped.Skipped || skipped.ID == nil {
		t.Fatalf("read skipped occurrence = %+v, %v", skipped, err)
	}
	if err := skip.Execute(ctx, userID, blueprint.ID, date, skipped.Revision); err != nil {
		t.Fatalf("retry skip with current revision: %v", err)
	}
	stillSkipped, err := get.Execute(ctx, userID, blueprint.ID, date)
	if err != nil || !stillSkipped.Skipped || stillSkipped.Revision != skipped.Revision {
		t.Fatalf("retry changed skipped occurrence: %+v, %v", stillSkipped, err)
	}
	restore := usecase.NewSetOccurrenceStateUseCase(uow, "restore", nil, blueprintTestIDs{})
	if err := restore.Execute(ctx, userID, blueprint.ID, date, stillSkipped.Revision); err != nil {
		t.Fatalf("restore skipped occurrence: %v", err)
	}
	restored, err := get.Execute(ctx, userID, blueprint.ID, date)
	if err != nil || restored.Skipped || restored.Revision == stillSkipped.Revision {
		t.Fatalf("read restored occurrence = %+v, %v", restored, err)
	}
	delete := usecase.NewSetOccurrenceStateUseCase(uow, "delete", nil, blueprintTestIDs{})
	if err := delete.Execute(ctx, userID, blueprint.ID, date, restored.Revision); err != nil {
		t.Fatalf("delete occurrence: %v", err)
	}
	var deletedRevision int32
	var deletedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT revision, deleted_at FROM schedules WHERE blueprint_id=$1 AND occurrence_date=$2`, blueprint.ID, date).Scan(&deletedRevision, &deletedAt); err != nil {
		t.Fatalf("read deleted occurrence row: %v", err)
	}
	if err := restore.Execute(ctx, userID, blueprint.ID, date, deletedRevision); err == nil {
		t.Fatal("restore cleared a deletion tombstone; deleted occurrences are only canceled by the requested API")
	}
}

func TestFollowingEditUsesOriginalStartAndRejectsOccurrenceDateChange(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	create := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now })
	makeDaily := func(title string, date time.Time) usecase.CreateBlueprintInput {
		start := date.Add(9 * time.Hour)
		return usecase.CreateBlueprintInput{Title: title, Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1}
	}
	update := func(id, date string, revision int32, patch usecase.UpdateBlueprintInput) (dao.Blueprint, error) {
		return usecase.NewUpdateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now }).Execute(ctx, userID, id, date, "following", patch, revision)
	}

	past := today.AddDate(0, 0, -4)
	pastRule, err := create.Execute(ctx, userID, makeDaily("Past original", past))
	if err != nil {
		t.Fatalf("create past rule: %v", err)
	}
	pastOccurrence := past.AddDate(0, 0, 1)
	movedFuture := time.Date(today.Year(), today.Month(), today.Day()+5, 15, 0, 0, 0, location)
	if _, err := usecase.NewRescheduleOccurrenceUseCase(uow, nil, blueprintTestIDs{}).Execute(ctx, userID, pastRule.ID,
		pastOccurrence.Format("2006-01-02"), movedFuture, movedFuture.Add(time.Hour), 0); err != nil {
		t.Fatalf("move past original into future: %v", err)
	}
	title := "must reject based on original start"
	if _, err := update(pastRule.ID, pastOccurrence.Format("2006-01-02"), pastRule.Revision, usecase.UpdateBlueprintInput{Title: &title}); err != usecase.ErrOccurrenceInactive {
		t.Fatalf("following edit for moved-future past origin error = %v, want original-occurrence guard", err)
	}

	futureDate := today.AddDate(0, 0, 5)
	futureRule, err := create.Execute(ctx, userID, makeDaily("Future original", futureDate))
	if err != nil {
		t.Fatalf("create future rule: %v", err)
	}
	futureOccurrence := futureDate.AddDate(0, 0, 1)
	movedPast := time.Date(today.Year(), today.Month(), today.Day()-2, 15, 0, 0, 0, location)
	if _, err := usecase.NewRescheduleOccurrenceUseCase(uow, nil, blueprintTestIDs{}).Execute(ctx, userID, futureRule.ID,
		futureOccurrence.Format("2006-01-02"), movedPast, movedPast.Add(time.Hour), 0); err != nil {
		t.Fatalf("move future original into past: %v", err)
	}
	localStartOnWrongDate := futureDate.AddDate(0, 0, 2).Add(10 * time.Hour)
	if _, err := update(futureRule.ID, futureOccurrence.Format("2006-01-02"), futureRule.Revision,
		usecase.UpdateBlueprintInput{LocalStart: &localStartOnWrongDate}); err != usecase.ErrRescheduleDateMismatch {
		t.Fatalf("following start date change error = %v, want ErrRescheduleDateMismatch", err)
	}
	newTitle := "valid following change"
	if _, err := update(futureRule.ID, futureOccurrence.Format("2006-01-02"), futureRule.Revision,
		usecase.UpdateBlueprintInput{Title: &newTitle}); err != nil {
		t.Fatalf("following edit for moved-past future origin: %v", err)
	}
}

func TestOnceRepeatConversionsAndFollowingBaseEdit(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	date := today.AddDate(0, 0, 5)
	start := date.Add(9 * time.Hour)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "One-off", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "once",
	})
	if err != nil {
		t.Fatalf("create one-off: %v", err)
	}
	update := usecase.NewUpdateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
	dateKey := date.Format("2006-01-02")
	newTitle := "Edited one-off"
	current, err := update.Execute(ctx, userID, blueprint.ID, dateKey, "following", usecase.UpdateBlueprintInput{Title: &newTitle}, blueprint.Revision)
	if err != nil {
		t.Fatalf("edit one-off following: %v", err)
	}
	occurrence, err := usecase.NewGetOccurrenceUseCase(uow).Execute(ctx, userID, blueprint.ID, dateKey)
	if err != nil || occurrence.Title != newTitle {
		t.Fatalf("one-off base edit occurrence = %+v, %v", occurrence, err)
	}
	daily, interval := "daily", 1
	current, err = update.Execute(ctx, userID, blueprint.ID, dateKey, "following", usecase.UpdateBlueprintInput{Frequency: &daily, Interval: &interval}, current.Revision)
	if err != nil {
		t.Fatalf("convert once to repeat: %v", err)
	}
	once := "once"
	nextDate := date.AddDate(0, 0, 2)
	current, err = update.Execute(ctx, userID, blueprint.ID, nextDate.Format("2006-01-02"), "following",
		usecase.UpdateBlueprintInput{Frequency: &once}, current.Revision)
	if err != nil {
		t.Fatalf("convert repeat to once: %v", err)
	}
	if current.Frequency != "once" {
		t.Fatalf("current frequency=%q, want once", current.Frequency)
	}
	from := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	page, err := usecase.NewListOccurrencesUseCase(uow).ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 20, From: from, To: from.AddDate(0, 0, 5)})
	if err != nil {
		t.Fatalf("list converted recurrence: %v", err)
	}
	allowedDates := map[string]bool{
		date.AddDate(0, 0, 1).Format("2006-01-02"): true,
		nextDate.Format("2006-01-02"):              true,
	}
	for _, row := range page.Items {
		if row.BlueprintID == blueprint.ID && !allowedDates[row.OccurrenceDate] {
			t.Fatalf("repeat-to-once left a virtual recurrence at %s: %+v", row.OccurrenceDate, row)
		}
	}
	if current.Revision != blueprint.Revision+3 {
		t.Fatalf("aggregate revision=%d, want %d", current.Revision, blueprint.Revision+3)
	}
}

func TestFollowingEditKeepsMultidayDurationAcrossYearBoundary(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	start := time.Date(2026, time.December, 30, 22, 0, 0, 0, time.UTC)
	end := time.Date(2027, time.January, 1, 1, 0, 0, 0, time.UTC)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Overnight operation", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: end, Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create multi-day rule: %v", err)
	}
	date := "2026-12-31"
	title := "Updated overnight operation"
	if _, err := usecase.NewUpdateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now }).Execute(ctx, userID,
		blueprint.ID, date, "following", usecase.UpdateBlueprintInput{Title: &title}, blueprint.Revision); err != nil {
		t.Fatalf("following edit over year boundary: %v", err)
	}
	occurrence, err := usecase.NewGetOccurrenceUseCase(uow).Execute(ctx, userID, blueprint.ID, date)
	if err != nil {
		t.Fatalf("get updated occurrence: %v", err)
	}
	if got, want := occurrence.EndAt-occurrence.StartAt, int64(27*60*60); got != want {
		t.Fatalf("following edit changed multi-day duration: got %s, want %s", time.Duration(got)*time.Second, time.Duration(want)*time.Second)
	}
}

func TestEarlierFollowingEditSupersedesPlannedRevisionAndRejectsStaleRevision(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 3)
	start := startDate.Add(9 * time.Hour)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Original", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurrence: %v", err)
	}
	update := usecase.NewUpdateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
	plannedDate := startDate.AddDate(0, 0, 5).Format("2006-01-02")
	plannedTitle := "Planned future revision"
	planned, err := update.Execute(ctx, userID, blueprint.ID, plannedDate, "following", usecase.UpdateBlueprintInput{Title: &plannedTitle}, blueprint.Revision)
	if err != nil {
		t.Fatalf("publish later following revision: %v", err)
	}
	earlierDate := startDate.AddDate(0, 0, 3).Format("2006-01-02")
	earlierTitle := "Replacement revision"
	replacement, err := update.Execute(ctx, userID, blueprint.ID, earlierDate, "following", usecase.UpdateBlueprintInput{Title: &earlierTitle}, planned.Revision)
	if err != nil {
		t.Fatalf("publish earlier following revision: %v", err)
	}
	if replacement.Revision != planned.Revision+1 {
		t.Fatalf("aggregate revision=%d, want %d", replacement.Revision, planned.Revision+1)
	}
	if _, err := update.Execute(ctx, userID, blueprint.ID, earlierDate, "following", usecase.UpdateBlueprintInput{Title: &plannedTitle}, blueprint.Revision); err != usecase.ErrOccurrenceRevisionMismatch {
		t.Fatalf("stale blueprint revision error=%v, want ErrOccurrenceRevisionMismatch", err)
	}
	get := usecase.NewGetOccurrenceUseCase(uow)
	for offset := 3; offset <= 5; offset++ {
		date := startDate.AddDate(0, 0, offset).Format("2006-01-02")
		occurrence, err := get.Execute(ctx, userID, blueprint.ID, date)
		if err != nil || occurrence.Title != earlierTitle {
			t.Fatalf("occurrence %s resurrected superseded planned revision: %+v, %v", date, occurrence, err)
		}
	}
}

func TestConcurrentFollowingEditsWithSameRevisionProduceOneConflict(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 7)
	start := startDate.Add(9 * time.Hour)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Concurrent edit", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurrence: %v", err)
	}

	date := startDate.AddDate(0, 0, 2).Format("2006-01-02")
	update := usecase.NewUpdateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
	titles := []string{"Concurrent edit A", "Concurrent edit B"}
	startGate := make(chan struct{})
	results := make([]error, len(titles))
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(len(titles))
	done.Add(len(titles))
	for i := range titles {
		go func(i int) {
			defer done.Done()
			title := titles[i]
			ready.Done()
			<-startGate
			_, results[i] = update.Execute(ctx, userID, blueprint.ID, date, "following", usecase.UpdateBlueprintInput{Title: &title}, blueprint.Revision)
		}(i)
	}
	ready.Wait()
	close(startGate)
	done.Wait()

	successes, conflicts := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, usecase.ErrOccurrenceRevisionMismatch):
			conflicts++
		default:
			t.Fatalf("concurrent following edit error=%v; want one success and one revision conflict", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent following edits yielded %d successes and %d conflicts; want 1 each", successes, conflicts)
	}
}

func TestProjectPauseResumePreservesCadenceAndBackfillHistory(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	projectID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other','Paused schedule project','low','open',$2)`, projectID, userID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, -8)
	start := startDate.Add(9 * time.Hour)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Project daily cadence", ProjectID: projectID, Timezone: "Asia/Tokyo", LocalStart: start,
		LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create project recurrence: %v", err)
	}
	pausedAt := time.Date(startDate.Year(), startDate.Month(), startDate.Day()+1, 8, 0, 0, 0, location).UTC()
	resumedAt := time.Date(startDate.Year(), startDate.Month(), startDate.Day()+3, 8, 0, 0, 0, location).UTC()
	if err := uow.Do(ctx, func(ctx context.Context, repos usecase.Repositories) error {
		if err := repos.Blueprints().PauseProjectBlueprints(ctx, userID, projectID, pausedAt); err != nil {
			return err
		}
		return repos.Blueprints().ResumeProjectBlueprints(ctx, userID, projectID, resumedAt)
	}); err != nil {
		t.Fatalf("pause and resume project schedules: %v", err)
	}
	var intervalCount int
	var persistedResume *time.Time
	if err := pool.QueryRow(ctx, `SELECT count(*), max(resumed_at) FROM schedule_project_pause_intervals WHERE project_id=$1`, projectID).Scan(&intervalCount, &persistedResume); err != nil {
		t.Fatalf("read pause history: %v", err)
	}
	if intervalCount != 1 || persistedResume == nil || !persistedResume.Equal(resumedAt) {
		t.Fatalf("pause history count=%d resumed_at=%v; want one interval ending at %s", intervalCount, persistedResume, resumedAt)
	}
	cutoff := time.Date(startDate.Year(), startDate.Month(), startDate.Day()+6, 12, 0, 0, 0, location).UTC()
	result, err := usecase.NewBackfillUseCase(uow, blueprintTestIDs{}).Execute(ctx, userID, cutoff, nil)
	if err != nil {
		t.Fatalf("backfill across pause interval: %v", err)
	}
	if result.GeneratedSchedules != 5 {
		t.Fatalf("backfill generated %d schedules, want 5 outside the two-day pause", result.GeneratedSchedules)
	}
	rows, err := pool.Query(ctx, `SELECT occurrence_date::text FROM schedules WHERE blueprint_id=$1 ORDER BY occurrence_date`, blueprint.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var dates []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			t.Fatal(err)
		}
		dates = append(dates, date)
	}
	want := []string{startDate.Format("2006-01-02"), startDate.AddDate(0, 0, 3).Format("2006-01-02"), startDate.AddDate(0, 0, 4).Format("2006-01-02"), startDate.AddDate(0, 0, 5).Format("2006-01-02"), startDate.AddDate(0, 0, 6).Format("2006-01-02")}
	if !reflect.DeepEqual(dates, want) {
		t.Fatalf("backfilled dates=%v, want %v (same original daily cadence)", dates, want)
	}
}

func TestProjectSchedulePermissionsAndProjectOwnerAssigneeDefault(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	ownerID := createScheduleTestUser(t, pool)
	editorID := createScheduleTestUser(t, pool)
	viewerID := createScheduleTestUser(t, pool)
	deleterID := createScheduleTestUser(t, pool)
	projectID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other','Schedule ACL project','low','open',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	roleID := "schedule_delete_only"
	if _, err := pool.Exec(ctx, `INSERT INTO roles(role_id,name) VALUES($1,'Schedule delete only')`, roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,permission_id FROM permissions WHERE resource_id='schedule' AND action='delete' AND effect='allow'`, roleID); err != nil {
		t.Fatal(err)
	}
	for _, member := range []struct{ id, role string }{{editorID, "editor"}, {viewerID, "viewer"}, {deleterID, roleID}} {
		if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, projectID, member.id, member.role, ownerID); err != nil {
			t.Fatalf("add %s membership: %v", member.role, err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE role_id=$1`, roleID)
	})
	uow := blueprintTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 6)
	start := startDate.Add(9 * time.Hour)
	create := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now })
	input := usecase.CreateBlueprintInput{Title: "Shared recurrence", ProjectID: projectID, Timezone: "Asia/Tokyo",
		LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1}
	blueprint, err := create.Execute(ctx, editorID, input)
	if err != nil {
		t.Fatalf("editor create blueprint: %v", err)
	}
	if blueprint.UserID != ownerID || blueprint.AssigneeID != ownerID {
		t.Fatalf("project-created blueprint identity/assignee = owner %s, assignee %s; want both project owner %s", blueprint.UserID, blueprint.AssigneeID, ownerID)
	}
	if _, err := create.Execute(ctx, viewerID, input); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("viewer create error=%v, want ErrPermissionDenied", err)
	}
	personalStart := start.Add(time.Hour)
	personal, err := create.Execute(ctx, editorID, usecase.CreateBlueprintInput{Title: "Editor personal schedule", Timezone: "Asia/Tokyo",
		LocalStart: personalStart, LocalEnd: personalStart.Add(time.Hour), Frequency: "daily", Interval: 1})
	if err != nil {
		t.Fatalf("create editor personal recurrence: %v", err)
	}
	from := startDate.UTC()
	list := usecase.NewListOccurrencesUseCase(uow)
	personalView, err := list.ExecutePage(ctx, viewerID, usecase.CursorPageRequest{Size: 20, From: from, To: from.AddDate(0, 0, 3)})
	if err != nil {
		t.Fatalf("list viewer's personal schedules: %v", err)
	}
	for _, occurrence := range personalView.Items {
		if occurrence.BlueprintID == blueprint.ID || occurrence.BlueprintID == personal.ID {
			t.Fatalf("personal /schedules view leaked another member's occurrence: %+v", occurrence)
		}
	}
	projectView, err := list.ExecutePage(ctx, viewerID, usecase.CursorPageRequest{Size: 20, From: from, To: from.AddDate(0, 0, 3), ProjectID: projectID})
	if err != nil {
		t.Fatalf("list viewer-readable Project schedules: %v", err)
	}
	foundProjectSchedule, foundPersonalSchedule := false, false
	for _, occurrence := range projectView.Items {
		foundProjectSchedule = foundProjectSchedule || occurrence.BlueprintID == blueprint.ID
		foundPersonalSchedule = foundPersonalSchedule || occurrence.BlueprintID == personal.ID
	}
	if !foundProjectSchedule || foundPersonalSchedule {
		t.Fatalf("Project-scoped view must include the shared Project Schedule and hide the editor's personal Schedule: %+v", projectView.Items)
	}
	get := usecase.NewGetBlueprintUseCase(uow)
	if _, err := get.Execute(ctx, editorID, blueprint.ID); err != nil {
		t.Fatalf("editor read blueprint: %v", err)
	}
	updatedTitle := "Editor updated recurrence"
	update := usecase.NewUpdateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
	current, err := update.Execute(ctx, editorID, blueprint.ID, startDate.Format("2006-01-02"), "following",
		usecase.UpdateBlueprintInput{Title: &updatedTitle}, blueprint.Revision)
	if err != nil {
		t.Fatalf("editor update blueprint: %v", err)
	}
	deleteFollowing := usecase.NewDeleteBlueprintFollowingUseCase(uow, blueprintTestIDs{}, func() time.Time { return now })
	cutoffDate := startDate.AddDate(0, 0, 2).Format("2006-01-02")
	if err := deleteFollowing.Execute(ctx, editorID, blueprint.ID, cutoffDate, current.Revision); err == nil {
		t.Fatal("editor deleted following occurrences without schedule:delete")
	}
	if err := deleteFollowing.Execute(ctx, deleterID, blueprint.ID, cutoffDate, current.Revision); err != nil {
		t.Fatalf("delete-only role could not delete following occurrences: %v", err)
	}

}

func TestCreatingRecurrenceInDoneProjectOpensAutomaticPauseInterval(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	projectID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other','Done schedule project','low','done',$2)`, projectID, userID); err != nil {
		t.Fatalf("create completed project: %v", err)
	}
	uow := blueprintTestUOW{pool: pool, lifecycle: blueprintProjectLifecycle{status: "done"}}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, 1).Add(9 * time.Hour)
	blueprint, err := usecase.NewCreateBlueprintUseCase(uow, blueprintTestIDs{}, blueprintTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateBlueprintInput{
		Title: "Paused after completion", ProjectID: projectID, Timezone: "Asia/Tokyo", LocalStart: start,
		LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurrence in done project: %v", err)
	}
	var pausedAt time.Time
	var resumedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT paused_at,resumed_at FROM schedule_project_pause_intervals WHERE project_id=$1 AND blueprint_id=$2`, projectID, blueprint.ID).Scan(&pausedAt, &resumedAt); err != nil {
		t.Fatalf("read automatic pause interval: %v", err)
	}
	if pausedAt.IsZero() || resumedAt != nil {
		t.Fatalf("automatic pause = %s resumed=%v; want open interval", pausedAt, resumedAt)
	}
	from := today.AddDate(0, 0, 1).UTC()
	page, err := usecase.NewListOccurrencesUseCase(uow).ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 10, From: from, To: from.AddDate(0, 0, 3)})
	if err != nil {
		t.Fatalf("list automatically paused recurrence: %v", err)
	}
	for _, occurrence := range page.Items {
		if occurrence.BlueprintID == blueprint.ID {
			t.Fatalf("done Project recurrence was not paused: %+v", occurrence)
		}
	}
}

func createScheduleTestUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := ulid.Make().String()
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Integration',$2,'unused','Asia/Tokyo')`, id, id+"@schedule.test"); err != nil {
		t.Fatalf("insert schedule integration user: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM schedule_revisions WHERE schedule_id IN (SELECT id FROM schedules WHERE user_id=$1)`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM schedule_backfill_checkpoints WHERE revision_id IN (SELECT id FROM schedule_blueprint_revisions WHERE user_id=$1)`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM schedule_project_pause_intervals WHERE blueprint_id IN (SELECT id FROM schedule_blueprints WHERE user_id=$1)`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM schedules WHERE user_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM schedule_blueprint_revisions WHERE user_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM schedule_blueprints WHERE user_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM project_revisions WHERE user_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE user_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	})
	return id
}

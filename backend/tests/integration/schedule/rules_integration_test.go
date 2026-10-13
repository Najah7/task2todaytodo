//go:build integration

package schedule_test

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	schedulerepo "github.com/Najah7/task2todaytodo/internal/application/schedule/repository"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type ruleTestUOW struct {
	pool      *pgxpool.Pool
	lifecycle shared.ProjectWorkLifecycle
}

type ruleTestRepositories struct {
	schedules *schedulerepo.ScheduleRepository
	lifecycle shared.ProjectWorkLifecycle
}

func (r ruleTestRepositories) Schedules() usecase.ScheduleRepository { return r.schedules }
func (r ruleTestRepositories) Rules() usecase.RuleRepository         { return r.schedules }

func (u ruleTestUOW) Do(ctx context.Context, fn func(context.Context, usecase.Repositories) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	lifecycle := u.lifecycle
	if lifecycle == nil {
		lifecycle = ruleProjectLifecycle{}
	}
	repositories := ruleTestRepositories{schedules: schedulerepo.NewScheduleRepository(tx), lifecycle: lifecycle}
	if err := fn(ctx, repositories); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (u ruleTestUOW) DoWithTodoImpact(ctx context.Context, _ usecase.TodoImpactScope, fn func(context.Context, usecase.Repositories) error) error {
	return u.Do(ctx, fn)
}

func (ruleProjectLifecycle) LockParent(context.Context, string) error { return nil }

type ruleProjectLifecycle struct{ status string }

func (l ruleProjectLifecycle) ReadStatus(context.Context, string) (string, error) {
	if l.status != "" {
		return l.status, nil
	}
	return "open", nil
}
func (ruleProjectLifecycle) CaptureWorkState(context.Context, string, time.Time) (shared.ProjectWorkState, error) {
	return shared.ProjectWorkState{Status: "open"}, nil
}
func (ruleProjectLifecycle) ReconcileWorkState(context.Context, string, string, shared.ProjectWorkState, time.Time) error {
	return nil
}

func (r ruleTestRepositories) ProjectLifecycle() shared.ProjectWorkLifecycle { return r.lifecycle }

type ruleTestIDs struct{}

func (ruleTestIDs) Generate() string { return ulid.Make().String() }

type ruleTimezoneValidator struct{ queries *sqlc.Queries }

func (v ruleTimezoneValidator) TimezoneExists(ctx context.Context, zone string) (bool, error) {
	return v.queries.IsTimezoneAllowed(ctx, zone)
}

func TestRuleCreateAndVirtualListDoesNotPersistRecurringRows(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	ids := (ruleTestIDs{}).Generate
	now := time.Now().UTC()
	localToday, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(localToday)
	weekStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	weekStart = weekStart.AddDate(0, 0, -((int(weekStart.Weekday()) + 6) % 7))
	clock := func() time.Time { return now }
	create := usecase.NewCreateRuleUseCase(uow, ids, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, clock)

	onceStart := weekStart.AddDate(0, 0, 1).Add(11 * time.Hour)
	once, err := create.Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "One-off planning session", Timezone: "Asia/Tokyo", LocalStart: onceStart,
		LocalEnd: onceStart.Add(time.Hour), Frequency: "once",
	})
	if err != nil {
		t.Fatalf("create one-off rule: %v", err)
	}
	recurringStart := weekStart.Add(9 * time.Hour)
	recurring, err := create.Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Daily focus block", Timezone: "Asia/Tokyo", LocalStart: recurringStart,
		LocalEnd: recurringStart.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurring rule: %v", err)
	}
	var onceRows, recurringRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedules WHERE rule_id=$1`, once.ID).Scan(&onceRows); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedules WHERE rule_id=$1`, recurring.ID).Scan(&recurringRows); err != nil {
		t.Fatal(err)
	}
	if onceRows != 1 || recurringRows != 0 {
		t.Fatalf("concrete rows: one-off=%d recurring=%d; want 1 and 0", onceRows, recurringRows)
	}
	var concreteScheduleID string
	if err := pool.QueryRow(ctx, `SELECT id FROM schedules WHERE rule_id=$1`, once.ID).Scan(&concreteScheduleID); err != nil {
		t.Fatalf("read concrete one-off schedule: %v", err)
	}
	todoID := ulid.Make().String()
	occurrenceDate := onceStart.Format("2006-01-02")
	if _, err := pool.Exec(ctx, `INSERT INTO todos(id,user_id,todo_date,timezone) VALUES($1,$2,$3,'Asia/Tokyo')`, todoID, userID, occurrenceDate); err != nil {
		t.Fatalf("create Todo: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO todo_items(id,todo_id,kind,schedule_rule_id,occurrence_date,scheduled_start,scheduled_end,position) VALUES($1,$2,'schedule',$3,$4,$5,$6,0)`, ulid.Make().String(), todoID, once.ID, occurrenceDate, onceStart.UTC(), onceStart.Add(time.Hour).UTC()); err != nil {
		t.Fatalf("link recurring identity into Todo: %v", err)
	}
	var linkedRuleID string
	if err := pool.QueryRow(ctx, `SELECT schedule_rule_id FROM todo_items WHERE todo_id=$1 AND occurrence_date=$2`, todoID, occurrenceDate).Scan(&linkedRuleID); err != nil || linkedRuleID != once.ID {
		t.Fatalf("Todo Schedule link=%q, err=%v; want stable rule %s", linkedRuleID, err, once.ID)
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
		if item.RuleID == recurring.ID && item.ID == nil {
			virtualDaily++
		}
	}
	if virtualDaily != 4 {
		t.Fatalf("virtual daily occurrences=%d; want 4", virtualDaily)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedules WHERE rule_id=$1`, recurring.ID).Scan(&recurringRows); err != nil {
		t.Fatal(err)
	}
	if recurringRows != 0 {
		t.Fatalf("read-only list persisted %d recurring rows", recurringRows)
	}
}

func TestScheduleTodoImpactUsersUsesProjectOwnerWhenAssigneeOmitted(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	ownerID := createScheduleTestUser(t, pool)
	projectID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other','Schedule impact owner','low','open',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("create Project: %v", err)
	}
	users, err := sqlc.New(pool).ListScheduleTodoImpactUsers(ctx, sqlc.ListScheduleTodoImpactUsersParams{
		RuleID: ulid.Make().String(), OccurrenceDate: pgtype.Date{Time: time.Now().UTC(), Valid: true},
		AssigneeID: pgtype.Text{}, ActorID: ownerID, ProjectID: pgtype.Text{String: projectID, Valid: true},
	})
	if err != nil {
		t.Fatalf("resolve impact users: %v", err)
	}
	if !reflect.DeepEqual(users, []string{ownerID}) {
		t.Fatalf("impact users=%v, want Project owner %q", users, ownerID)
	}
}

func TestScheduleTodoImpactUsersIncludesExceptionAssignee(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	ownerID := createScheduleTestUser(t, pool)
	oldAssigneeID := createScheduleTestUser(t, pool)
	exceptionAssigneeID := createScheduleTestUser(t, pool)
	projectID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other','Schedule exception impact','low','open',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("create Project: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3),($1,$4,'editor',$3)`, projectID, oldAssigneeID, ownerID, exceptionAssigneeID); err != nil {
		t.Fatalf("add Schedule assignees: %v", err)
	}
	uow := ruleTestUOW{pool: pool}
	now := time.Now()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(location)
	localDate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	start := localDate.Add(10 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, ownerID, usecase.CreateRuleInput{
		Title: "Schedule with exception assignee", ProjectID: projectID, AssigneeID: oldAssigneeID,
		Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "once",
	})
	if err != nil {
		t.Fatalf("create Schedule rule: %v", err)
	}
	occurrenceDate := start.Format("2006-01-02")
	movedStart := start.Add(2 * time.Hour)
	if _, err := usecase.NewRescheduleOccurrenceUseCase(uow, nil, (ruleTestIDs{}).Generate).Execute(ctx, ownerID, rule.ID, occurrenceDate, movedStart, movedStart.Add(time.Hour), 1); err != nil {
		t.Fatalf("create occurrence exception: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE schedule_exceptions SET assignee_id=$1 WHERE rule_id=$2 AND occurrence_date=$3`, exceptionAssigneeID, rule.ID, occurrenceDate); err != nil {
		t.Fatalf("set exception assignee: %v", err)
	}
	date, err := time.Parse("2006-01-02", occurrenceDate)
	if err != nil {
		t.Fatal(err)
	}
	users, err := sqlc.New(pool).ListScheduleTodoImpactUsers(ctx, sqlc.ListScheduleTodoImpactUsersParams{
		RuleID: rule.ID, OccurrenceDate: pgtype.Date{Time: date, Valid: true}, Following: false,
		AssigneeID: pgtype.Text{}, ActorID: ownerID, ProjectID: pgtype.Text{String: projectID, Valid: true},
	})
	if err != nil {
		t.Fatalf("resolve Schedule impact users: %v", err)
	}
	want := []string{ownerID, oldAssigneeID, exceptionAssigneeID}
	sort.Strings(want)
	if !reflect.DeepEqual(users, want) {
		t.Fatalf("Schedule impact users=%v, want owner, rule assignee, and exception assignee %v", users, want)
	}
}

func TestMovedOccurrenceKeepsOriginalIdentityAndAppearsOnlyAtActualTime(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(location)
	localDate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Daily stand-up", Timezone: "Asia/Tokyo", LocalStart: localDate.Add(9 * time.Hour),
		LocalEnd: localDate.Add(10 * time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurring rule: %v", err)
	}
	originalDate := localDate.Format("2006-01-02")
	movedStart := time.Date(localDate.Year(), localDate.Month(), localDate.Day()+1, 11, 0, 0, 0, location)
	movedEnd := movedStart.Add(time.Hour)
	moved, err := usecase.NewRescheduleOccurrenceUseCase(uow, nil, (ruleTestIDs{}).Generate).Execute(ctx, userID, rule.ID, originalDate, movedStart, movedEnd, 0)
	if err != nil {
		t.Fatalf("move occurrence: %v", err)
	}
	if moved.RuleID != rule.ID || moved.OccurrenceDate != originalDate || moved.ID == nil || moved.StartAt != movedStart.Unix() {
		t.Fatalf("moved occurrence lost its original identity: %+v", moved)
	}
	var fullExceptionSnapshot bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM schedule_exceptions
		WHERE rule_id=$1 AND occurrence_date=$2 AND start_at=$3 AND end_at=$4
		  AND title='Daily stand-up' AND timezone='Asia/Tokyo' AND NOT all_day
		  AND NOT completed AND skipped_at IS NULL AND deleted_at IS NULL
	)`, rule.ID, originalDate, movedStart.UTC(), movedEnd.UTC()).Scan(&fullExceptionSnapshot); err != nil {
		t.Fatalf("read full saved exception snapshot: %v", err)
	}
	if !fullExceptionSnapshot {
		t.Fatal("individual move did not persist full exception snapshot")
	}
	if _, err := usecase.NewRescheduleOccurrenceUseCase(uow, nil, (ruleTestIDs{}).Generate).Execute(ctx, userID, rule.ID,
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
		if item.RuleID == rule.ID && item.OccurrenceDate == originalDate {
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
		if item.RuleID == rule.ID && item.OccurrenceDate == originalDate && item.StartAt == movedStart.Unix() {
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
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := now.In(location)
	localDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	startDate := localDate.AddDate(0, 0, -3)
	create := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now })
	makeDaily := func(title string, hour time.Duration) usecase.CreateRuleInput {
		start := startDate.Add(hour)
		return usecase.CreateRuleInput{Title: title, Timezone: "Asia/Tokyo", LocalStart: start,
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
	_, err = usecase.NewRescheduleOccurrenceUseCase(uow, nil, (ruleTestIDs{}).Generate).Execute(ctx, userID, before.ID, overrideDate, movedStart, movedStart.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("save override before backfill: %v", err)
	}
	backfill := usecase.NewBackfillUseCase(uow, (ruleTestIDs{}).Generate)
	cutoff := time.Date(today.Year(), today.Month(), today.Day(), 12, 0, 0, 0, location).UTC()
	if _, _, err := backfill.Execute(ctx, userID, cutoff, nil); err != nil {
		t.Fatalf("backfill at noon cutoff: %v", err)
	}
	todayKey := localDate.Format("2006-01-02")
	countOnDate := func(ruleID string) int {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedules WHERE rule_id=$1 AND occurrence_date=$2`, ruleID, todayKey).Scan(&count); err != nil {
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
	if _, _, err := backfill.Execute(ctx, userID, laterCutoff, nil); err != nil {
		t.Fatalf("backfill later on same local day: %v", err)
	}
	if countOnDate(equal.ID) != 1 || countOnDate(future.ID) != 1 {
		t.Fatalf("later cutoff rows: equal=%d future=%d; want 1 and 1", countOnDate(equal.ID), countOnDate(future.ID))
	}
	var persistedStart time.Time
	if err := pool.QueryRow(ctx, `SELECT start_at FROM schedules WHERE rule_id=$1 AND occurrence_date=$2`, before.ID, overrideDate).Scan(&persistedStart); err != nil {
		t.Fatalf("read saved override: %v", err)
	}
	if !persistedStart.Equal(movedStart.UTC()) {
		t.Fatalf("backfill overwrote saved override start: got %s, want %s", persistedStart, movedStart.UTC())
	}
	var exceptions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedule_exceptions WHERE rule_id=$1 AND occurrence_date=$2`, before.ID, overrideDate).Scan(&exceptions); err != nil {
		t.Fatalf("count saved exception after backfill retry: %v", err)
	}
	if exceptions != 1 {
		t.Fatalf("backfill exception count=%d; want 1", exceptions)
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
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 14)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Recurring review", ProjectID: projectID, Timezone: "Asia/Tokyo", LocalStart: startDate.Add(9 * time.Hour),
		LocalEnd: startDate.Add(10 * time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurring rule: %v", err)
	}
	deletedFrom := startDate.AddDate(0, 0, 3)
	savedDate := startDate.AddDate(0, 0, 5)
	movedStart := time.Date(savedDate.Year(), savedDate.Month(), savedDate.Day()+2, 16, 0, 0, 0, location)
	_, err = usecase.NewRescheduleOccurrenceUseCase(uow, nil, (ruleTestIDs{}).Generate).Execute(ctx, userID, rule.ID,
		savedDate.Format("2006-01-02"), movedStart, movedStart.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("save tail override: %v", err)
	}
	if _, err := usecase.NewGetOccurrenceUseCase(uow).Execute(ctx, viewerID, rule.ID, savedDate.Format("2006-01-02")); err != nil {
		t.Fatalf("Project viewer cannot read saved occurrence before deletion: %v", err)
	}
	deleteFollowing := usecase.NewDeleteRuleFollowingUseCase(uow, (ruleTestIDs{}).Generate, func() time.Time { return now })
	if err := deleteFollowing.Execute(ctx, userID, rule.ID, deletedFrom.Format("2006-01-02"), rule.Revision); err != nil {
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
		if occurrence.RuleID == rule.ID && occurrence.OccurrenceDate >= deletedFrom.Format("2006-01-02") {
			t.Fatalf("deleted tail was projected or saved override resurfaced: %+v", occurrence)
		}
	}
	var deletedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT deleted_at FROM schedules WHERE rule_id=$1 AND occurrence_date=$2`, rule.ID, savedDate.Format("2006-01-02")).Scan(&deletedAt); err != nil {
		t.Fatalf("read deleted tail override: %v", err)
	}
	if deletedAt == nil {
		t.Fatal("saved override in deleted tail was not tombstoned")
	}
	var exceptionDeletedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT deleted_at FROM schedule_exceptions WHERE rule_id=$1 AND occurrence_date=$2`, rule.ID, savedDate.Format("2006-01-02")).Scan(&exceptionDeletedAt); err != nil {
		t.Fatalf("read cancelled exception: %v", err)
	}
	if exceptionDeletedAt == nil {
		t.Fatal("following deletion did not cancel saved exception")
	}
	history := usecase.NewListOccurrenceRevisionsUseCase(uow)
	ownerHistory, err := history.ExecutePage(ctx, userID, rule.ID, savedDate.Format("2006-01-02"), 20, 0)
	if err != nil || len(ownerHistory.Items) == 0 {
		t.Fatalf("owner could not read cancelled occurrence history: items=%v err=%v", ownerHistory.Items, err)
	}
	if _, err := history.ExecutePage(ctx, viewerID, rule.ID, savedDate.Format("2006-01-02"), 20, 0); err == nil {
		t.Fatal("Project viewer could read cancelled occurrence audit history")
	}
}

func TestDeleteFollowingAdvancesAggregateRevisionAndInvalidatesStaleEditor(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 10)
	start := startDate.Add(9 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Deletion revision", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurrence: %v", err)
	}
	deletedFrom := startDate.AddDate(0, 0, 3)
	if err := usecase.NewDeleteRuleFollowingUseCase(uow, (ruleTestIDs{}).Generate, func() time.Time { return now }).Execute(ctx, userID, rule.ID, deletedFrom.Format("2006-01-02"), rule.Revision); err != nil {
		t.Fatalf("delete future tail: %v", err)
	}
	current, err := usecase.NewGetRuleUseCase(uow).Execute(ctx, userID, rule.ID)
	if err != nil {
		t.Fatalf("get rule after deletion: %v", err)
	}
	if current.Revision <= rule.Revision {
		t.Fatalf("aggregate revision after deletion=%d, want greater than initial %d", current.Revision, rule.Revision)
	}
	staleTitle := "Stale earlier edit"
	if _, err := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now }).Execute(
		ctx, userID, rule.ID, startDate.AddDate(0, 0, 1).Format("2006-01-02"), "following",
		usecase.UpdateRuleInput{Title: &staleTitle}, rule.Revision,
	); !errors.Is(err, usecase.ErrOccurrenceRevisionMismatch) {
		t.Fatalf("earlier edit with stale pre-deletion revision error=%v, want ErrOccurrenceRevisionMismatch", err)
	}
	updatedTitle := "Earlier edit after cancellation"
	if _, err := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now }).Execute(
		ctx, userID, rule.ID, startDate.AddDate(0, 0, 1).Format("2006-01-02"), "following",
		usecase.UpdateRuleInput{Title: &updatedTitle}, current.Revision,
	); err != nil {
		t.Fatalf("earlier edit with fresh post-deletion revision: %v", err)
	}
	from := deletedFrom.UTC()
	page, err := usecase.NewListOccurrencesUseCase(uow).ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 20, From: from, To: from.AddDate(0, 0, 7)})
	if err != nil {
		t.Fatalf("list after earlier edit following cancellation: %v", err)
	}
	for _, occurrence := range page.Items {
		if occurrence.RuleID == rule.ID && occurrence.OccurrenceDate >= deletedFrom.Format("2006-01-02") {
			t.Fatalf("earlier edit revived cancelled tail: %+v", occurrence)
		}
	}
}

func TestSkipRetryIsIdempotentAndRestoreOnlyUnskips(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, 2).Add(9 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Daily task", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurring rule: %v", err)
	}
	date := start.Format("2006-01-02")
	get := usecase.NewGetOccurrenceUseCase(uow)
	skip := usecase.NewSetOccurrenceStateUseCase(uow, "skip", nil, (ruleTestIDs{}).Generate)
	if err := skip.Execute(ctx, userID, rule.ID, date, 0); err != nil {
		t.Fatalf("skip virtual occurrence: %v", err)
	}
	var skippedExceptionCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedule_exceptions WHERE rule_id=$1 AND occurrence_date=$2 AND skipped_at IS NOT NULL`, rule.ID, date).Scan(&skippedExceptionCount); err != nil {
		t.Fatalf("read skipped exception: %v", err)
	}
	if skippedExceptionCount != 1 {
		t.Fatal("skip did not persist exception state")
	}
	skipped, err := get.Execute(ctx, userID, rule.ID, date)
	if err != nil || !skipped.Skipped || skipped.ID == nil {
		t.Fatalf("read skipped occurrence = %+v, %v", skipped, err)
	}
	secondDate := start.AddDate(0, 0, 1).Format("2006-01-02")
	if err := skip.Execute(ctx, userID, rule.ID, secondDate, 0); err != nil {
		t.Fatalf("skip second virtual occurrence: %v", err)
	}
	secondSkipped, err := get.Execute(ctx, userID, rule.ID, secondDate)
	if err != nil || !secondSkipped.Skipped || secondSkipped.ID == nil {
		t.Fatalf("read second skipped occurrence = %+v, %v", secondSkipped, err)
	}
	list := usecase.NewListOccurrencesUseCase(uow)
	from := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC).Add(-time.Hour)
	defaultPage, err := list.ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 10, From: from, To: from.Add(72 * time.Hour)})
	if err != nil {
		t.Fatalf("list default skipped visibility: %v", err)
	}
	for _, item := range defaultPage.Items {
		if item.RuleID == rule.ID && (item.OccurrenceDate == date || item.OccurrenceDate == secondDate) {
			t.Fatalf("default list included skipped occurrence %+v", item)
		}
	}
	includeSkippedPage, err := list.ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 10, From: from, To: from.Add(72 * time.Hour), IncludeSkipped: true})
	if err != nil {
		t.Fatalf("list with skipped occurrences: %v", err)
	}
	var listedSkipped int
	for _, item := range includeSkippedPage.Items {
		if item.RuleID == rule.ID && item.Skipped {
			listedSkipped++
		}
	}
	if listedSkipped != 2 {
		t.Fatalf("include_skipped listed %d skipped rows, want 2", listedSkipped)
	}
	if *secondSkipped.ID == *skipped.ID {
		t.Fatalf("distinct skipped occurrences share ID %q", *skipped.ID)
	}
	for _, id := range []string{*skipped.ID, *secondSkipped.ID} {
		if _, err := ulid.Parse(id); err != nil {
			t.Errorf("occurrence ID %q is not a ULID: %v", id, err)
		}
	}
	if err := skip.Execute(ctx, userID, rule.ID, date, skipped.Revision); err != nil {
		t.Fatalf("retry skip with current revision: %v", err)
	}
	stillSkipped, err := get.Execute(ctx, userID, rule.ID, date)
	if err != nil || !stillSkipped.Skipped || stillSkipped.Revision != skipped.Revision {
		t.Fatalf("retry changed skipped occurrence: %+v, %v", stillSkipped, err)
	}
	restore := usecase.NewSetOccurrenceStateUseCase(uow, "restore", nil, (ruleTestIDs{}).Generate)
	if err := restore.Execute(ctx, userID, rule.ID, date, stillSkipped.Revision); err != nil {
		t.Fatalf("restore skipped occurrence: %v", err)
	}
	restored, err := get.Execute(ctx, userID, rule.ID, date)
	if err != nil || restored.Skipped || restored.Revision == stillSkipped.Revision {
		t.Fatalf("read restored occurrence = %+v, %v", restored, err)
	}
	restoredPage, err := list.ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 10, From: from, To: from.Add(72 * time.Hour), IncludeSkipped: true})
	if err != nil {
		t.Fatalf("list restored occurrence: %v", err)
	}
	foundRestored, foundStillSkipped := false, false
	for _, item := range restoredPage.Items {
		if item.RuleID == rule.ID && item.OccurrenceDate == date {
			foundRestored = !item.Skipped
		}
		if item.RuleID == rule.ID && item.OccurrenceDate == secondDate {
			foundStillSkipped = item.Skipped
		}
	}
	if !foundRestored || !foundStillSkipped {
		t.Fatalf("restore changed more than the selected skip state: %+v", restoredPage.Items)
	}
	delete := usecase.NewSetOccurrenceStateUseCase(uow, "delete", nil, (ruleTestIDs{}).Generate)
	if err := delete.Execute(ctx, userID, rule.ID, date, restored.Revision); err != nil {
		t.Fatalf("delete occurrence: %v", err)
	}
	var deletedRevision int32
	var deletedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT revision, deleted_at FROM schedules WHERE rule_id=$1 AND occurrence_date=$2`, rule.ID, date).Scan(&deletedRevision, &deletedAt); err != nil {
		t.Fatalf("read deleted occurrence row: %v", err)
	}
	if err := restore.Execute(ctx, userID, rule.ID, date, deletedRevision); err == nil {
		t.Fatal("restore cleared a deletion tombstone; deleted occurrences are only canceled by the requested API")
	}
	deletedPage, err := list.ExecutePage(ctx, userID, usecase.CursorPageRequest{Size: 10, From: from, To: from.Add(72 * time.Hour), IncludeSkipped: true})
	if err != nil {
		t.Fatalf("list after occurrence deletion: %v", err)
	}
	for _, item := range deletedPage.Items {
		if item.RuleID == rule.ID && item.OccurrenceDate == date {
			t.Fatalf("deleted occurrence remained visible in calendar list: %+v", item)
		}
	}
}

func TestAllDayPersistsLocalCalendarSpanAndOccurrenceRevision(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	ids := (ruleTestIDs{}).Generate
	create := usecase.NewCreateRuleUseCase(uow, ids, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil)
	localStart := time.Date(2026, time.March, 8, 0, 0, 0, 0, time.UTC)
	localEnd := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
	rule, err := create.Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "DST all-day span", Timezone: "America/New_York", LocalStart: localStart, LocalEnd: localEnd,
		AllDay: true, Frequency: "once",
	})
	if err != nil {
		t.Fatalf("create all-day rule: %v", err)
	}
	if !rule.AllDay {
		t.Fatalf("created rule all_day=%v, want true", rule.AllDay)
	}
	var startAt, endAt time.Time
	var allDay bool
	if err := pool.QueryRow(ctx, `SELECT start_at,end_at,all_day FROM schedules WHERE rule_id=$1`, rule.ID).Scan(&startAt, &endAt, &allDay); err != nil {
		t.Fatalf("read persisted all-day occurrence: %v", err)
	}
	if !allDay || endAt.Sub(startAt) != 47*time.Hour {
		t.Fatalf("persisted all-day interval %s..%s all_day=%v, want 47h across DST", startAt, endAt, allDay)
	}
	var revisionAllDay bool
	if err := pool.QueryRow(ctx, `SELECT all_day FROM schedule_revisions WHERE schedule_id=(SELECT id FROM schedules WHERE rule_id=$1) AND revision=1`, rule.ID).Scan(&revisionAllDay); err != nil {
		t.Fatalf("read all-day audit snapshot: %v", err)
	}
	if !revisionAllDay {
		t.Fatal("initial occurrence revision did not snapshot all_day=true")
	}
}

func TestAllDayCanToggleForOccurrenceAndFollowingRevision(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	startDate := time.Date(2026, time.March, 8, 0, 0, 0, 0, time.UTC)
	start := startDate.Add(9 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Toggle all-day", Timezone: "America/New_York", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create timed schedule: %v", err)
	}
	date := start.Format("2006-01-02")
	toggle := true
	updated, err := usecase.NewUpdateOccurrenceUseCase(uow, nil, (ruleTestIDs{}).Generate).Execute(ctx, userID, rule.ID, date,
		usecase.UpdateScheduleInput{AllDay: &toggle}, 0)
	if err != nil {
		t.Fatalf("toggle virtual occurrence to all-day: %v", err)
	}
	if !updated.AllDay || time.Unix(updated.StartAt, 0).In(location).Format("15:04") != "00:00" ||
		time.Unix(updated.EndAt, 0).Sub(time.Unix(updated.StartAt, 0)) != 23*time.Hour {
		t.Fatalf("all-day occurrence after toggle=%+v", updated)
	}
	var snapshotAllDay bool
	if err := pool.QueryRow(ctx, `SELECT all_day FROM schedule_revisions WHERE schedule_id=$1 AND revision=$2`, *updated.ID, updated.Revision).Scan(&snapshotAllDay); err != nil || !snapshotAllDay {
		t.Fatalf("all-day toggle audit snapshot=%v err=%v", snapshotAllDay, err)
	}

	allDay := true
	localToday := now.In(location)
	followingDate := time.Date(localToday.Year(), localToday.Month(), localToday.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 3).Format("2006-01-02")
	update := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
	allDayRevision, err := update.Execute(ctx, userID, rule.ID, followingDate, "following", usecase.UpdateRuleInput{AllDay: &allDay}, rule.Revision)
	if err != nil || !allDayRevision.AllDay {
		t.Fatalf("following all-day revision=%+v err=%v", allDayRevision, err)
	}
	timed := false
	timedRevision, err := update.Execute(ctx, userID, rule.ID, followingDate, "following", usecase.UpdateRuleInput{AllDay: &timed}, allDayRevision.Revision)
	if err != nil || timedRevision.AllDay {
		t.Fatalf("following timed revision=%+v err=%v", timedRevision, err)
	}
	var auditRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedule_rule_revisions WHERE rule_id=$1 AND effective_from=$2 AND all_day`, rule.ID, followingDate).Scan(&auditRows); err != nil {
		t.Fatal(err)
	}
	if auditRows != 1 {
		t.Fatalf("all-day following revision audit rows=%d, want one", auditRows)
	}
}

func TestFollowingAllDayRejectsExplicitNonMidnightStart(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 5)
	start := startDate.Add(9 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate,
		ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID,
		usecase.CreateRuleInput{Title: "Timed recurrence", Timezone: "Asia/Tokyo", LocalStart: start,
			LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1})
	if err != nil {
		t.Fatalf("create timed recurrence: %v", err)
	}
	allDay := true
	explicitStart := startDate.Add(10 * time.Hour)
	update := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate,
		ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
	if _, err := update.Execute(ctx, userID, rule.ID, startDate.Format("2006-01-02"), "following",
		usecase.UpdateRuleInput{AllDay: &allDay, LocalStart: &explicitStart}, rule.Revision); err != domain.ErrRuleAllDayTime {
		t.Fatalf("explicit non-midnight all-day start error=%v, want %v", err, domain.ErrRuleAllDayTime)
	}
	if _, err := update.Execute(ctx, userID, rule.ID, startDate.Format("2006-01-02"), "following",
		usecase.UpdateRuleInput{AllDay: &allDay}, rule.Revision); err != nil {
		t.Fatalf("all-day toggle with omitted start should normalize to midnight: %v", err)
	}
}

func TestFollowingAllDayIncludesDatesTouchedByOvernightEnd(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 5)
	localStart := startDate.Add(22 * time.Hour)
	localEnd := startDate.AddDate(0, 0, 1).Add(2 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate,
		ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID,
		usecase.CreateRuleInput{Title: "Overnight timed recurrence", Timezone: "Asia/Tokyo", LocalStart: localStart,
			LocalEnd: localEnd, Frequency: "daily", Interval: 1})
	if err != nil {
		t.Fatalf("create overnight recurrence: %v", err)
	}
	allDay := true
	updated, err := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate,
		ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now }).Execute(ctx, userID,
		rule.ID, startDate.Format("2006-01-02"), "following", usecase.UpdateRuleInput{AllDay: &allDay}, rule.Revision)
	if err != nil {
		t.Fatalf("toggle overnight recurrence to all-day: %v", err)
	}
	if want := startDate.AddDate(0, 0, 2).Format("2006-01-02 15:04:05"); updated.LocalEnd != want {
		t.Fatalf("all-day exclusive end = %q, want %q to include the overnight end date", updated.LocalEnd, want)
	}
	occurrence, err := usecase.NewGetOccurrenceUseCase(uow).Execute(ctx, userID, rule.ID, startDate.Format("2006-01-02"))
	if err != nil {
		t.Fatalf("get all-day occurrence: %v", err)
	}
	if !occurrence.AllDay || occurrence.EndAt-occurrence.StartAt != int64(48*time.Hour/time.Second) {
		t.Fatalf("all-day overnight occurrence = %+v; want 48h across both local dates", occurrence)
	}
}

func TestAllDayBackfillCopiesRevisionAndMaterializesLocalBoundaries(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, -3)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Backfilled all-day", Timezone: "UTC", LocalStart: start, LocalEnd: start.AddDate(0, 0, 2),
		AllDay: true, Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurring all-day definition: %v", err)
	}
	if _, _, err := usecase.NewBackfillUseCase(uow, (ruleTestIDs{}).Generate).Execute(ctx, userID, now, nil); err != nil {
		t.Fatalf("backfill all-day occurrences: %v", err)
	}
	var rows, nonAllDay int
	if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE NOT all_day) FROM schedules WHERE rule_id=$1`, rule.ID).Scan(&rows, &nonAllDay); err != nil {
		t.Fatal(err)
	}
	if rows < 2 || nonAllDay != 0 {
		t.Fatalf("backfilled rows=%d, non-all-day=%d; want multiple all-day rows", rows, nonAllDay)
	}
	var badBoundaries int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedules WHERE rule_id=$1 AND (start_at AT TIME ZONE timezone <> date_trunc('day',start_at AT TIME ZONE timezone) OR end_at AT TIME ZONE timezone <> date_trunc('day',end_at AT TIME ZONE timezone))`, rule.ID).Scan(&badBoundaries); err != nil {
		t.Fatal(err)
	}
	if badBoundaries != 0 {
		t.Fatalf("backfill produced %d non-midnight local boundaries", badBoundaries)
	}
}

func TestFollowingEditUsesOriginalStartAndRejectsOccurrenceDateChange(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	create := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now })
	makeDaily := func(title string, date time.Time) usecase.CreateRuleInput {
		start := date.Add(9 * time.Hour)
		return usecase.CreateRuleInput{Title: title, Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1}
	}
	update := func(id, date string, revision int32, patch usecase.UpdateRuleInput) (dao.Rule, error) {
		return usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now }).Execute(ctx, userID, id, date, "following", patch, revision)
	}

	past := today.AddDate(0, 0, -4)
	pastRule, err := create.Execute(ctx, userID, makeDaily("Past original", past))
	if err != nil {
		t.Fatalf("create past rule: %v", err)
	}
	pastOccurrence := past.AddDate(0, 0, 1)
	movedFuture := time.Date(today.Year(), today.Month(), today.Day()+5, 15, 0, 0, 0, location)
	if _, err := usecase.NewRescheduleOccurrenceUseCase(uow, nil, (ruleTestIDs{}).Generate).Execute(ctx, userID, pastRule.ID,
		pastOccurrence.Format("2006-01-02"), movedFuture, movedFuture.Add(time.Hour), 0); err != nil {
		t.Fatalf("move past original into future: %v", err)
	}
	title := "must reject based on original start"
	if _, err := update(pastRule.ID, pastOccurrence.Format("2006-01-02"), pastRule.Revision, usecase.UpdateRuleInput{Title: &title}); err != usecase.ErrOccurrenceInactive {
		t.Fatalf("following edit for moved-future past origin error = %v, want original-occurrence guard", err)
	}

	futureDate := today.AddDate(0, 0, 5)
	futureRule, err := create.Execute(ctx, userID, makeDaily("Future original", futureDate))
	if err != nil {
		t.Fatalf("create future rule: %v", err)
	}
	futureOccurrence := futureDate.AddDate(0, 0, 1)
	movedPast := time.Date(today.Year(), today.Month(), today.Day()-2, 15, 0, 0, 0, location)
	if _, err := usecase.NewRescheduleOccurrenceUseCase(uow, nil, (ruleTestIDs{}).Generate).Execute(ctx, userID, futureRule.ID,
		futureOccurrence.Format("2006-01-02"), movedPast, movedPast.Add(time.Hour), 0); err != nil {
		t.Fatalf("move future original into past: %v", err)
	}
	localStartOnWrongDate := futureDate.AddDate(0, 0, 2).Add(10 * time.Hour)
	if _, err := update(futureRule.ID, futureOccurrence.Format("2006-01-02"), futureRule.Revision,
		usecase.UpdateRuleInput{LocalStart: &localStartOnWrongDate}); err != usecase.ErrRescheduleDateMismatch {
		t.Fatalf("following start date change error = %v, want ErrRescheduleDateMismatch", err)
	}
	newTitle := "valid following change"
	if _, err := update(futureRule.ID, futureOccurrence.Format("2006-01-02"), futureRule.Revision,
		usecase.UpdateRuleInput{Title: &newTitle}); err != nil {
		t.Fatalf("following edit for moved-past future origin: %v", err)
	}
}

func TestOnceRepeatConversionsAndFollowingBaseEdit(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	date := today.AddDate(0, 0, 5)
	start := date.Add(9 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "One-off", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "once",
	})
	if err != nil {
		t.Fatalf("create one-off: %v", err)
	}
	update := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
	dateKey := date.Format("2006-01-02")
	newTitle := "Edited one-off"
	current, err := update.Execute(ctx, userID, rule.ID, dateKey, "following", usecase.UpdateRuleInput{Title: &newTitle}, rule.Revision)
	if err != nil {
		t.Fatalf("edit one-off following: %v", err)
	}
	occurrence, err := usecase.NewGetOccurrenceUseCase(uow).Execute(ctx, userID, rule.ID, dateKey)
	if err != nil || occurrence.Title != newTitle {
		t.Fatalf("one-off base edit occurrence = %+v, %v", occurrence, err)
	}
	daily, interval := "daily", 1
	current, err = update.Execute(ctx, userID, rule.ID, dateKey, "following", usecase.UpdateRuleInput{Frequency: &daily, Interval: &interval}, current.Revision)
	if err != nil {
		t.Fatalf("convert once to repeat: %v", err)
	}
	once := "once"
	nextDate := date.AddDate(0, 0, 2)
	current, err = update.Execute(ctx, userID, rule.ID, nextDate.Format("2006-01-02"), "following",
		usecase.UpdateRuleInput{Frequency: &once}, current.Revision)
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
		if row.RuleID == rule.ID && !allowedDates[row.OccurrenceDate] {
			t.Fatalf("repeat-to-once left a virtual recurrence at %s: %+v", row.OccurrenceDate, row)
		}
	}
	if current.Revision != rule.Revision+3 {
		t.Fatalf("aggregate revision=%d, want %d", current.Revision, rule.Revision+3)
	}
}

func TestFollowingEditKeepsMultidayDurationAcrossYearBoundary(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	start := time.Date(2026, time.December, 30, 22, 0, 0, 0, time.UTC)
	end := time.Date(2027, time.January, 1, 1, 0, 0, 0, time.UTC)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Overnight operation", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: end, Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create multi-day rule: %v", err)
	}
	date := "2026-12-31"
	title := "Updated overnight operation"
	if _, err := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now }).Execute(ctx, userID,
		rule.ID, date, "following", usecase.UpdateRuleInput{Title: &title}, rule.Revision); err != nil {
		t.Fatalf("following edit over year boundary: %v", err)
	}
	occurrence, err := usecase.NewGetOccurrenceUseCase(uow).Execute(ctx, userID, rule.ID, date)
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
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 3)
	start := startDate.Add(9 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Original", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurrence: %v", err)
	}
	update := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
	plannedDate := startDate.AddDate(0, 0, 5).Format("2006-01-02")
	plannedTitle := "Planned future revision"
	planned, err := update.Execute(ctx, userID, rule.ID, plannedDate, "following", usecase.UpdateRuleInput{Title: &plannedTitle}, rule.Revision)
	if err != nil {
		t.Fatalf("publish later following revision: %v", err)
	}
	earlierDate := startDate.AddDate(0, 0, 3).Format("2006-01-02")
	earlierTitle := "Replacement revision"
	replacement, err := update.Execute(ctx, userID, rule.ID, earlierDate, "following", usecase.UpdateRuleInput{Title: &earlierTitle}, planned.Revision)
	if err != nil {
		t.Fatalf("publish earlier following revision: %v", err)
	}
	if replacement.Revision != planned.Revision+1 {
		t.Fatalf("aggregate revision=%d, want %d", replacement.Revision, planned.Revision+1)
	}
	if _, err := update.Execute(ctx, userID, rule.ID, earlierDate, "following", usecase.UpdateRuleInput{Title: &plannedTitle}, rule.Revision); err != usecase.ErrOccurrenceRevisionMismatch {
		t.Fatalf("stale rule revision error=%v, want ErrOccurrenceRevisionMismatch", err)
	}
	get := usecase.NewGetOccurrenceUseCase(uow)
	for offset := 3; offset <= 5; offset++ {
		date := startDate.AddDate(0, 0, offset).Format("2006-01-02")
		occurrence, err := get.Execute(ctx, userID, rule.ID, date)
		if err != nil || occurrence.Title != earlierTitle {
			t.Fatalf("occurrence %s resurrected superseded planned revision: %+v, %v", date, occurrence, err)
		}
	}
}

func TestConcurrentFollowingEditsWithSameRevisionProduceOneConflict(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID := createScheduleTestUser(t, pool)
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 7)
	start := startDate.Add(9 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Concurrent edit", Timezone: "Asia/Tokyo", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurrence: %v", err)
	}

	date := startDate.AddDate(0, 0, 2).Format("2006-01-02")
	update := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
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
			_, results[i] = update.Execute(ctx, userID, rule.ID, date, "following", usecase.UpdateRuleInput{Title: &title}, rule.Revision)
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
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, -8)
	start := startDate.Add(9 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Project daily cadence", ProjectID: projectID, Timezone: "Asia/Tokyo", LocalStart: start,
		LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create project recurrence: %v", err)
	}
	pausedAt := time.Date(startDate.Year(), startDate.Month(), startDate.Day()+1, 8, 0, 0, 0, location).UTC()
	resumedAt := time.Date(startDate.Year(), startDate.Month(), startDate.Day()+3, 8, 0, 0, 0, location).UTC()
	if err := uow.Do(ctx, func(ctx context.Context, repos usecase.Repositories) error {
		if err := repos.Rules().PauseProjectRules(ctx, userID, projectID, pausedAt); err != nil {
			return err
		}
		return repos.Rules().ResumeProjectRules(ctx, userID, projectID, resumedAt)
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
	var matchingPausedOccurrences int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM generate_series(1, 2) AS paused_day(day_offset)
		JOIN schedule_project_pause_intervals pause
		  ON pause.project_id = $1 AND pause.rule_id = $2
		 AND pause.paused_at <= (($3::date + paused_day.day_offset) + time '09:00') AT TIME ZONE 'Asia/Tokyo'
		 AND ((($3::date + paused_day.day_offset) + time '09:00') AT TIME ZONE 'Asia/Tokyo') < pause.resumed_at`,
		projectID, rule.ID, startDate.Format("2006-01-02")).Scan(&matchingPausedOccurrences); err != nil {
		t.Fatalf("check pause interval against recurrence starts: %v", err)
	}
	if matchingPausedOccurrences != 2 {
		t.Fatalf("pause interval matches %d of two expected occurrence starts; project=%s rule=%s paused_at=%s resumed_at=%v", matchingPausedOccurrences, projectID, rule.ID, pausedAt, persistedResume)
	}
	cutoff := time.Date(startDate.Year(), startDate.Month(), startDate.Day()+6, 12, 0, 0, 0, location).UTC()
	_, generatedSchedules, err := usecase.NewBackfillUseCase(uow, (ruleTestIDs{}).Generate).Execute(ctx, userID, cutoff, nil)
	if err != nil {
		t.Fatalf("backfill across pause interval: %v", err)
	}
	if generatedSchedules != 5 {
		t.Fatalf("backfill generated %d schedules, want 5 outside the two-day pause", generatedSchedules)
	}
	rows, err := pool.Query(ctx, `SELECT occurrence_date::text FROM schedules WHERE rule_id=$1 ORDER BY occurrence_date`, rule.ID)
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
	skippingID := createScheduleTestUser(t, pool)
	projectID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other','Schedule ACL project','low','open',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	roleID := "schedule_delete_only"
	skipRoleID := "schedule_skip_" + strings.ToLower(ulid.Make().String())
	if _, err := pool.Exec(ctx, `INSERT INTO roles(role_id,name) VALUES($1,'Schedule delete only')`, roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,permission_id FROM permissions WHERE resource_id='schedule' AND action IN ('read','delete') AND effect='allow'`, roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO roles(role_id,name) VALUES($1,'Occurrence skip only')`, skipRoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,permission_id FROM permissions WHERE effect='allow' AND ((resource_id='schedule' AND action='read') OR (resource_id='occurrence' AND action='update'))`, skipRoleID); err != nil {
		t.Fatal(err)
	}
	for _, member := range []struct{ id, role string }{{editorID, "editor"}, {viewerID, "viewer"}, {deleterID, roleID}, {skippingID, skipRoleID}} {
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
		_, _ = pool.Exec(context.Background(), `DELETE FROM role_permissions WHERE role_id=$1`, skipRoleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE role_id=$1`, skipRoleID)
	})
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	startDate := today.AddDate(0, 0, 6)
	start := startDate.Add(9 * time.Hour)
	create := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now })
	input := usecase.CreateRuleInput{Title: "Shared recurrence", ProjectID: projectID, Timezone: "Asia/Tokyo",
		LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1}
	rule, err := create.Execute(ctx, editorID, input)
	if err != nil {
		t.Fatalf("editor create rule: %v", err)
	}
	if rule.UserID != ownerID || rule.AssigneeID != ownerID {
		t.Fatalf("project-created rule identity/assignee = owner %s, assignee %s; want both project owner %s", rule.UserID, rule.AssigneeID, ownerID)
	}
	if _, err := create.Execute(ctx, viewerID, input); !errors.Is(err, usecase.ErrPermissionDenied) {
		t.Fatalf("viewer create error=%v, want ErrPermissionDenied", err)
	}
	personalStart := start.Add(time.Hour)
	personal, err := create.Execute(ctx, editorID, usecase.CreateRuleInput{Title: "Editor personal schedule", Timezone: "Asia/Tokyo",
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
		if occurrence.RuleID == rule.ID || occurrence.RuleID == personal.ID {
			t.Fatalf("personal /schedules view leaked another member's occurrence: %+v", occurrence)
		}
	}
	projectView, err := list.ExecutePage(ctx, viewerID, usecase.CursorPageRequest{Size: 20, From: from, To: from.AddDate(0, 0, 3), ProjectID: projectID})
	if err != nil {
		t.Fatalf("list viewer-readable Project schedules: %v", err)
	}
	foundProjectSchedule, foundPersonalSchedule := false, false
	var viewerCapabilities dao.ScheduleOccurrence
	for _, occurrence := range projectView.Items {
		if occurrence.RuleID == rule.ID {
			foundProjectSchedule = true
			viewerCapabilities = occurrence
		}
		foundPersonalSchedule = foundPersonalSchedule || occurrence.RuleID == personal.ID
	}
	if !foundProjectSchedule || foundPersonalSchedule {
		t.Fatalf("Project-scoped view must include the shared Project Schedule and hide the editor's personal Schedule: %+v", projectView.Items)
	}
	if viewerCapabilities.CanUpdate || viewerCapabilities.CanDelete || viewerCapabilities.CanSkip || viewerCapabilities.CanAssign || viewerCapabilities.CanUpdateFollowing || viewerCapabilities.CanDeleteFollowing {
		t.Fatalf("viewer capability flags exceed read-only ACL: %+v", viewerCapabilities)
	}
	deleterView, err := list.ExecutePage(ctx, deleterID, usecase.CursorPageRequest{Size: 20, From: from, To: from.AddDate(0, 0, 3), ProjectID: projectID})
	if err != nil {
		t.Fatalf("list delete-only member schedule: %v", err)
	}
	foundDeleterSchedule := false
	for _, occurrence := range deleterView.Items {
		if occurrence.RuleID == rule.ID {
			foundDeleterSchedule = true
			if !occurrence.CanDelete || occurrence.CanUpdate || occurrence.CanSkip || occurrence.CanAssign || !occurrence.CanDeleteFollowing || occurrence.CanUpdateFollowing {
				t.Fatalf("delete-only capability flags do not match ACL: %+v", occurrence)
			}
			break
		}
	}
	if !foundDeleterSchedule {
		t.Fatal("delete-only member could not read the shared schedule")
	}
	skippingView, err := list.ExecutePage(ctx, skippingID, usecase.CursorPageRequest{Size: 20, From: from, To: from.AddDate(0, 0, 3), ProjectID: projectID})
	if err != nil {
		t.Fatalf("list occurrence-skip-only member schedule: %v", err)
	}
	foundSkippingSchedule := false
	for _, occurrence := range skippingView.Items {
		if occurrence.RuleID == rule.ID {
			foundSkippingSchedule = true
			if occurrence.CanUpdate || !occurrence.CanSkip || occurrence.CanDelete || occurrence.CanUpdateFollowing || occurrence.CanDeleteFollowing {
				t.Fatalf("schedule update and occurrence skip capabilities were conflated: %+v", occurrence)
			}
			break
		}
	}
	if !foundSkippingSchedule {
		t.Fatal("occurrence-skip-only member could not read the shared schedule")
	}
	get := usecase.NewGetRuleUseCase(uow)
	if _, err := get.Execute(ctx, editorID, rule.ID); err != nil {
		t.Fatalf("editor read rule: %v", err)
	}
	updatedTitle := "Editor updated recurrence"
	update := usecase.NewUpdateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, func() time.Time { return now })
	current, err := update.Execute(ctx, editorID, rule.ID, startDate.Format("2006-01-02"), "following",
		usecase.UpdateRuleInput{Title: &updatedTitle}, rule.Revision)
	if err != nil {
		t.Fatalf("editor update rule: %v", err)
	}
	deleteFollowing := usecase.NewDeleteRuleFollowingUseCase(uow, (ruleTestIDs{}).Generate, func() time.Time { return now })
	cutoffDate := startDate.AddDate(0, 0, 2).Format("2006-01-02")
	if err := deleteFollowing.Execute(ctx, editorID, rule.ID, cutoffDate, current.Revision); err == nil {
		t.Fatal("editor deleted following occurrences without schedule:delete")
	}
	if err := deleteFollowing.Execute(ctx, deleterID, rule.ID, cutoffDate, current.Revision); err != nil {
		t.Fatalf("delete-only role could not delete following occurrences: %v", err)
	}

}

func TestGetMovedOccurrenceDoesNotRequireAccessToRuleProject(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	sourceOwnerID := createScheduleTestUser(t, pool)
	viewerID := createScheduleTestUser(t, pool)
	sourceProjectID, targetProjectID := ulid.Make().String(), ulid.Make().String()
	for _, project := range []struct{ id, owner, title string }{
		{sourceProjectID, sourceOwnerID, "Private source Project"},
		{targetProjectID, sourceOwnerID, "Readable destination Project"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other',$3,'low','open',$2)`,
			project.id, project.owner, project.title); err != nil {
			t.Fatalf("create Project %q: %v", project.title, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'viewer',$3)`,
		targetProjectID, viewerID, sourceOwnerID); err != nil {
		t.Fatalf("add destination Project viewer: %v", err)
	}
	uow := ruleTestUOW{pool: pool}
	now := time.Now().UTC()
	start := time.Now().In(time.FixedZone("JST", 9*60*60))
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, start.Location()).AddDate(0, 0, 2)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate,
		ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, sourceOwnerID,
		usecase.CreateRuleInput{Title: "Moved one-off", ProjectID: sourceProjectID, Timezone: "Asia/Tokyo",
			LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "once"})
	if err != nil {
		t.Fatalf("create source Project schedule: %v", err)
	}
	date := start.Format("2006-01-02")
	if _, err := pool.Exec(ctx, `UPDATE schedules SET project_id=$1 WHERE rule_id=$2 AND occurrence_date=$3`, targetProjectID, rule.ID, date); err != nil {
		t.Fatalf("move saved occurrence to readable Project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE schedules SET project_id=NULL WHERE rule_id=$1`, rule.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, targetProjectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, targetProjectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, targetProjectID)
	})

	occurrence, err := usecase.NewGetOccurrenceUseCase(uow).Execute(ctx, viewerID, rule.ID, date)
	if err != nil {
		t.Fatalf("read occurrence from destination Project without source Project access: %v", err)
	}
	if occurrence.ProjectID != targetProjectID {
		t.Fatalf("occurrence ProjectID = %q, want destination %q", occurrence.ProjectID, targetProjectID)
	}
	if occurrence.CanUpdateFollowing || occurrence.CanDeleteFollowing {
		t.Fatalf("following permissions leaked from unreadable source rule: %+v", occurrence)
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
	uow := ruleTestUOW{pool: pool, lifecycle: ruleProjectLifecycle{status: "done"}}
	now := time.Now().UTC()
	localNow := now.In(time.FixedZone("JST", 9*60*60))
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, 1).Add(9 * time.Hour)
	rule, err := usecase.NewCreateRuleUseCase(uow, (ruleTestIDs{}).Generate, ruleTimezoneValidator{queries: sqlc.New(pool)}, nil, func() time.Time { return now }).Execute(ctx, userID, usecase.CreateRuleInput{
		Title: "Paused after completion", ProjectID: projectID, Timezone: "Asia/Tokyo", LocalStart: start,
		LocalEnd: start.Add(time.Hour), Frequency: "daily", Interval: 1,
	})
	if err != nil {
		t.Fatalf("create recurrence in done project: %v", err)
	}
	var pausedAt time.Time
	var resumedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT paused_at,resumed_at FROM schedule_project_pause_intervals WHERE project_id=$1 AND rule_id=$2`, projectID, rule.ID).Scan(&pausedAt, &resumedAt); err != nil {
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
		if occurrence.RuleID == rule.ID {
			t.Fatalf("done Project recurrence was not paused: %+v", occurrence)
		}
	}
}

var scheduleTestUserRegistry = struct {
	sync.Mutex
	users map[*testing.T][]string
}{users: make(map[*testing.T][]string)}

func createScheduleTestUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := ulid.Make().String()
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone) VALUES($1,'Schedule','Integration',$2,'unused',now(),'Asia/Tokyo')`, id, id+"@schedule.test"); err != nil {
		t.Fatalf("insert schedule integration user: %v", err)
	}
	scheduleTestUserRegistry.Lock()
	_, alreadyRegistered := scheduleTestUserRegistry.users[t]
	scheduleTestUserRegistry.users[t] = append(scheduleTestUserRegistry.users[t], id)
	scheduleTestUserRegistry.Unlock()
	if !alreadyRegistered {
		t.Cleanup(func() {
			scheduleTestUserRegistry.Lock()
			users := append([]string(nil), scheduleTestUserRegistry.users[t]...)
			delete(scheduleTestUserRegistry.users, t)
			scheduleTestUserRegistry.Unlock()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cleanup := []struct {
				name  string
				query string
			}{
				{"schedule exceptions", `DELETE FROM schedule_exceptions WHERE user_id = ANY($1::text[]) OR changed_by = ANY($1::text[]) OR rule_id IN (SELECT id FROM schedule_rules WHERE user_id = ANY($1::text[]) OR id IN (SELECT rule_id FROM schedule_rule_revisions WHERE changed_by = ANY($1::text[])))`},
				{"schedule revisions", `DELETE FROM schedule_revisions WHERE changed_by = ANY($1::text[]) OR schedule_id IN (SELECT id FROM schedules WHERE user_id = ANY($1::text[]) OR changed_by = ANY($1::text[]) OR assignee_id = ANY($1::text[]) OR rule_id IN (SELECT id FROM schedule_rules WHERE user_id = ANY($1::text[]) OR id IN (SELECT rule_id FROM schedule_rule_revisions WHERE changed_by = ANY($1::text[]))))`},
				{"schedule backfill checkpoints", `DELETE FROM schedule_backfill_checkpoints WHERE revision_id IN (SELECT id FROM schedule_rule_revisions WHERE user_id = ANY($1::text[]) OR changed_by = ANY($1::text[]))`},
				{"schedule project pause intervals", `DELETE FROM schedule_project_pause_intervals WHERE rule_id IN (SELECT id FROM schedule_rules WHERE user_id = ANY($1::text[]) OR id IN (SELECT rule_id FROM schedule_rule_revisions WHERE changed_by = ANY($1::text[])))`},
				{"schedules", `DELETE FROM schedules WHERE user_id = ANY($1::text[]) OR changed_by = ANY($1::text[]) OR assignee_id = ANY($1::text[]) OR rule_id IN (SELECT id FROM schedule_rules WHERE user_id = ANY($1::text[]) OR id IN (SELECT rule_id FROM schedule_rule_revisions WHERE changed_by = ANY($1::text[])))`},
				{"schedule rule revisions", `DELETE FROM schedule_rule_revisions WHERE user_id = ANY($1::text[]) OR changed_by = ANY($1::text[]) OR rule_id IN (SELECT id FROM schedule_rules WHERE user_id = ANY($1::text[]) OR id IN (SELECT rule_id FROM schedule_rule_revisions WHERE changed_by = ANY($1::text[])))`},
				{"schedule rules", `DELETE FROM schedule_rules WHERE user_id = ANY($1::text[]) OR id IN (SELECT rule_id FROM schedule_rule_revisions WHERE changed_by = ANY($1::text[]))`},
				{"project members", `DELETE FROM project_members WHERE user_id = ANY($1::text[]) OR added_by = ANY($1::text[]) OR project_id IN (SELECT id FROM projects WHERE user_id = ANY($1::text[]))`},
				{"project revisions", `DELETE FROM project_revisions WHERE user_id = ANY($1::text[]) OR changed_by = ANY($1::text[]) OR id IN (SELECT id FROM projects WHERE user_id = ANY($1::text[]))`},
				{"projects", `DELETE FROM projects WHERE user_id = ANY($1::text[])`},
				{"users", `DELETE FROM users WHERE id = ANY($1::text[])`},
			}
			for _, step := range cleanup {
				if _, err := pool.Exec(ctx, step.query, users); err != nil {
					t.Errorf("cleanup %s for schedule integration users %v: %v", step.name, users, err)
					return
				}
			}
		})
	}
	return id
}

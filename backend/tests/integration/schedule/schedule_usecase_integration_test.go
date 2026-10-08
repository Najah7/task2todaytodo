//go:build integration

package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	schedulerepo "github.com/Najah7/task2todaytodo/internal/application/schedule/repository"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type scheduleDatabaseUOW struct{ pool *pgxpool.Pool }
type scheduleDatabaseRepositories struct{ schedules usecase.ScheduleRepository }

func (r scheduleDatabaseRepositories) Schedules() usecase.ScheduleRepository { return r.schedules }
func (scheduleDatabaseRepositories) ProjectLifecycle() shared.ProjectWorkLifecycle {
	return scheduleIntegrationProjectLifecycle{}
}

type scheduleIntegrationProjectLifecycle struct{}

func (scheduleIntegrationProjectLifecycle) LockParent(context.Context, string) error { return nil }
func (scheduleIntegrationProjectLifecycle) ReadStatus(context.Context, string) (string, error) {
	return "open", nil
}
func (scheduleIntegrationProjectLifecycle) CaptureWorkState(context.Context, string, time.Time) (shared.ProjectWorkState, error) {
	return shared.ProjectWorkState{Status: "open"}, nil
}
func (scheduleIntegrationProjectLifecycle) ReconcileWorkState(context.Context, string, string, shared.ProjectWorkState, time.Time) error {
	return nil
}

func (u scheduleDatabaseUOW) Do(ctx context.Context, fn func(context.Context, usecase.Repositories) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(ctx, scheduleDatabaseRepositories{schedules: schedulerepo.NewScheduleRepository(tx)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type scheduleTestID struct{}

func (scheduleTestID) Generate() string { return ulid.Make().String() }

func TestRecurringScheduleMutationsPreserveFirstOccurrenceSnapshot(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	userID, scheduleID := ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Occurrence',$2,'unused','Asia/Tokyo')`, userID, userID+"@schedule.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id IN (SELECT id FROM schedules WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE user_id=$1`, userID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, userID)
	})
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, location)
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(userID), "", domain.UserID(userID), "Original", "agenda", "Room A", 1, domain.Frequencies{{Value: "mon"}}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: rootDate, Timezone: location.String()})
	if err != nil {
		t.Fatal(err)
	}
	repo := schedulerepo.NewScheduleRepository(pool)
	if _, err := repo.CreateByUserID(ctx, domain.UserID(userID), schedule); err != nil {
		t.Fatalf("create Schedule root: %v", err)
	}
	uow := scheduleDatabaseUOW{pool: pool}
	idGenerator := scheduleTestID{}
	firstTitle, laterTitle := "First only", "Later template"
	update := usecase.NewUpdateScheduleUseCase(uow, nil, idGenerator)
	first, err := update.ExecuteOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), "2026-10-05", "current", usecase.PatchField[string]{Present: true, Value: &firstTitle}, usecase.PatchField[string]{}, usecase.PatchField[string]{})
	if err != nil {
		t.Fatalf("edit root occurrence: %v", err)
	}
	if first.ID != scheduleID || first.Title != firstTitle {
		t.Fatalf("edited root occurrence = %+v; want root ID and first-only title", first)
	}
	if _, err := update.ExecuteOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), "2026-10-19", "future", usecase.PatchField[string]{Present: true, Value: &laterTitle}, usecase.PatchField[string]{}, usecase.PatchField[string]{}); err != nil {
		t.Fatalf("update future template: %v", err)
	}
	page, err := usecase.NewListSchedulesUseCase(uow, nil, func() time.Time { return time.Date(2026, 10, 7, 0, 0, 0, 0, location) }).ExecutePage(ctx, domain.UserID(userID), usecase.CursorPageRequest{Size: 4, FromDate: "2026-10-05"})
	if err != nil {
		t.Fatal(err)
	}
	wantTitles := []string{firstTitle, laterTitle, laterTitle, laterTitle}
	for i, title := range wantTitles {
		if len(page.Items) <= i || page.Items[i].Title != title {
			t.Fatalf("schedule titles = %+v; item %d want %q", page.Items, i, title)
		}
	}
	stored, err := repo.GetByUserIDWithPermission(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), usecaseScheduleUpdate())
	if err != nil || stored.Title != laterTitle || stored.RepeatState != "active" || stored.FrequencyAnchorDate != rootDate.Unix() {
		t.Fatalf("stored root = %+v error=%v; want later active template anchored at original date", stored, err)
	}
}

func TestVirtualScheduleMutationsPersistOccurrencesAndCompletionDoesNotStopSeries(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	userID, scheduleID := ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Virtual',$2,'unused','Asia/Tokyo')`, userID, userID+"@schedule.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id IN (SELECT id FROM schedules WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE user_id=$1`, userID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, userID)
	})
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, location)
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(userID), "", domain.UserID(userID), "Weekly meeting", "", "Room A", 1, domain.Frequencies{{Value: "mon"}}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: rootDate, Timezone: location.String()})
	if err != nil {
		t.Fatal(err)
	}
	repo := schedulerepo.NewScheduleRepository(pool)
	if _, err := repo.CreateByUserID(ctx, domain.UserID(userID), schedule); err != nil {
		t.Fatalf("create recurring schedule: %v", err)
	}
	uow := scheduleDatabaseUOW{pool: pool}
	ids := scheduleTestID{}
	title := "Edited weekly meeting"
	updated, err := usecase.NewUpdateScheduleUseCase(uow, nil, ids).ExecuteOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), "2026-10-12", "current", usecase.PatchField[string]{Present: true, Value: &title}, usecase.PatchField[string]{}, usecase.PatchField[string]{})
	if err != nil {
		t.Fatalf("update virtual schedule: %v", err)
	}
	if _, err := ulid.ParseStrict(updated.ID); err != nil || updated.ID == usecase.VirtualOccurrenceID {
		t.Fatalf("edited virtual occurrence ID = %q, want persisted ULID (error %v)", updated.ID, err)
	}
	if err := usecase.NewCompleteScheduleUseCase(uow, nil, ids).ExecuteOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), "2026-10-05"); err != nil {
		t.Fatalf("complete root occurrence: %v", err)
	}
	if err := usecase.NewCompleteScheduleUseCase(uow, nil, ids).ExecuteOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), "2026-10-19"); err != nil {
		t.Fatalf("complete later virtual occurrence: %v", err)
	}
	rows, err := repo.ListByAssigneeUserID(ctx, domain.UserID(userID))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"2026-10-05": true, "2026-10-12": false, "2026-10-19": true}
	for date, completed := range want {
		found := false
		for _, row := range rows {
			if row.OccurrenceDate == date {
				found = true
				if _, err := ulid.ParseStrict(row.ID); err != nil || row.ID == usecase.VirtualOccurrenceID {
					t.Errorf("occurrence %s persisted ID=%q, error=%v", date, row.ID, err)
				}
				if row.Completed != completed {
					t.Errorf("occurrence %s completed=%v, want %v", date, row.Completed, completed)
				}
			}
		}
		if !found {
			t.Errorf("occurrence %s was not saved", date)
		}
	}
	page, err := usecase.NewListSchedulesUseCase(uow, nil, func() time.Time { return time.Date(2026, 10, 7, 0, 0, 0, 0, location) }).ExecutePage(ctx, domain.UserID(userID), usecase.CursorPageRequest{Size: 5, FromDate: "2026-10-05"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 5 {
		t.Fatalf("completed occurrence stopped future generation: got %d occurrences, want 5", len(page.Items))
	}
	if !page.Items[2].Completed {
		t.Fatalf("completed October 19 occurrence not visible as completed: %+v", page.Items[2])
	}
	if !page.Items[0].Completed {
		t.Fatalf("completed root occurrence not visible as completed: %+v", page.Items[0])
	}
	if page.Items[3].Completed || page.Items[4].Completed {
		t.Fatalf("completion leaked to future virtual occurrences: %+v", page.Items)
	}
}

func TestOneOffScheduleCanRescheduleWithoutOccurrenceDate(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	userID, scheduleID := ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','One off',$2,'unused','Asia/Tokyo')`, userID, userID+"@schedule.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id IN (SELECT id FROM schedules WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE user_id=$1`, userID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, userID)
	})
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 6, 9, 0, 0, 0, location)
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(userID), "", domain.UserID(userID), "One-off", "", "", 0, nil, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), Timezone: location.String()})
	if err != nil {
		t.Fatal(err)
	}
	repo := schedulerepo.NewScheduleRepository(pool)
	if _, err := repo.CreateByUserID(ctx, domain.UserID(userID), schedule); err != nil {
		t.Fatalf("create one-off Schedule: %v", err)
	}
	newStart := start.Add(2 * time.Hour)
	updated, err := usecase.NewRescheduleScheduleUseCase(scheduleDatabaseUOW{pool: pool}, nil, scheduleTestID{}).Execute(ctx, usecase.RescheduleScheduleInput{UserID: domain.UserID(userID), ScheduleID: domain.ScheduleID(scheduleID), StartAt: newStart, EndAt: newStart.Add(time.Hour), Scope: "future"})
	if err != nil {
		t.Fatalf("reschedule one-off without occurrence date: %v", err)
	}
	if updated.ID != scheduleID || updated.StartAt != newStart.Unix() || updated.RepeatState != "one_off" || updated.IntervalWeeks != 0 {
		t.Fatalf("rescheduled one-off Schedule = %+v", updated)
	}
}

func TestScheduleRecurrenceStoragePreservesSnapshotsAndSkipOverrides(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	userID, scheduleID := ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Storage',$2,'unused','Asia/Tokyo')`, userID, userID+"@schedule.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id IN (SELECT id FROM schedules WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE user_id=$1`, userID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, userID)
	})
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 10, 5, 9, 30, 0, 0, location)
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(userID), "", domain.UserID(userID), "Weekly meeting", "agenda", "Room A", 1, domain.Frequencies{{Value: "mon"}}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: rootDate, Timezone: location.String()})
	if err != nil {
		t.Fatal(err)
	}
	repo := schedulerepo.NewScheduleRepository(pool)
	if _, err := repo.CreateByUserID(ctx, domain.UserID(userID), schedule); err != nil {
		t.Fatalf("create schedule root: %v", err)
	}
	newAnchor := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	if err := repo.SetRecurrenceByUserID(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), newAnchor, 2, []dao.Frequency{{Value: "wed"}}); err != nil {
		t.Fatalf("replace recurrence: %v", err)
	}
	root, err := repo.GetByUserIDWithPermission(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), shared.ScheduleUpdate())
	if err != nil || root.RepeatState != "active" || root.FrequencyAnchorDate != newAnchor.Unix() || root.IntervalWeeks != 2 || len(root.Frequencies) != 1 || root.Frequencies[0].Value != "wed" {
		t.Fatalf("updated recurrence root = %+v, error=%v", root, err)
	}
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), rootDate, true); err != nil {
		t.Fatalf("skip root occurrence: %v", err)
	}
	var rootDeleted, firstDateSkipped bool
	if err := pool.QueryRow(ctx, `SELECT root.deleted_at IS NOT NULL, child.skipped_at IS NOT NULL FROM schedules root JOIN schedules child ON child.series_id=root.id AND child.occurrence_date=root.occurrence_date AND child.id<>child.series_id WHERE root.id=$1`, scheduleID).Scan(&rootDeleted, &firstDateSkipped); err != nil {
		t.Fatalf("read skipped root-date marker: %v", err)
	}
	if rootDeleted || !firstDateSkipped {
		t.Fatalf("skipping root occurrence deleted_root=%v skipped_marker=%v", rootDeleted, firstDateSkipped)
	}
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), rootDate, false); err != nil {
		t.Fatalf("restore root occurrence: %v", err)
	}
	var firstDateChildren, activeFirstDateChildren int
	if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE deleted_at IS NULL) FROM schedules WHERE series_id=$1 AND occurrence_date=$2 AND id<>series_id`, scheduleID, rootDate).Scan(&firstDateChildren, &activeFirstDateChildren); err != nil || firstDateChildren != 1 || activeFirstDateChildren != 0 {
		t.Fatalf("restored root occurrence children=%d active=%d error=%v; want a retained tombstone and no active marker", firstDateChildren, activeFirstDateChildren, err)
	}
	updated := schedule
	updated.Title, updated.Description, updated.Location = "New template", "future template", "Room B"
	updated.StartAt = time.Date(2026, 10, 5, 10, 0, 0, 0, location)
	updated.EndAt = updated.StartAt.Add(time.Hour)
	snapshotID := domain.ScheduleID(ulid.Make().String())
	if _, err := repo.UpdateSeriesTemplateByUserID(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), snapshotID, updated); err != nil {
		t.Fatalf("update recurrence template: %v", err)
	}
	var snapshotTitle, rootTitle string
	var snapshotStart, rootStart time.Time
	if err := pool.QueryRow(ctx, `SELECT child.title,root.title,child.start_at,root.start_at FROM schedules root JOIN schedules child ON child.series_id=root.id AND child.occurrence_date=root.occurrence_date AND child.id<>child.series_id WHERE root.id=$1`, scheduleID).Scan(&snapshotTitle, &rootTitle, &snapshotStart, &rootStart); err != nil {
		t.Fatalf("read root snapshot: %v", err)
	}
	if snapshotTitle != "Weekly meeting" || rootTitle != "New template" || !snapshotStart.Equal(start.UTC()) || !rootStart.Equal(updated.StartAt.UTC()) {
		t.Fatalf("saved first occurrence/root template=%q/%q %v/%v", snapshotTitle, rootTitle, snapshotStart, rootStart)
	}
	skipDate := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), skipDate, true); err != nil {
		t.Fatalf("skip future date: %v", err)
	}
	skips, err := repo.ListSkippedOccurrences(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID))
	if err != nil || len(skips) != 1 || skips[0] != skipDate.Unix() {
		t.Fatalf("skipped dates=%v error=%v; want %s", skips, err, skipDate.Format("2006-01-02"))
	}
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), skipDate, false); err != nil {
		t.Fatalf("restore future date: %v", err)
	}
	skips, err = repo.ListSkippedOccurrences(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID))
	if err != nil || len(skips) != 0 {
		t.Fatalf("restored skipped dates=%v error=%v; want none", skips, err)
	}
	editedDate := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	editedID := domain.ScheduleID(ulid.Make().String())
	editedStart := time.Date(2026, 11, 2, 12, 0, 0, 0, location)
	edited, err := domain.NewScheduleWithDetails(editedID, domain.UserID(userID), "", domain.UserID(userID), "Saved edit", "keep this", "Room C", 0, nil, editedStart, editedStart.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	edited, err = edited.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: editedDate, Timezone: location.String(), IsException: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertOverrideByUserID(ctx, domain.UserID(userID), edited); err != nil {
		t.Fatalf("save edited override: %v", err)
	}
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), editedDate, true); err != nil {
		t.Fatalf("skip saved override: %v", err)
	}
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), editedDate, false); err != nil {
		t.Fatalf("restore saved override: %v", err)
	}
	var restoredTitle string
	var restoredDeleted, restoredSkipped bool
	if err := pool.QueryRow(ctx, `SELECT title,deleted_at IS NOT NULL,skipped_at IS NOT NULL FROM schedules WHERE series_id=$1 AND occurrence_date=$2 AND id<>series_id`, scheduleID, editedDate).Scan(&restoredTitle, &restoredDeleted, &restoredSkipped); err != nil {
		t.Fatalf("read restored override: %v", err)
	}
	if restoredTitle != "Saved edit" || restoredDeleted || restoredSkipped {
		t.Fatalf("restored override title=%q deleted=%v skipped=%v", restoredTitle, restoredDeleted, restoredSkipped)
	}
	deletedSkipDate := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), deletedSkipDate, true); err != nil {
		t.Fatalf("create deleted skip marker: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE schedules SET deleted_at=now() WHERE series_id=$1 AND occurrence_date=$2 AND id<>series_id`, scheduleID, deletedSkipDate); err != nil {
		t.Fatalf("tombstone skip marker: %v", err)
	}
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), deletedSkipDate, false); err != nil {
		t.Fatalf("restore deleted skip marker: %v", err)
	}
	var skipDeleted, skipRetained bool
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,skipped_at IS NOT NULL FROM schedules WHERE series_id=$1 AND occurrence_date=$2 AND id<>series_id`, scheduleID, deletedSkipDate).Scan(&skipDeleted, &skipRetained); err != nil {
		t.Fatalf("read tombstoned skip marker after restore: %v", err)
	}
	if !skipDeleted || !skipRetained {
		t.Fatalf("restored tombstoned skip marker deleted=%v skipped=%v; want both retained", skipDeleted, skipRetained)
	}
	skips, err = repo.ListSkippedOccurrences(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID))
	if err != nil || len(skips) != 0 {
		t.Fatalf("active skipped dates after deleted-marker restore=%v error=%v; want none", skips, err)
	}
	if err := repo.StopRecurrenceByUserID(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID)); err != nil {
		t.Fatalf("stop recurrence: %v", err)
	}
	root, err = repo.GetByUserIDWithPermission(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), shared.ScheduleUpdate())
	if err != nil || root.RepeatState != "stopped" || root.IntervalWeeks != 0 || root.FrequencyAnchorDate != newAnchor.Unix() || len(root.Frequencies) != 0 {
		t.Fatalf("stopped root = %+v error=%v", root, err)
	}
}

func usecaseScheduleUpdate() shared.Capability { return shared.ScheduleUpdate() }

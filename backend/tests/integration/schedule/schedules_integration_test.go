//go:build integration

package schedule_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	projectrepo "github.com/Najah7/task2todaytodo/internal/application/project/repository"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	schedulerepo "github.com/Najah7/task2todaytodo/internal/application/schedule/repository"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func scheduleIntegrationPool(t *testing.T) *pgxpool.Pool {
	return testdb.Open(t)
}

// Project-created schedules are owned and initially assigned to the Project
// owner, even when a member with schedule.create is the actor.
func TestProjectScheduleCreationUsesProjectOwnerForOwnerAndAssignee(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	ownerID, actorID, viewerID, projectID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, actorID, viewerID} {
		if _, err := tx.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Test',$2,'unused','Asia/Tokyo')`, id, id+"@schedule.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Schedule project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3)`, projectID, actorID, ownerID); err != nil {
		t.Fatalf("add editor: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'viewer',$3)`, projectID, viewerID, ownerID); err != nil {
		t.Fatalf("add viewer: %v", err)
	}
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(actorID), domain.ProjectID(projectID), domain.UserID(actorID), "Member-created schedule", "", "", 0, nil, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := schedulerepo.NewScheduleRepository(tx).CreateByUserID(ctx, domain.UserID(actorID), schedule)
	if err != nil {
		t.Fatalf("create Project Schedule: %v", err)
	}
	if created.UserID != ownerID || created.AssigneeID != ownerID {
		t.Fatalf("created Schedule owner=%q assignee=%q; want Project owner %q for both", created.UserID, created.AssigneeID, ownerID)
	}
	viewerScheduleID := ulid.Make().String()
	viewerSchedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(viewerScheduleID), domain.UserID(viewerID), domain.ProjectID(projectID), domain.UserID(viewerID), "Viewer cannot create", "", "", 0, nil, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	viewerSchedule, err = viewerSchedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: viewerScheduleID, OccurrenceDate: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schedulerepo.NewScheduleRepository(tx).CreateByUserID(ctx, domain.UserID(viewerID), viewerSchedule); err == nil {
		t.Fatal("Project viewer created a Schedule without schedule.create permission")
	}
	read, err := schedulerepo.NewScheduleRepository(tx).GetByUserIDWithPermission(ctx, domain.UserID(actorID), domain.ScheduleID(scheduleID), shared.ScheduleRead())
	if err != nil {
		t.Fatalf("editor cannot read created Project schedule: %v", err)
	}
	if read.ProjectID != projectID {
		t.Fatalf("project ID = %q, want %q", read.ProjectID, projectID)
	}
	revisions, err := schedulerepo.NewScheduleRepository(tx).ListScheduleRevisionsByActor(ctx, domain.UserID(actorID), domain.ScheduleID(scheduleID), 10, nil)
	if err != nil || len(revisions) != 1 {
		t.Fatalf("initial Schedule history = %+v, error %v; want one snapshot", revisions, err)
	}
	if revisions[0].Revision != 1 || revisions[0].ChangedBy != actorID {
		t.Fatalf("initial Schedule history revision/actor = %d/%q; want 1/%q", revisions[0].Revision, revisions[0].ChangedBy, actorID)
	}
	schedule.Title = "Editor updated shared Schedule"
	updated, err := schedulerepo.NewScheduleRepository(tx).UpdateByUserID(ctx, domain.UserID(actorID), schedule, shared.ScheduleUpdate())
	if err != nil || updated.Title != schedule.Title {
		t.Fatalf("Project editor update = %+v, error=%v; want title %q", updated, err, schedule.Title)
	}
	if err := schedulerepo.NewScheduleRepository(tx).DeleteByUserID(ctx, domain.UserID(actorID), domain.ScheduleID(scheduleID), shared.ScheduleDelete()); !errors.Is(err, domain.ErrScheduleNotFound) {
		t.Fatalf("Project editor delete = %v, want denied/hidden Schedule", err)
	}
}

func TestScheduleHistoryPreservesFullRecurrenceSettings(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	userID, scheduleID := ulid.Make().String(), ulid.Make().String()
	if _, err := tx.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','History',$2,'unused','Asia/Tokyo')`, userID, userID+"@schedule.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	frequency := domain.Frequency{Value: "mon"}
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(userID), "", domain.UserID(userID), "Weekly history", "", "", 1, domain.Frequencies{frequency}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	repo := schedulerepo.NewScheduleRepository(tx)
	if _, err := repo.CreateByUserID(ctx, domain.UserID(userID), schedule); err != nil {
		t.Fatalf("create recurring Schedule: %v", err)
	}
	initial, err := repo.ListScheduleRevisionsByActor(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), 10, nil)
	if err != nil || len(initial) != 1 {
		t.Fatalf("initial recurring history = %+v, error %v; want one snapshot", initial, err)
	}
	if initial[0].IntervalWeeks != 1 || len(initial[0].Frequencies) != 1 || initial[0].Frequencies[0].Value != "mon" {
		t.Fatalf("initial history recurrence interval/frequencies = %d/%+v, want 1/[mon]", initial[0].IntervalWeeks, initial[0].Frequencies)
	}
	if err := repo.SetRecurrenceByUserID(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), 2, []dao.Frequency{{Value: "tue"}}); err != nil {
		t.Fatalf("update recurring settings: %v", err)
	}
	history, err := repo.ListScheduleRevisionsByActor(ctx, domain.UserID(userID), domain.ScheduleID(scheduleID), 10, nil)
	if err != nil || len(history) != 2 {
		t.Fatalf("updated recurrence history = %+v, error %v; want two snapshots", history, err)
	}
	latest := history[0]
	if latest.Revision != 2 || latest.ChangedBy != userID || latest.IntervalWeeks != 2 || len(latest.Frequencies) != 1 || latest.Frequencies[0].Value != "tue" {
		t.Fatalf("latest history snapshot = %+v; want revision 2, actor owner, interval 2, [tue]", latest)
	}
}

func TestProjectScheduleDeletionSoftDeletesRootOverrideAndSkippedMarker(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	ownerID, projectID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := tx.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Delete',$2,'unused','Asia/Tokyo')`, ownerID, ownerID+"@schedule.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Delete schedule project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(ownerID), domain.ProjectID(projectID), domain.UserID(ownerID), "Weekly project schedule", "", "", 1, domain.Frequencies{{Value: "mon"}}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	repo := schedulerepo.NewScheduleRepository(tx)
	if _, err := repo.CreateByUserID(ctx, domain.UserID(ownerID), schedule); err != nil {
		t.Fatalf("create Project Schedule: %v", err)
	}
	overrideDate := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	override := schedule
	override.ID = domain.ScheduleID(ulid.Make().String())
	override.OccurrenceDate = overrideDate
	override.StartAt, override.EndAt = start.AddDate(0, 0, 7), start.AddDate(0, 0, 7).Add(time.Hour)
	override.IsException = true
	if _, err := repo.UpsertOverrideByUserID(ctx, domain.UserID(ownerID), override); err != nil {
		t.Fatalf("create saved override: %v", err)
	}
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(ownerID), domain.ScheduleID(scheduleID), time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), true); err != nil {
		t.Fatalf("create skipped marker: %v", err)
	}
	if err := repo.DeleteProjectSchedulesByActor(ctx, domain.UserID(ownerID), domain.ProjectID(projectID)); err != nil {
		t.Fatalf("delete Project Schedules: %v", err)
	}
	var rows, deleted, wrongActor int
	if err := tx.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE deleted_at IS NOT NULL), count(*) FILTER (WHERE changed_by <> $2) FROM schedules WHERE series_id=$1`, scheduleID, ownerID).Scan(&rows, &deleted, &wrongActor); err != nil {
		t.Fatalf("inspect deleted Project Schedules: %v", err)
	}
	if rows != 3 || deleted != 3 || wrongActor != 0 {
		t.Fatalf("Project deletion rows/deleted/wrong actor = %d/%d/%d; want 3/3/0", rows, deleted, wrongActor)
	}
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(ownerID), domain.ScheduleID(scheduleID), time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), true); err == nil {
		t.Fatal("schedule occurrence mutation after Project deletion succeeded")
	}
}

func TestProjectAndAssigneeChangesPropagateToSavedAndTombstonedOccurrences(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	ownerID, firstAssigneeID, secondAssigneeID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	firstProjectID, secondProjectID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, firstAssigneeID, secondAssigneeID} {
		if _, err := tx.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Assignment',$2,'unused','Asia/Tokyo')`, id, id+"@schedule.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	for _, projectID := range []string{firstProjectID, secondProjectID} {
		if _, err := tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Assignment project','low',$2)`, projectID, ownerID); err != nil {
			t.Fatalf("insert Project: %v", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3),($4,$5,'editor',$3)`, firstProjectID, firstAssigneeID, ownerID, secondProjectID, secondAssigneeID); err != nil {
		t.Fatalf("add eligible assignees: %v", err)
	}
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(ownerID), domain.ProjectID(firstProjectID), domain.UserID(ownerID), "Weekly assignment", "", "", 1, domain.Frequencies{{Value: "mon"}}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: rootDate, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	repo := schedulerepo.NewScheduleRepository(tx)
	if _, err := repo.CreateByUserID(ctx, domain.UserID(ownerID), schedule); err != nil {
		t.Fatalf("create Project Schedule: %v", err)
	}
	overrideDate := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	override := schedule
	override.ID = domain.ScheduleID(ulid.Make().String())
	override.OccurrenceDate = overrideDate
	override.StartAt, override.EndAt = start.AddDate(0, 0, 7), start.AddDate(0, 0, 7).Add(time.Hour)
	override.IsException = true
	if _, err := repo.UpsertOverrideByUserID(ctx, domain.UserID(ownerID), override); err != nil {
		t.Fatalf("create saved override: %v", err)
	}
	tombstoneDate := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	if err := repo.SetSkippedOccurrence(ctx, domain.UserID(ownerID), domain.ScheduleID(scheduleID), tombstoneDate, true); err != nil {
		t.Fatalf("create skipped occurrence: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE schedules SET deleted_at=now() WHERE series_id=$1 AND occurrence_date=$2 AND id<>series_id`, scheduleID, tombstoneDate); err != nil {
		t.Fatalf("tombstone skipped occurrence: %v", err)
	}
	if err := repo.SetAssigneeByUserID(ctx, domain.UserID(ownerID), domain.ScheduleID(scheduleID), domain.UserID(firstAssigneeID)); err != nil {
		t.Fatalf("assign current project member: %v", err)
	}
	if err := repo.ReassignProjectMemberSchedules(ctx, domain.UserID(ownerID), domain.ProjectID(firstProjectID), domain.UserID(firstAssigneeID)); err != nil {
		t.Fatalf("reassign removed Project member's Schedule occurrences: %v", err)
	}
	var reassignRows, wrongOwnerOrActor int
	if err := tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE assignee_id<>user_id OR changed_by<>$2) FROM schedules WHERE series_id=$1`, scheduleID, ownerID).Scan(&reassignRows, &wrongOwnerOrActor); err != nil {
		t.Fatalf("inspect removed-member reassignment: %v", err)
	}
	if reassignRows != 3 || wrongOwnerOrActor != 0 {
		t.Fatalf("member reassignment rows=%d wrong owner/actor=%d; want root, override, tombstone assigned to owner", reassignRows, wrongOwnerOrActor)
	}
	if err := repo.SetAssigneeByUserID(ctx, domain.UserID(ownerID), domain.ScheduleID(scheduleID), domain.UserID(firstAssigneeID)); err != nil {
		t.Fatalf("reassign to first Project member before moving series: %v", err)
	}
	if err := repo.SetProjectByUserID(ctx, domain.UserID(ownerID), domain.ScheduleID(scheduleID), projectIDPointer(domain.ProjectID(secondProjectID))); err != nil {
		t.Fatalf("move recurring series to second Project: %v", err)
	}
	var movedRows, movedProjectWrongAssignee int
	if err := tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE project_id<>$2 OR assignee_id<>user_id) FROM schedules WHERE series_id=$1`, scheduleID, secondProjectID).Scan(&movedRows, &movedProjectWrongAssignee); err != nil {
		t.Fatalf("inspect moved series: %v", err)
	}
	if movedRows != 3 || movedProjectWrongAssignee != 0 {
		t.Fatalf("moved series rows=%d with wrong Project/assignee=%d; want root, override, tombstone all moved and ineligible assignee reset", movedRows, movedProjectWrongAssignee)
	}
	if err := repo.SetAssigneeByUserID(ctx, domain.UserID(ownerID), domain.ScheduleID(scheduleID), domain.UserID(secondAssigneeID)); err != nil {
		t.Fatalf("assign eligible member in destination Project: %v", err)
	}
	var wrongAssignee int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE assignee_id<>$2 OR changed_by<>$3) FROM schedules WHERE series_id=$1`, scheduleID, secondAssigneeID, ownerID).Scan(&wrongAssignee); err != nil {
		t.Fatalf("inspect assigned series: %v", err)
	}
	if wrongAssignee != 0 {
		t.Fatalf("root, saved override, and tombstone with wrong assignee/actor=%d; want none", wrongAssignee)
	}
}

func projectIDPointer(id domain.ProjectID) *domain.ProjectID { return &id }

func TestScheduleDeletePermissionWorksWithoutReadOrUpdateGrant(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	ownerID, actorID, projectID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	roleID := "schedule_delete_only_" + strings.ToLower(ulid.Make().String())
	for _, id := range []string{ownerID, actorID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Permission',$2,'unused','Asia/Tokyo')`, id, id+"@schedule.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Permission project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO roles(role_id,name) VALUES($1,$1)`, roleID); err != nil {
		t.Fatalf("insert operation-only role: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, projectID, actorID, roleID, ownerID); err != nil {
		t.Fatalf("add operation-only project member: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,permission_id FROM permissions WHERE resource_id='schedule' AND action='delete' AND effect='allow'`, roleID); err != nil {
		t.Fatalf("grant schedule.delete only: %v", err)
	}
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(ownerID), domain.ProjectID(projectID), domain.UserID(ownerID), "Operation-only schedule", "", "", 0, nil, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schedulerepo.NewScheduleRepository(pool).CreateByUserID(ctx, domain.UserID(ownerID), schedule); err != nil {
		t.Fatalf("create Schedule: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_members WHERE project_id=$1 AND user_id=$2`, projectID, actorID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM roles WHERE role_id=$1`, roleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id IN ($1,$2)`, ownerID, actorID)
	})
	repo := schedulerepo.NewScheduleRepository(pool)
	if _, err := repo.GetByUserIDWithPermission(ctx, domain.UserID(actorID), domain.ScheduleID(scheduleID), shared.ScheduleRead()); !errors.Is(err, domain.ErrScheduleNotFound) {
		t.Fatalf("schedule.read without grant = %v, want hidden schedule", err)
	}
	if err := usecase.NewDeleteScheduleUseCase(scheduleDatabaseUOW{pool: pool}, nil).Execute(ctx, domain.UserID(actorID), domain.ScheduleID(scheduleID)); err != nil {
		t.Fatalf("schedule.delete with no read/update grant: %v", err)
	}
	var deleted bool
	var changedBy string
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,changed_by FROM schedules WHERE id=$1`, scheduleID).Scan(&deleted, &changedBy); err != nil {
		t.Fatalf("read deleted Schedule: %v", err)
	}
	if !deleted || changedBy != actorID {
		t.Fatalf("deleted=%v changed_by=%q; want deleted by operation-only actor %q", deleted, changedBy, actorID)
	}
}

func TestProjectScheduleSharingListsCreatesUpdatesAndDeletesByRole(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	ownerID, viewerID, editorID, adminID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	projectID, ownerScheduleID, editorScheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, viewerID, editorID, adminID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Shared',$2,'unused','Asia/Tokyo')`, id, id+"@schedule.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Shared Schedule project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	for _, member := range []struct{ id, role string }{{viewerID, "viewer"}, {editorID, "editor"}, {adminID, "admin"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, projectID, member.id, member.role, ownerID); err != nil {
			t.Fatalf("add %s member: %v", member.role, err)
		}
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id IN ($1,$2)`, ownerScheduleID, editorScheduleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE id IN ($1,$2)`, ownerScheduleID, editorScheduleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id IN ($1,$2,$3,$4)`, ownerID, viewerID, editorID, adminID)
	})
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	create := func(id, actor, title string) {
		t.Helper()
		schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(id), domain.UserID(actor), domain.ProjectID(projectID), domain.UserID(actor), title, "", "", 0, nil, start, start.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: id, OccurrenceDate: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := schedulerepo.NewScheduleRepository(pool).CreateByUserID(ctx, domain.UserID(actor), schedule); err != nil {
			t.Fatalf("create shared Schedule as %s: %v", actor, err)
		}
	}
	create(ownerScheduleID, ownerID, "Owner Schedule")
	create(editorScheduleID, editorID, "Editor Schedule")
	uow := scheduleDatabaseUOW{pool: pool}
	viewerPage, err := usecase.NewListProjectSchedulesUseCase(uow, nil, func() time.Time { return start }).ExecutePage(ctx, domain.UserID(viewerID), domain.ProjectID(projectID), usecase.CursorPageRequest{Size: 10, FromDate: "2026-10-07"})
	if err != nil || len(viewerPage.Items) != 2 {
		t.Fatalf("viewer Project Schedule page=%+v error=%v; want both shared schedules", viewerPage, err)
	}
	viewerPersonalPage, err := usecase.NewListSchedulesUseCase(uow, nil, func() time.Time { return start }).ExecutePage(ctx, domain.UserID(viewerID), usecase.CursorPageRequest{Size: 10, FromDate: "2026-10-07"})
	if err != nil || len(viewerPersonalPage.Items) != 0 {
		t.Fatalf("viewer personal Schedule page=%+v error=%v; want no schedules assigned to viewer", viewerPersonalPage, err)
	}
	updatedTitle := "Editor updated Schedule"
	updated, err := usecase.NewUpdateScheduleUseCase(uow, nil).ExecuteOccurrence(ctx, domain.UserID(editorID), domain.ScheduleID(editorScheduleID), "2026-10-07", "current", usecase.PatchField[string]{Present: true, Value: &updatedTitle}, usecase.PatchField[string]{}, usecase.PatchField[string]{})
	if err != nil || updated.Title != updatedTitle {
		t.Fatalf("editor update=%+v error=%v; want %q", updated, err, updatedTitle)
	}
	if err := usecase.NewDeleteScheduleUseCase(uow, nil).Execute(ctx, domain.UserID(editorID), domain.ScheduleID(editorScheduleID)); !errors.Is(err, domain.ErrScheduleNotFound) {
		t.Fatalf("editor delete=%v; want permission denied", err)
	}
	if err := usecase.NewDeleteScheduleUseCase(uow, nil).Execute(ctx, domain.UserID(adminID), domain.ScheduleID(editorScheduleID)); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
}

func TestTodoListCanLinkIndependentSchedule(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	userID, scheduleID, todoListID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','TodoList',$2,'unused','Asia/Tokyo')`, userID, userID+"@schedule.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(userID), "", domain.UserID(userID), "Linked independent schedule", "", "", 0, nil, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schedulerepo.NewScheduleRepository(pool).CreateByUserID(ctx, domain.UserID(userID), schedule); err != nil {
		t.Fatalf("create independent Schedule: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO todo_lists(id,user_id,list_date) VALUES($1,$2,'2026-10-07')`, todoListID, userID); err != nil {
		t.Fatalf("create TodoList: %v", err)
	}
	queries := sqlc.New(pool)
	if _, err := queries.AddTodoListSchedule(ctx, sqlc.AddTodoListScheduleParams{TodoListID: todoListID, ScheduleID: scheduleID}); err != nil {
		t.Fatalf("link independent Schedule: %v", err)
	}
	links, err := queries.ListTodoListSchedules(ctx, todoListID)
	if err != nil || len(links) != 1 || links[0].ScheduleID != scheduleID {
		t.Fatalf("TodoList Schedule links=%+v error=%v; want independent Schedule %q", links, err, scheduleID)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM todo_lists WHERE id=$1`, todoListID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, userID)
	})
}

func TestProjectScheduleProgressIncludesLiveOverrideAfterRootDeletion(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	ownerID, projectID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := tx.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Progress',$2,'unused','Asia/Tokyo')`, ownerID, ownerID+"@schedule.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Progress project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, location)
	root, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(ownerID), domain.ProjectID(projectID), domain.UserID(ownerID), "Weekly project schedule", "", "", 1, domain.Frequencies{{Value: "mon"}}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: rootDate, Timezone: location.String()})
	if err != nil {
		t.Fatal(err)
	}
	repo := schedulerepo.NewScheduleRepository(tx)
	if _, err := repo.CreateByUserID(ctx, domain.UserID(ownerID), root); err != nil {
		t.Fatalf("create Project Schedule root: %v", err)
	}
	overrideDate := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	override := root
	override.ID = domain.ScheduleID(ulid.Make().String())
	override.OccurrenceDate = overrideDate
	override.StartAt, override.EndAt = start.AddDate(0, 0, 7), start.AddDate(0, 0, 7).Add(time.Hour)
	override.Completed, override.IsException = true, true
	if _, err := repo.UpsertOverrideByUserID(ctx, domain.UserID(ownerID), override); err != nil {
		t.Fatalf("save completed override: %v", err)
	}
	if err := repo.TombstoneByUserID(ctx, domain.UserID(ownerID), domain.ScheduleID(scheduleID), shared.ScheduleDelete()); err != nil {
		t.Fatalf("delete series root: %v", err)
	}
	sources, err := projectrepo.NewProjectRepository(tx).ReadProjectProgressSources(ctx, []string{projectID}, time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC))
	if err != nil || len(sources.Schedules) != 1 {
		t.Fatalf("read Project Schedule progress sources=%+v error=%v", sources, err)
	}
	if sources.Schedules[0].Total != 1 || sources.Schedules[0].Completed != 1 || len(sources.Schedules[0].Roots) != 0 {
		t.Fatalf("progress after root deletion=%+v; want live completed override 1/1 and no active root projection", sources.Schedules[0])
	}
}

func TestScheduleTagsShareCatalogRespectProjectPermissionsAndDoNotReviseSchedule(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	ownerID, editorID, viewerID, otherOwnerID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	projectID, scheduleID, taskID, sharedTagID, foreignTagID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, editorID, viewerID, otherOwnerID} {
		if _, err := tx.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Tag',$2,'unused','Asia/Tokyo')`, id, id+"@schedule.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Tag project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	for _, membership := range []struct{ id, role string }{{editorID, "editor"}, {viewerID, "viewer"}} {
		if _, err := tx.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, projectID, membership.id, membership.role, ownerID); err != nil {
			t.Fatalf("add %s member: %v", membership.role, err)
		}
	}
	for _, tag := range []struct{ id, user, name string }{{sharedTagID, ownerID, "Shared"}, {foreignTagID, otherOwnerID, "Foreign"}} {
		if _, err := tx.Exec(ctx, `INSERT INTO tags(id,user_id,name) VALUES($1,$2,$3)`, tag.id, tag.user, tag.name); err != nil {
			t.Fatalf("insert tag: %v", err)
		}
	}
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	schedule, err := domain.NewScheduleWithDetails(domain.ScheduleID(scheduleID), domain.UserID(editorID), domain.ProjectID(projectID), domain.UserID(editorID), "Shared schedule", "", "", 0, nil, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, err = schedule.WithRecurrence(domain.RecurrenceMetadata{SeriesID: scheduleID, OccurrenceDate: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schedulerepo.NewScheduleRepository(tx).CreateByUserID(ctx, domain.UserID(editorID), schedule); err != nil {
		t.Fatalf("create Project Schedule: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,priority,status,changed_by) VALUES($1,$2,$3,$2,'Tag task','low','open',$4)`, taskID, ownerID, projectID, editorID); err != nil {
		t.Fatalf("insert shared task: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO task_tag_assignments(task_id,tag_id) VALUES($1,$2)`, taskID, sharedTagID); err != nil {
		t.Fatalf("attach catalog tag to Task: %v", err)
	}
	repo := schedulerepo.NewScheduleRepository(tx)
	var historyBefore int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM schedule_revisions WHERE id=$1`, scheduleID).Scan(&historyBefore); err != nil {
		t.Fatalf("count Schedule revisions before tag edits: %v", err)
	}
	if err := repo.AddTag(ctx, domain.UserID(editorID), domain.ScheduleID(scheduleID), sharedTagID); err != nil {
		t.Fatalf("editor add shared catalog tag: %v", err)
	}
	var sharedTask, sharedSchedule int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM task_tag_assignments WHERE task_id=$1 AND tag_id=$2`, taskID, sharedTagID).Scan(&sharedTask); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM schedule_tag_assignments WHERE schedule_id=$1 AND tag_id=$2`, scheduleID, sharedTagID).Scan(&sharedSchedule); err != nil {
		t.Fatal(err)
	}
	if sharedTask != 1 || sharedSchedule != 1 {
		t.Fatalf("shared tag assignments Task=%d Schedule=%d; want both 1", sharedTask, sharedSchedule)
	}
	if err := repo.AddTag(ctx, domain.UserID(editorID), domain.ScheduleID(scheduleID), foreignTagID); err != usecase.ErrScheduleTagNotFound {
		t.Fatalf("cross-owner tag add error = %v, want %v", err, usecase.ErrScheduleTagNotFound)
	}
	if err := repo.AddTag(ctx, domain.UserID(viewerID), domain.ScheduleID(scheduleID), sharedTagID); err != usecase.ErrScheduleTagNotFound {
		t.Fatalf("viewer tag add error = %v, want %v", err, usecase.ErrScheduleTagNotFound)
	}
	tags, err := repo.ListTags(ctx, domain.UserID(editorID), domain.ScheduleID(scheduleID))
	if err != nil || len(tags) != 1 || tags[0].ID != sharedTagID {
		t.Fatalf("editor Schedule tags = %+v, error %v", tags, err)
	}
	if err := repo.RemoveTag(ctx, domain.UserID(editorID), domain.ScheduleID(scheduleID), sharedTagID); err != nil {
		t.Fatalf("editor remove Schedule tag: %v", err)
	}
	var historyAfter int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM schedule_revisions WHERE id=$1`, scheduleID).Scan(&historyAfter); err != nil {
		t.Fatalf("count Schedule revisions after tag edits: %v", err)
	}
	if historyAfter != historyBefore {
		t.Fatalf("Schedule revisions changed from %d to %d after tag add/remove", historyBefore, historyAfter)
	}
}

func TestScheduleTagsAreSharedAcrossSeriesAndSavedOverrides(t *testing.T) {
	pool := scheduleIntegrationPool(t)
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	ownerID, editorID, viewerID, otherOwnerID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	projectID, rootID, overrideID, otherSeriesID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	tagA, tagB, foreignTag := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, editorID, viewerID, otherOwnerID} {
		if _, err := tx.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Series tags',$2,'unused','Asia/Tokyo')`, id, id+"@schedule.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Series tag project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	for _, membership := range []struct{ id, role string }{{editorID, "editor"}, {viewerID, "viewer"}} {
		if _, err := tx.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, projectID, membership.id, membership.role, ownerID); err != nil {
			t.Fatalf("add %s member: %v", membership.role, err)
		}
	}
	for _, tag := range []struct{ id, user, name string }{{tagA, ownerID, "Series A"}, {tagB, ownerID, "Series B"}, {foreignTag, otherOwnerID, "Foreign owner"}} {
		if _, err := tx.Exec(ctx, `INSERT INTO tags(id,user_id,name) VALUES($1,$2,$3)`, tag.id, tag.user, tag.name); err != nil {
			t.Fatalf("insert tag: %v", err)
		}
	}
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	root, err := domain.NewScheduleWithDetails(domain.ScheduleID(rootID), domain.UserID(editorID), domain.ProjectID(projectID), domain.UserID(editorID), "Weekly tagged series", "", "", 1, domain.Frequencies{{Value: "mon"}}, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.WithRecurrence(domain.RecurrenceMetadata{SeriesID: rootID, OccurrenceDate: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	repo := schedulerepo.NewScheduleRepository(tx)
	if _, err := repo.CreateByUserID(ctx, domain.UserID(editorID), root); err != nil {
		t.Fatalf("create recurring Schedule root: %v", err)
	}
	override := root
	override.ID = domain.ScheduleID(overrideID)
	override.OccurrenceDate = time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	override.StartAt = start.AddDate(0, 0, 9) // move this saved occurrence to Wednesday.
	override.EndAt = override.StartAt.Add(time.Hour)
	override.IsException = true
	if _, err := repo.UpsertOverrideByUserID(ctx, domain.UserID(editorID), override); err != nil {
		t.Fatalf("save moved override: %v", err)
	}
	other, err := domain.NewScheduleWithDetails(domain.ScheduleID(otherSeriesID), domain.UserID(ownerID), domain.ProjectID(projectID), domain.UserID(ownerID), "Separate tagged series", "", "", 0, nil, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	other, err = other.WithRecurrence(domain.RecurrenceMetadata{SeriesID: otherSeriesID, OccurrenceDate: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateByUserID(ctx, domain.UserID(ownerID), other); err != nil {
		t.Fatalf("create independent Schedule: %v", err)
	}

	countHistory := func() int64 {
		t.Helper()
		var count int64
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM schedule_revisions WHERE id IN ($1,$2,$3)`, rootID, overrideID, otherSeriesID).Scan(&count); err != nil {
			t.Fatalf("count Schedule history: %v", err)
		}
		return count
	}
	historyBefore := countHistory()
	assertTags := func(actor domain.UserID, scheduleID domain.ScheduleID, want ...string) {
		t.Helper()
		got, err := repo.ListTags(ctx, actor, scheduleID)
		if err != nil {
			t.Fatalf("list tags for %s: %v", scheduleID, err)
		}
		gotIDs := make([]string, 0, len(got))
		for _, tag := range got {
			gotIDs = append(gotIDs, tag.ID)
		}
		wantIDs := append([]string{}, want...)
		if !reflect.DeepEqual(gotIDs, wantIDs) {
			t.Fatalf("tags for %s = %v; want %v", scheduleID, gotIDs, want)
		}
	}
	if err := repo.AddTag(ctx, domain.UserID(editorID), domain.ScheduleID(rootID), tagA); err != nil {
		t.Fatalf("add tag through root: %v", err)
	}
	assertTags(domain.UserID(editorID), domain.ScheduleID(overrideID), tagA)
	if err := repo.AddTag(ctx, domain.UserID(editorID), domain.ScheduleID(overrideID), tagB); err != nil {
		t.Fatalf("add tag through moved override: %v", err)
	}
	assertTags(domain.UserID(editorID), domain.ScheduleID(rootID), tagA, tagB)
	assertTags(domain.UserID(editorID), domain.ScheduleID(overrideID), tagA, tagB)
	assertTags(domain.UserID(editorID), domain.ScheduleID(otherSeriesID))
	batchTags, err := repo.ListTagsForSchedules(ctx, domain.UserID(editorID), []domain.ScheduleID{domain.ScheduleID(rootID), domain.ScheduleID(overrideID), domain.ScheduleID(otherSeriesID)})
	if err != nil {
		t.Fatalf("batch list Schedule tags: %v", err)
	}
	if len(batchTags[rootID]) != 2 || len(batchTags[overrideID]) != 2 || batchTags[otherSeriesID] == nil || len(batchTags[otherSeriesID]) != 0 {
		t.Fatalf("batch tag projection = %+v; want shared series tags for root/override and empty separate series", batchTags)
	}
	if err := repo.RemoveTag(ctx, domain.UserID(editorID), domain.ScheduleID(overrideID), tagA); err != nil {
		t.Fatalf("remove tag through override: %v", err)
	}
	assertTags(domain.UserID(editorID), domain.ScheduleID(rootID), tagB)
	if err := repo.RemoveTag(ctx, domain.UserID(editorID), domain.ScheduleID(rootID), tagB); err != nil {
		t.Fatalf("remove tag through root: %v", err)
	}
	assertTags(domain.UserID(editorID), domain.ScheduleID(overrideID))
	if err := repo.AddTag(ctx, domain.UserID(editorID), domain.ScheduleID(overrideID), foreignTag); err != usecase.ErrScheduleTagNotFound {
		t.Fatalf("foreign-owner tag add through override = %v; want %v", err, usecase.ErrScheduleTagNotFound)
	}
	if err := repo.AddTag(ctx, domain.UserID(viewerID), domain.ScheduleID(overrideID), tagA); err != usecase.ErrScheduleTagNotFound {
		t.Fatalf("viewer tag add through override = %v; want %v", err, usecase.ErrScheduleTagNotFound)
	}
	if err := repo.AddTag(ctx, domain.UserID(editorID), domain.ScheduleID(rootID), tagA); err != nil {
		t.Fatalf("re-add tag before root deletion: %v", err)
	}
	if historyAfter := countHistory(); historyAfter != historyBefore {
		t.Fatalf("Schedule history changed from %d to %d after root/override tag operations", historyBefore, historyAfter)
	}
	if err := repo.TombstoneByUserID(ctx, domain.UserID(ownerID), domain.ScheduleID(rootID), shared.ScheduleDelete()); err != nil {
		t.Fatalf("soft-delete series root: %v", err)
	}
	assertTags(domain.UserID(editorID), domain.ScheduleID(overrideID), tagA)
	deletedRootBatch, err := repo.ListTagsForSchedules(ctx, domain.UserID(editorID), []domain.ScheduleID{domain.ScheduleID(overrideID)})
	if err != nil || len(deletedRootBatch[overrideID]) != 1 || deletedRootBatch[overrideID][0].ID != tagA {
		t.Fatalf("batch tag projection for live override after root deletion = %+v, error %v; want preserved series tag", deletedRootBatch, err)
	}
	if err := repo.AddTag(ctx, domain.UserID(editorID), domain.ScheduleID(overrideID), tagB); err != nil {
		t.Fatalf("add tag through live override after root deletion: %v", err)
	}
	assertTags(domain.UserID(editorID), domain.ScheduleID(overrideID), tagA, tagB)
	if err := repo.RemoveTag(ctx, domain.UserID(editorID), domain.ScheduleID(overrideID), tagA); err != nil {
		t.Fatalf("remove tag through live override after root deletion: %v", err)
	}
	assertTags(domain.UserID(editorID), domain.ScheduleID(overrideID), tagB)
	if err := repo.AddTag(ctx, domain.UserID(editorID), domain.ScheduleID(rootID), tagA); err != usecase.ErrScheduleTagNotFound {
		t.Fatalf("deleted root tag add = %v; want %v", err, usecase.ErrScheduleTagNotFound)
	}
	if historyAfter := countHistory(); historyAfter != historyBefore+1 {
		t.Fatalf("Schedule history after root tombstone/tag edits = %d; want only the one root deletion revision over %d", historyAfter, historyBefore)
	}
}

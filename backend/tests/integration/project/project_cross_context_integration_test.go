//go:build integration

package project_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	projectdomain "github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func projectCrossContextTestPool(t *testing.T) *pgxpool.Pool {
	return testdb.Open(t)
}

func TestProjectDeleteCoordinatesTaskAndScheduleAtomically(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	actorID, projectID, taskID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Audit',$2,'unused','Asia/Tokyo')`, actorID, actorID+"@project-audit.test"); err != nil {
		t.Fatalf("insert actor: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Atomic Project Delete','low',$2)`, projectID, actorID); err != nil {
		t.Fatalf("insert Project: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,priority,status,changed_by) VALUES($1,$2,$3,$2,'Task child','low','open',$2)`, taskID, actorID, projectID); err != nil {
		t.Fatalf("insert Task child: %v", err)
	}
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	if _, err := pool.Exec(ctx, `INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,repeat_state,interval_weeks,changed_by) VALUES($1,$2,$3,$2,'Schedule child',$4,$5,$1,'2026-10-07','Asia/Tokyo','one_off',0,$2)`, scheduleID, actorID, projectID, start, start.Add(time.Hour)); err != nil {
		t.Fatalf("insert Schedule child: %v", err)
	}
	triggerName := "fail_schedule_delete_" + strings.ToLower(scheduleID)
	functionName := "fail_schedule_delete_fn_" + strings.ToLower(scheduleID)
	triggerCreated := false
	t.Cleanup(func() {
		if triggerCreated {
			_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON schedules`, triggerName))
			_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, functionName))
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id = $1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id = $1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id = $1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id = $1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id = $1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, actorID)
	})
	triggerFunctionDDL := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.id = '%s' AND NEW.deleted_at IS NOT NULL THEN RAISE EXCEPTION 'injected Schedule delete failure'; END IF; RETURN NEW; END $$`, functionName, scheduleID)
	if _, err := pool.Exec(ctx, triggerFunctionDDL); err != nil {
		t.Fatalf("install scoped Schedule failure function: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE ON schedules FOR EACH ROW EXECUTE FUNCTION %s()`, triggerName, functionName)); err != nil {
		t.Fatalf("install scoped Schedule failure trigger: %v", err)
	}
	triggerCreated = true
	store := application.NewStore(pool)
	unitOfWork := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	deleteProject := projectusecase.NewDeleteProjectUseCase(unitOfWork, nil)
	if err := deleteProject.Execute(ctx, projectdomain.UserID(actorID), projectdomain.ProjectID(projectID), 1); err == nil || !strings.Contains(err.Error(), "injected Schedule delete failure") {
		t.Fatalf("DeleteProject() error = %v; want injected Schedule failure", err)
	}
	var projectDeleted, taskDeleted, scheduleDeleted bool
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM projects WHERE id=$1`, projectID).Scan(&projectDeleted); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM tasks WHERE id=$1`, taskID).Scan(&taskDeleted); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM schedules WHERE id=$1`, scheduleID).Scan(&scheduleDeleted); err != nil {
		t.Fatal(err)
	}
	if projectDeleted || taskDeleted || scheduleDeleted {
		t.Fatalf("failed Project delete partially committed: project=%v task=%v schedule=%v", projectDeleted, taskDeleted, scheduleDeleted)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER %s ON schedules`, triggerName)); err != nil {
		t.Fatalf("remove scoped Schedule failure trigger: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP FUNCTION %s()`, functionName)); err != nil {
		t.Fatalf("remove scoped Schedule failure function: %v", err)
	}
	triggerCreated = false
	if err := deleteProject.Execute(ctx, projectdomain.UserID(actorID), projectdomain.ProjectID(projectID), 1); err != nil {
		t.Fatalf("DeleteProject() after removing failure injection: %v", err)
	}
	var projectRevision, taskRevision, scheduleRevision int
	var projectChangedBy, taskChangedBy, scheduleChangedBy string
	if err := pool.QueryRow(ctx, `SELECT revision,changed_by FROM projects WHERE id=$1 AND deleted_at IS NOT NULL`, projectID).Scan(&projectRevision, &projectChangedBy); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT revision,changed_by FROM tasks WHERE id=$1 AND deleted_at IS NOT NULL`, taskID).Scan(&taskRevision, &taskChangedBy); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT revision,changed_by FROM schedules WHERE id=$1 AND deleted_at IS NOT NULL`, scheduleID).Scan(&scheduleRevision, &scheduleChangedBy); err != nil {
		t.Fatal(err)
	}
	if projectRevision != 2 || taskRevision != 2 || scheduleRevision != 2 || projectChangedBy != actorID || taskChangedBy != actorID || scheduleChangedBy != actorID {
		t.Fatalf("successful Project delete snapshots: project=(%d,%q) task=(%d,%q) schedule=(%d,%q); want revision 2 and actor %q", projectRevision, projectChangedBy, taskRevision, taskChangedBy, scheduleRevision, scheduleChangedBy, actorID)
	}
}

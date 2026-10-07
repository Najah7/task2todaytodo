//go:build integration

package project_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	projectdomain "github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectrepo "github.com/Najah7/task2todaytodo/internal/application/project/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduledomain "github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	sharedprogress "github.com/Najah7/task2todaytodo/internal/application/shared/progress"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type projectAuditTimezoneReader struct{}

func (projectAuditTimezoneReader) GetTimezone(context.Context, string) (string, error) {
	return "UTC", nil
}

type projectTaskAuditTimezoneReader struct{}

func (projectTaskAuditTimezoneReader) GetTimezone(context.Context, string) (string, error) {
	return "UTC", nil
}

func TestProjectScheduleAndTaskCreateUseProjectOwnerAndRecordMemberActor(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, actorID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, actorID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Actor',$2,'unused','UTC')`, id, id+"@project-flow.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Owner project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3)`, projectID, actorID, ownerID); err != nil {
		t.Fatal(err)
	}
	taskID, scheduleID := ulid.Make().String(), ulid.Make().String()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, actorID})
	})

	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task)
	task, err := taskusecase.NewCreateTaskInProjectUseCase(taskUOW, nil).Execute(ctx, taskusecase.CreateTaskInProjectInput{
		ID: taskdomain.TaskID(taskID), UserID: taskdomain.UserID(actorID), ProjectID: taskdomain.ProjectID(projectID), Title: "Created by editor",
	})
	if err != nil {
		t.Fatalf("create shared Task: %v", err)
	}
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule)
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	schedule, err := scheduleusecase.NewCreateScheduleUseCase(scheduleUOW, projectAuditTimezoneReader{}, nil).Execute(ctx, scheduleusecase.CreateScheduleInput{
		ID: scheduledomain.ScheduleID(scheduleID), UserID: scheduledomain.UserID(actorID), ProjectID: scheduledomain.ProjectID(projectID),
		Title: "Created by editor", StartAt: start, EndAt: start.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create shared Schedule: %v", err)
	}
	for _, got := range []struct {
		name, owner, assignee, actor string
		revision                     int32
	}{
		{"Task", task.UserID, task.AssigneeID, task.ChangedBy, task.Revision},
		{"Schedule", schedule.UserID, schedule.AssigneeID, schedule.ChangedBy, schedule.Revision},
	} {
		if got.owner != ownerID || got.assignee != ownerID || got.actor != actorID || got.revision != 1 {
			t.Errorf("shared %s create owner=%q assignee=%q actor=%q revision=%d; want project owner, owner, editor, 1", got.name, got.owner, got.assignee, got.actor, got.revision)
		}
	}
}

func TestProjectTaskListIncludesReadableTasksAcrossAssignees(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, editorID, viewerID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, editorID, viewerID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','List',$2,'unused','UTC')`, id, id+"@project-flow.test"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Task list project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	roleID := "project_list_viewer_" + strings.ToLower(ulid.Make().String())
	if _, err := pool.Exec(ctx, `INSERT INTO roles(role_id,name) VALUES($1,$1)`, roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3),($1,$4,$5,$3)`, projectID, editorID, ownerID, viewerID, roleID); err != nil {
		t.Fatal(err)
	}
	var taskRead int64
	if err := pool.QueryRow(ctx, `SELECT permission_id FROM permissions WHERE resource_id='task' AND action='read' AND effect='allow'`).Scan(&taskRead); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, roleID, taskRead); err != nil {
		t.Fatal(err)
	}
	taskIDs := []string{ulid.Make().String(), ulid.Make().String(), ulid.Make().String()}
	for i, value := range []struct{ assignee, title string }{{ownerID, "Owner task"}, {editorID, "Editor task"}, {viewerID, "Viewer task"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,changed_by) VALUES($1,$2,$3,$4,$5,$2)`, taskIDs[i], ownerID, projectID, value.assignee, value.title); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1 AND role_id=$2`, projectID, roleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE role_id=$1`, roleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=ANY($1::text[])`, taskIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=ANY($1::text[])`, taskIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, editorID, viewerID})
	})
	tasks := application.NewStore(pool).Task.Tasks
	projectList, err := tasks.ListByProjectAndUserIDCursor(ctx, taskdomain.UserID(viewerID), taskdomain.ProjectID(projectID), 10, nil)
	if err != nil || len(projectList) != 3 {
		t.Fatalf("readable Project task list = %d tasks, error %v; want all 3 assignees", len(projectList), err)
	}
	personal, err := tasks.ListByUserIDCursor(ctx, taskdomain.UserID(viewerID), 10, nil)
	if err != nil || len(personal) != 1 || personal[0].ID != taskIDs[2] {
		t.Fatalf("personal viewer task list = %+v, error %v; want only viewer-assigned Task", personal, err)
	}
}

func TestProjectMemberRemovalReassignsTaskAndScheduleRowsAtomically(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, memberID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	taskID, rootID, childID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, memberID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Member',$2,'unused','UTC')`, id, id+"@project-flow.test"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Member removal','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3)`, projectID, memberID, ownerID); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,changed_by) VALUES($1,$2,$3,$4,'Member task',$2)`, taskID, ownerID, projectID, memberID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,repeat_state,changed_by) VALUES($1,$2,$3,$4,'Member schedule',$5,$6,$1,'2026-10-07','UTC','one_off',$2)`, rootID, ownerID, projectID, memberID, start, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,is_exception,repeat_state,deleted_at,changed_by) VALUES($1,$2,$3,$4,'Saved deleted override',$5,$6,$7,'2026-10-08','UTC',true,NULL,now(),$2)`, childID, ownerID, projectID, memberID, start.AddDate(0, 0, 1), start.AddDate(0, 0, 1).Add(time.Hour), rootID); err != nil {
		t.Fatal(err)
	}
	triggerName := "fail_member_schedule_reassignment_" + strings.ToLower(rootID)
	functionName := "fail_member_schedule_reassignment_fn_" + strings.ToLower(rootID)
	triggerCreated := false
	t.Cleanup(func() {
		if triggerCreated {
			_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON schedules`, triggerName))
			_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, functionName))
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id=ANY($1::text[])`, []string{rootID, childID})
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id=ANY($1::text[])`, []string{rootID, childID})
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, memberID})
	})
	fn := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.id='%s' AND NEW.assignee_id='%s' THEN RAISE EXCEPTION 'injected Schedule reassignment failure'; END IF; RETURN NEW; END $$`, functionName, rootID, ownerID)
	if _, err := pool.Exec(ctx, fn); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE ON schedules FOR EACH ROW EXECUTE FUNCTION %s()`, triggerName, functionName)); err != nil {
		t.Fatal(err)
	}
	triggerCreated = true
	store := application.NewStore(pool)
	uow := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	remove := projectusecase.NewDeleteProjectMemberUseCase(uow, nil)
	if err := remove.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), projectdomain.UserID(memberID)); err == nil || !strings.Contains(err.Error(), "injected Schedule reassignment failure") {
		t.Fatalf("member removal error = %v; want injected Schedule failure", err)
	}
	var taskAssignee, rootAssignee, childAssignee string
	var memberExists bool
	if err := pool.QueryRow(ctx, `SELECT assignee_id FROM tasks WHERE id=$1`, taskID).Scan(&taskAssignee); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT assignee_id FROM schedules WHERE id=$1`, rootID).Scan(&rootAssignee); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT assignee_id FROM schedules WHERE id=$1`, childID).Scan(&childAssignee); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=$1 AND user_id=$2)`, projectID, memberID).Scan(&memberExists); err != nil {
		t.Fatal(err)
	}
	if taskAssignee != memberID || rootAssignee != memberID || childAssignee != memberID || !memberExists {
		t.Fatalf("failed reassignment partially committed: Task=%q root=%q child=%q member=%t", taskAssignee, rootAssignee, childAssignee, memberExists)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER %s ON schedules`, triggerName)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP FUNCTION %s()`, functionName)); err != nil {
		t.Fatal(err)
	}
	triggerCreated = false
	if err := remove.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), projectdomain.UserID(memberID)); err != nil {
		t.Fatalf("remove member after removing fault trigger: %v", err)
	}
	for _, row := range []struct{ table, id string }{{"tasks", taskID}, {"schedules", rootID}, {"schedules", childID}} {
		var assignee, changedBy string
		var revision int32
		query := fmt.Sprintf(`SELECT assignee_id,changed_by,revision FROM %s WHERE id=$1`, row.table)
		if err := pool.QueryRow(ctx, query, row.id).Scan(&assignee, &changedBy, &revision); err != nil {
			t.Fatal(err)
		}
		if assignee != ownerID || changedBy != ownerID || revision != 2 {
			t.Errorf("reassigned %s %s assignee=%q actor=%q revision=%d; want owner/owner/2", row.table, row.id, assignee, changedBy, revision)
		}
	}
}

func TestProjectMemberRemovalRacesCannotLeaveRemovedAssignee(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, memberID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	taskID, scheduleID := ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, memberID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Member race',$2,'unused','UTC')`, id, id+"@project-flow.test"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Member assignment race','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'viewer',$3)`, projectID, memberID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,changed_by) VALUES($1,$2,$3,$2,'Race task',$2)`, taskID, ownerID, projectID); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,repeat_state,changed_by) VALUES($1,$2,$3,$2,'Race schedule',$4,$5,$1,'2026-10-07','UTC','one_off',$2)`, scheduleID, ownerID, projectID, start, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, memberID})
	})
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule)
	projectUOW := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	startGate := make(chan struct{})
	assignTask := make(chan error, 1)
	assignSchedule := make(chan error, 1)
	removeMember := make(chan error, 1)
	go func() {
		<-startGate
		_, err := taskusecase.NewAssignTaskUseCase(taskUOW, nil).Execute(raceCtx, taskdomain.UserID(ownerID), taskdomain.TaskID(taskID), taskdomain.UserID(memberID), 1)
		assignTask <- err
	}()
	go func() {
		<-startGate
		assignSchedule <- scheduleusecase.NewAssignScheduleUseCase(scheduleUOW, nil).Execute(raceCtx, scheduledomain.UserID(ownerID), scheduledomain.ScheduleID(scheduleID), scheduledomain.UserID(memberID))
	}()
	go func() {
		<-startGate
		removeMember <- projectusecase.NewDeleteProjectMemberUseCase(projectUOW, nil).Execute(raceCtx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), projectdomain.UserID(memberID))
	}()
	close(startGate)
	var taskErr, scheduleErr, removeErr error
	for received := 0; received < 3; received++ {
		select {
		case taskErr = <-assignTask:
		case scheduleErr = <-assignSchedule:
		case removeErr = <-removeMember:
		case <-raceCtx.Done():
			t.Fatalf("Project member removal/assignment race timed out: %v", raceCtx.Err())
		}
	}
	if removeErr != nil {
		t.Fatalf("remove member during assignments: %v", removeErr)
	}
	if taskErr != nil && !errors.Is(taskErr, taskusecase.ErrPermissionDenied) {
		t.Fatalf("Task assignment race error = %v; want success or permission denied", taskErr)
	}
	if scheduleErr != nil && !errors.Is(scheduleErr, scheduleusecase.ErrScheduleAssigneeNotEligible) && !errors.Is(scheduleErr, scheduleusecase.ErrPermissionDenied) && !errors.Is(scheduleErr, scheduledomain.ErrScheduleNotFound) {
		t.Fatalf("Schedule assignment race error = %v; want success or not eligible/permission denied", scheduleErr)
	}
	var taskAssignee, scheduleAssignee string
	if err := pool.QueryRow(ctx, `SELECT assignee_id FROM tasks WHERE id=$1`, taskID).Scan(&taskAssignee); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT assignee_id FROM schedules WHERE id=$1`, scheduleID).Scan(&scheduleAssignee); err != nil {
		t.Fatal(err)
	}
	var memberExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=$1 AND user_id=$2)`, projectID, memberID).Scan(&memberExists); err != nil {
		t.Fatal(err)
	}
	if taskAssignee != ownerID || scheduleAssignee != ownerID || memberExists {
		t.Fatalf("removed member remains assigned: Task=%q Schedule=%q member=%t (assign errors Task=%v Schedule=%v)", taskAssignee, scheduleAssignee, memberExists, taskErr, scheduleErr)
	}
}

func TestProjectDeletionSoftDeletesTaskAndScheduleChildren(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID := ulid.Make().String(), ulid.Make().String()
	taskID, todoID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Delete',$2,'unused','UTC')`, ownerID, ownerID+"@project-flow.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Delete all children','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,changed_by) VALUES($1,$2,$3,$2,'Child task',$2)`, taskID, ownerID, projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO todo_items(id,task_id,title,position,series_id,occurrence_date,timezone) VALUES($1,$2,'Child todo',0,$1,'2026-10-07','UTC')`, todoID, taskID); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,repeat_state,changed_by) VALUES($1,$2,$3,$2,'Child schedule',$4,$5,$1,'2026-10-07','UTC','one_off',$2)`, scheduleID, ownerID, projectID, start, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	uow := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	if err := projectusecase.NewDeleteProjectUseCase(uow, nil).Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), 1); err != nil {
		t.Fatalf("delete Project: %v", err)
	}
	for _, row := range []struct{ table, id string }{{"projects", projectID}, {"tasks", taskID}, {"todo_items", todoID}, {"schedules", scheduleID}} {
		var deleted bool
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT deleted_at IS NOT NULL FROM %s WHERE id=$1`, row.table), row.id).Scan(&deleted); err != nil {
			t.Fatal(err)
		}
		if !deleted {
			t.Errorf("%s %s remained live after Project deletion", row.table, row.id)
		}
	}
	var taskRevision, scheduleRevision, projectRevision int32
	var taskActor, scheduleActor, projectActor string
	if err := pool.QueryRow(ctx, `SELECT revision,changed_by FROM tasks WHERE id=$1`, taskID).Scan(&taskRevision, &taskActor); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT revision,changed_by FROM schedules WHERE id=$1`, scheduleID).Scan(&scheduleRevision, &scheduleActor); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT revision,changed_by FROM projects WHERE id=$1`, projectID).Scan(&projectRevision, &projectActor); err != nil {
		t.Fatal(err)
	}
	if taskRevision != 2 || scheduleRevision != 2 || projectRevision != 2 || taskActor != ownerID || scheduleActor != ownerID || projectActor != ownerID {
		t.Fatalf("delete revisions Task=%d/%q Schedule=%d/%q Project=%d/%q; want revision 2 attributed to owner", taskRevision, taskActor, scheduleRevision, scheduleActor, projectRevision, projectActor)
	}
}

func TestProjectProgressCombinesPerTaskFloorsAndScheduleOccurrencesFromDB(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID := ulid.Make().String(), ulid.Make().String()
	taskIDs := []string{ulid.Make().String(), ulid.Make().String()}
	scheduleID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Progress',$2,'unused','UTC')`, ownerID, ownerID+"@project-flow.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Progress project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	for i, id := range taskIDs {
		if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,changed_by) VALUES($1,$2,$3,$2,$4,$2)`, id, ownerID, projectID, fmt.Sprintf("Task %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	insertTodo := func(id, taskID string, position int, completed bool) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO todo_items(id,task_id,title,position,series_id,occurrence_date,timezone,completed) VALUES($1,$2,$3,$4,$1,'2026-10-07','UTC',$5)`, id, taskID, "Todo", position, completed); err != nil {
			t.Fatal(err)
		}
	}
	insertTodo(ulid.Make().String(), taskIDs[0], 0, true)
	insertTodo(ulid.Make().String(), taskIDs[0], 1, false)
	insertTodo(ulid.Make().String(), taskIDs[0], 2, false)
	insertTodo(ulid.Make().String(), taskIDs[1], 0, true)
	insertTodo(ulid.Make().String(), taskIDs[1], 1, false)
	start := time.Now().UTC().Truncate(time.Minute)
	if _, err := pool.Exec(ctx, `INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,repeat_state,completed,changed_by) VALUES($1,$2,$3,$2,'Completed occurrence',$4,$5,$1,$6::date,'UTC','one_off',true,$2)`, scheduleID, ownerID, projectID, start, start.Add(time.Hour), start.Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=ANY($1::text[])`, taskIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=ANY($1::text[])`, taskIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	getProject := projectusecase.NewGetProjectUseCase(store.Project.Projects, store.Project.Projects, nil)
	got, err := getProject.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID))
	if err != nil {
		t.Fatalf("get Project with derived progress: %v", err)
	}
	if got.Progress != 61 {
		t.Fatalf("Project progress = %d; want floor((33 + 50 + 100)/3) = 61", got.Progress)
	}
}

func TestProjectProgressUsesScheduleWallTimeAcrossDSTGap(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','DST',$2,'unused','America/New_York')`, ownerID, ownerID+"@project-dst.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','DST progress','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 3, 1, 2, 30, 0, 0, time.FixedZone("EST", -5*60*60))
	if _, err := pool.Exec(ctx, `INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,repeat_state,frequency_anchor_date,interval_weeks,completed,changed_by) VALUES($1,$2,$3,$2,'Sunday at 2:30',$4,$5,$1,'2026-03-01','America/New_York','active','2026-03-01',1,true,$2)`, scheduleID, ownerID, projectID, start, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schedule_frequencies(schedule_id,frequency) VALUES($1,'sun')`, scheduleID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})

	readProgress := func(asOf time.Time) int {
		t.Helper()
		sources, err := projectrepo.NewProjectRepository(pool).ReadProjectProgressSources(ctx, []string{projectID}, asOf)
		if err != nil || len(sources.Schedules) != 1 {
			t.Fatalf("read Project progress at %s: sources=%+v error=%v", asOf, sources, err)
		}
		schedule := sources.Schedules[0]
		facts := projectdomain.ScheduleProgressFacts{Total: schedule.Total, Completed: schedule.Completed}
		for _, root := range schedule.Roots {
			facts.Roots = append(facts.Roots, sharedprogress.RecurrenceRule{
				OccurrenceDate: root.OccurrenceDate, Timezone: root.Timezone, IntervalWeeks: root.IntervalWeeks,
				FrequencyAnchorDate: root.FrequencyAnchorDate, Frequencies: root.Frequencies,
				OccurrenceSavedToday: root.OccurrenceSavedToday, StartAt: root.StartAt, EndAt: root.EndAt,
			})
		}
		progress, err := projectdomain.CalculateProjectProgress(nil, []projectdomain.ScheduleProgressFacts{facts}, asOf)
		if err != nil {
			t.Fatalf("calculate Project progress at %s: %v", asOf, err)
		}
		return progress
	}
	if got := readProgress(time.Date(2026, 3, 8, 7, 30, 0, 0, time.UTC)); got != 100 {
		t.Fatalf("progress on nonexistent 02:30 DST wall time = %d; want 100 with virtual occurrence excluded", got)
	}
	if got := readProgress(time.Date(2026, 3, 15, 6, 30, 0, 0, time.UTC)); got != 50 {
		t.Fatalf("progress on valid recurring Sunday = %d; want completed root plus one virtual occurrence", got)
	}
}

func TestProjectTaskAndScheduleCreationRaceWithDeletionLeavesNoLiveChildren(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, editorID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, editorID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Race',$2,'unused','UTC')`, id, id+"@project-flow.test"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Create/delete race','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3)`, projectID, editorID, ownerID); err != nil {
		t.Fatal(err)
	}
	taskID, scheduleID := ulid.Make().String(), ulid.Make().String()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id=$1`, scheduleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, editorID})
	})
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule)
	projectUOW := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	taskIDValue, scheduleIDValue := taskdomain.TaskID(taskID), scheduledomain.ScheduleID(scheduleID)
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	raceCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	startGate := make(chan struct{})
	createTask := make(chan error, 1)
	createSchedule := make(chan error, 1)
	deleteProject := make(chan error, 1)
	go func() {
		<-startGate
		_, err := taskusecase.NewCreateTaskInProjectUseCase(taskUOW, nil).Execute(raceCtx, taskusecase.CreateTaskInProjectInput{ID: taskIDValue, UserID: taskdomain.UserID(editorID), ProjectID: taskdomain.ProjectID(projectID), Title: "Racing Task"})
		createTask <- err
	}()
	go func() {
		<-startGate
		_, err := scheduleusecase.NewCreateScheduleUseCase(scheduleUOW, projectAuditTimezoneReader{}, nil).Execute(raceCtx, scheduleusecase.CreateScheduleInput{ID: scheduleIDValue, UserID: scheduledomain.UserID(editorID), ProjectID: scheduledomain.ProjectID(projectID), Title: "Racing Schedule", StartAt: start, EndAt: start.Add(time.Hour)})
		createSchedule <- err
	}()
	go func() {
		<-startGate
		deleteProject <- projectusecase.NewDeleteProjectUseCase(projectUOW, nil).Execute(raceCtx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), 1)
	}()
	close(startGate)
	var taskErr, scheduleErr, deleteErr error
	for received := 0; received < 3; received++ {
		select {
		case taskErr = <-createTask:
		case scheduleErr = <-createSchedule:
		case deleteErr = <-deleteProject:
		case <-raceCtx.Done():
			t.Fatalf("Project creation/deletion race exceeded deadline: %v", raceCtx.Err())
		}
	}
	if deleteErr != nil {
		t.Fatalf("delete Project during child creation: %v", deleteErr)
	}
	_ = taskErr
	_ = scheduleErr
	var activeTasks, activeSchedules int32
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM tasks WHERE project_id=$1 AND deleted_at IS NULL`, projectID).Scan(&activeTasks); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM schedules WHERE project_id=$1 AND deleted_at IS NULL`, projectID).Scan(&activeSchedules); err != nil {
		t.Fatal(err)
	}
	if activeTasks != 0 || activeSchedules != 0 {
		t.Fatalf("deleted Project retains live children: Tasks=%d Schedules=%d (create errors Task=%v Schedule=%v)", activeTasks, activeSchedules, taskErr, scheduleErr)
	}
}

func TestProjectDeletionCascadesTodoCreatedWhileWaitingForTaskLock(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, editorID, projectID, taskID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, editorID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Todo race',$2,'unused','UTC')`, id, id+"@project-flow.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Todo create/delete race','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3)`, projectID, editorID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,changed_by) VALUES($1,$2,$3,$2,'Todo race task',$2)`, taskID, ownerID, projectID); err != nil {
		t.Fatal(err)
	}
	todoID := ulid.Make().String()
	functionName := "project_pause_todo_" + strings.ToLower(todoID)
	triggerName := "project_pause_todo_trigger_" + strings.ToLower(todoID)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(hashtext(NEW.task_id));
			RETURN NEW;
		END;
		$$
	`, functionName)); err != nil {
		t.Fatalf("create Todo insert synchronization function: %v", err)
	}
	triggerCreated := true
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if triggerCreated {
			_, _ = pool.Exec(cleanupCtx, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON todo_items`, triggerName))
			_, _ = pool.Exec(cleanupCtx, fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, functionName))
		}
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM todo_items WHERE id=$1`, todoID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, editorID})
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON todo_items FOR EACH ROW EXECUTE FUNCTION %s()`, triggerName, functionName)); err != nil {
		t.Fatalf("create Todo insert synchronization trigger: %v", err)
	}

	lockConnection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire advisory-lock connection: %v", err)
	}
	lockHeld := true
	defer func() {
		if lockHeld {
			_, _ = lockConnection.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, taskID)
		}
		lockConnection.Release()
	}()
	if _, err := lockConnection.Exec(ctx, `SELECT pg_advisory_lock(hashtext($1))`, taskID); err != nil {
		t.Fatalf("hold Todo insert advisory lock: %v", err)
	}

	childApp, deleteApp := "project-todo-child-"+taskID, "project-todo-delete-"+taskID
	childPool := projectRaceNamedPool(t, childApp)
	deletePool := projectRaceNamedPool(t, deleteApp)
	childStore, deleteStore := application.NewStore(childPool), application.NewStore(deletePool)
	create := taskusecase.NewCreateTodoItemUseCase(application.NewTaskUOW(childPool, childStore.Task), projectTaskAuditTimezoneReader{}, nil)
	remove := projectusecase.NewDeleteProjectUseCase(application.NewProjectUOW(deletePool, deleteStore.Project, deleteStore.Task, deleteStore.Schedule), nil)
	raceCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	createResult := make(chan error, 1)
	go func() {
		_, err := create.Execute(raceCtx, taskusecase.CreateTodoItemInput{
			ID: taskdomain.TodoItemID(todoID), UserID: taskdomain.UserID(editorID), TaskID: taskdomain.TaskID(taskID), Title: "Created during Project deletion",
		})
		createResult <- err
	}()
	waitForProjectApplicationLockWait(t, pool, childApp)

	deleteResult := make(chan error, 1)
	go func() {
		deleteResult <- remove.Execute(raceCtx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), 1)
	}()
	waitForProjectApplicationLockWait(t, pool, deleteApp)
	if _, err := lockConnection.Exec(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, taskID); err != nil {
		t.Fatalf("release Todo insert advisory lock: %v", err)
	}
	lockHeld = false

	for _, operation := range []struct {
		name string
		ch   <-chan error
	}{{"Todo creation", createResult}, {"Project deletion", deleteResult}} {
		select {
		case err := <-operation.ch:
			if err != nil {
				t.Fatalf("%s during Todo cascade race: %v", operation.name, err)
			}
		case <-raceCtx.Done():
			t.Fatalf("%s did not finish before race deadline: %v", operation.name, raceCtx.Err())
		}
	}
	for table, id := range map[string]string{"tasks": taskID, "todo_items": todoID} {
		var deleted bool
		if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM `+table+` WHERE id=$1`, id).Scan(&deleted); err != nil {
			t.Fatalf("read %s after Project delete/Todo create race: %v", table, err)
		}
		if !deleted {
			t.Errorf("%s %s remains active after Project deletion", table, id)
		}
	}
}

func projectRaceNamedPool(t *testing.T, applicationName string) *pgxpool.Pool {
	return testdb.OpenWithApplicationName(t, applicationName)
}

func waitForProjectApplicationLockWait(t *testing.T, pool *pgxpool.Pool, applicationName string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock')`, applicationName).Scan(&waiting); err != nil {
			t.Fatalf("check %s lock wait: %v", applicationName, err)
		}
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("%s did not reach a database lock wait: %v", applicationName, ctx.Err())
		case <-ticker.C:
		}
	}
}

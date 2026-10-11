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
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

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
	taskID, blueprintID := ulid.Make().String(), ""
	t.Cleanup(func() {
		_ = testdb.DeleteScheduleFixture(context.Background(), pool, blueprintID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, actorID})
	})

	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project, store.Schedule)
	task, err := taskusecase.NewCreateTaskInProjectUseCase(taskUOW, nil).Execute(ctx, taskusecase.CreateTaskInProjectInput{
		ID: taskdomain.TaskID(taskID), UserID: taskdomain.UserID(actorID), ProjectID: taskdomain.ProjectID(projectID), Title: "Created by editor",
	})
	if err != nil {
		t.Fatalf("create shared Task: %v", err)
	}
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project)
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	schedule, err := scheduleusecase.NewCreateBlueprintUseCase(scheduleUOW, (projectActionItemID{}).Generate, store.Auth.Users, nil).Execute(ctx, actorID, scheduleusecase.CreateBlueprintInput{
		Title: "Created by editor", ProjectID: projectID, Timezone: "UTC", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "once",
	})
	if err != nil {
		t.Fatalf("create shared Schedule: %v", err)
	}
	blueprintID = schedule.ID
	var scheduleAssignee, scheduleActor string
	var scheduleRevision int32
	if err := pool.QueryRow(ctx, `SELECT assignee_id,changed_by,revision FROM schedules WHERE blueprint_id=$1`, schedule.ID).Scan(&scheduleAssignee, &scheduleActor, &scheduleRevision); err != nil {
		t.Fatalf("read created Schedule occurrence: %v", err)
	}
	for _, got := range []struct {
		name, owner, assignee, actor string
		revision                     int32
	}{
		{"Task", task.UserID, task.AssigneeID, task.ChangedBy, task.Revision},
		{"Schedule", schedule.UserID, scheduleAssignee, scheduleActor, scheduleRevision},
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
	rootBlueprintID, childBlueprintID := ulid.Make().String(), ulid.Make().String()
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
	for _, fixture := range []testdb.ScheduleFixture{
		{BlueprintID: rootBlueprintID, ScheduleID: rootID, UserID: ownerID, ProjectID: projectID, AssigneeID: memberID, Title: "Member schedule", Timezone: "UTC", StartAt: start, EndAt: start.Add(time.Hour), OccurrenceDate: "2026-10-07"},
		{BlueprintID: childBlueprintID, ScheduleID: childID, UserID: ownerID, ProjectID: projectID, AssigneeID: memberID, Title: "Saved deleted occurrence", Timezone: "UTC", StartAt: start.AddDate(0, 0, 1), EndAt: start.AddDate(0, 0, 1).Add(time.Hour), OccurrenceDate: "2026-10-08", Deleted: true},
	} {
		if err := testdb.InsertScheduleFixture(ctx, pool, fixture); err != nil {
			t.Fatalf("insert Schedule fixture: %v", err)
		}
	}
	triggerName := "fail_member_schedule_reassignment_" + strings.ToLower(rootID)
	functionName := "fail_member_schedule_reassignment_fn_" + strings.ToLower(rootID)
	triggerCreated := false
	t.Cleanup(func() {
		if triggerCreated {
			_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON schedules`, triggerName))
			_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, functionName))
		}
		_ = testdb.DeleteScheduleFixture(context.Background(), pool, rootBlueprintID)
		_ = testdb.DeleteScheduleFixture(context.Background(), pool, childBlueprintID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
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
	taskID, scheduleID, blueprintID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
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
	start := time.Now().UTC().AddDate(0, 0, 2).Truncate(time.Hour)
	occurrenceDate := start.Format("2006-01-02")
	if err := testdb.InsertScheduleFixture(ctx, pool, testdb.ScheduleFixture{BlueprintID: blueprintID, ScheduleID: scheduleID, UserID: ownerID, ProjectID: projectID, Title: "Race schedule", StartAt: start, EndAt: start.Add(time.Hour), OccurrenceDate: occurrenceDate}); err != nil {
		t.Fatalf("insert Schedule fixture: %v", err)
	}
	t.Cleanup(func() {
		_ = testdb.DeleteScheduleFixture(context.Background(), pool, blueprintID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, memberID})
	})
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project, store.Schedule)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project)
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
		member := memberID
		_, err := scheduleusecase.NewUpdateOccurrenceUseCase(scheduleUOW, nil, (projectActionItemID{}).Generate).Execute(raceCtx, ownerID, blueprintID, occurrenceDate, scheduleusecase.UpdateScheduleInput{AssigneeID: &member}, 1)
		assignSchedule <- err
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
	if scheduleErr != nil && !errors.Is(scheduleErr, scheduleusecase.ErrScheduleAssigneeNotEligible) && !errors.Is(scheduleErr, scheduleusecase.ErrPermissionDenied) && !errors.Is(scheduleErr, scheduleusecase.ErrOccurrenceNotFound) && !errors.Is(scheduleErr, scheduleusecase.ErrOccurrenceRevisionMismatch) {
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

func TestProjectDeletionLeavesTaskAndScheduleChildrenUntouched(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID := ulid.Make().String(), ulid.Make().String()
	taskID, actionItemID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	blueprintID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Delete',$2,'unused','UTC')`, ownerID, ownerID+"@project-flow.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Delete all children','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,changed_by) VALUES($1,$2,$3,$2,'Child task',$2)`, taskID, ownerID, projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO action_items(id,task_id,title,position) VALUES($1,$2,'Child actionItem',0)`, actionItemID, taskID); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	if err := testdb.InsertScheduleFixture(ctx, pool, testdb.ScheduleFixture{BlueprintID: blueprintID, ScheduleID: scheduleID, UserID: ownerID, ProjectID: projectID, Title: "Child schedule", OccurrenceDate: "2026-10-07", StartAt: start, EndAt: start.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = testdb.DeleteScheduleFixture(context.Background(), pool, blueprintID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	uow := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	if err := projectusecase.NewDeleteProjectUseCase(uow, nil).Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), 1); err != nil {
		t.Fatalf("delete Project: %v", err)
	}
	for _, row := range []struct {
		table string
		id    string
		want  bool
	}{{"projects", projectID, true}, {"tasks", taskID, false}, {"action_items", actionItemID, false}, {"schedules", scheduleID, false}} {
		var deleted bool
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT deleted_at IS NOT NULL FROM %s WHERE id=$1`, row.table), row.id).Scan(&deleted); err != nil {
			t.Fatal(err)
		}
		if deleted != row.want {
			t.Errorf("%s %s deleted=%t after Project deletion; want %t", row.table, row.id, deleted, row.want)
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
	if taskRevision != 1 || scheduleRevision != 1 || projectRevision != 2 || taskActor != ownerID || scheduleActor != ownerID || projectActor != ownerID {
		t.Fatalf("delete revisions Task=%d/%q Schedule=%d/%q Project=%d/%q; want unchanged child revisions 1 and parent revision 2 attributed to owner", taskRevision, taskActor, scheduleRevision, scheduleActor, projectRevision, projectActor)
	}
}

func TestProjectProgressCombinesPerTaskFloorsAndScheduleOccurrencesFromDB(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, memberID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	taskIDs := []string{ulid.Make().String(), ulid.Make().String(), ulid.Make().String()}
	scheduleID, blueprintID := ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, memberID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Progress',$2,'unused','UTC')`, id, id+"@project-flow.test"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Progress project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3)`, projectID, memberID, ownerID); err != nil {
		t.Fatal(err)
	}
	for i, id := range taskIDs {
		status, assigneeID := "open", ownerID
		if i == 1 {
			assigneeID = memberID
		}
		if i == 2 {
			status = "done"
		}
		if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,status,changed_by) VALUES($1,$2,$3,$4,$5,$6,$2)`, id, ownerID, projectID, assigneeID, fmt.Sprintf("Task %d", i), status); err != nil {
			t.Fatal(err)
		}
	}
	insertActionItem := func(id, taskID string, position int, completed bool) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO action_items(id,task_id,title,position,completed) VALUES($1,$2,$3,$4,$5)`, id, taskID, "ActionItem", position, completed); err != nil {
			t.Fatal(err)
		}
	}
	insertActionItem(ulid.Make().String(), taskIDs[0], 0, true)
	insertActionItem(ulid.Make().String(), taskIDs[0], 1, false)
	insertActionItem(ulid.Make().String(), taskIDs[0], 2, false)
	deletedActionItemID := ulid.Make().String()
	insertActionItem(deletedActionItemID, taskIDs[0], 3, true)
	if _, err := pool.Exec(ctx, `UPDATE action_items SET deleted_at=now() WHERE id=$1`, deletedActionItemID); err != nil {
		t.Fatal(err)
	}
	insertActionItem(ulid.Make().String(), taskIDs[1], 0, true)
	insertActionItem(ulid.Make().String(), taskIDs[1], 1, false)
	start := time.Now().UTC().Truncate(time.Minute)
	if err := testdb.InsertScheduleFixture(ctx, pool, testdb.ScheduleFixture{BlueprintID: blueprintID, ScheduleID: scheduleID, UserID: ownerID, ProjectID: projectID, Title: "Completed occurrence", OccurrenceDate: start.Format("2006-01-02"), StartAt: start, EndAt: start.Add(time.Hour), Completed: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = testdb.DeleteScheduleFixture(context.Background(), pool, blueprintID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=ANY($1::text[])`, taskIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=ANY($1::text[])`, taskIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	getProject := newProjectUseCases(t, pool, store).Get
	got, err := getProject.Execute(ctx, projectdomain.UserID(memberID), projectdomain.ProjectID(projectID))
	if err != nil {
		t.Fatalf("get Project with derived progress as a member: %v", err)
	}
	if got.Progress != 70 {
		t.Fatalf("Project progress = %d; want floor((33 + 50 + 100 + 100)/4) = 70", got.Progress)
	}
}

func TestProjectProgressUsesOnlySavedScheduleOccurrences(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID, scheduleID, blueprintID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','DST',$2,'unused','America/New_York')`, ownerID, ownerID+"@project-dst.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','DST progress','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(location)
	occurrenceDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	rootDate := occurrenceDate.AddDate(0, 0, -7)
	weekday := strings.ToLower(rootDate.Weekday().String())[:3]
	start := time.Date(rootDate.Year(), rootDate.Month(), rootDate.Day(), 12, 0, 0, 0, location)
	end := start.Add(time.Hour)
	if err := testdb.InsertScheduleFixture(ctx, pool, testdb.ScheduleFixture{BlueprintID: blueprintID, ScheduleID: scheduleID, UserID: ownerID, ProjectID: projectID, Title: "Weekly Schedule", Timezone: location.String(), Frequency: "weekly", Interval: 1, Weekdays: []string{weekday}, AnchorDate: rootDate.Format("2006-01-02"), OccurrenceDate: rootDate.Format("2006-01-02"), StartAt: start, EndAt: end, Completed: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = testdb.DeleteScheduleFixture(context.Background(), pool, blueprintID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})

	store := application.NewStore(pool)
	projectCases := newProjectUseCases(t, pool, store)
	project, err := projectCases.Get.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID))
	if err != nil || project.Progress != 100 {
		t.Fatalf("Project progress with only the completed recurrence root saved = %d, error=%v; want 100%%", project.Progress, err)
	}

	backfillUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project)
	cutoff := time.Date(occurrenceDate.Year(), occurrenceDate.Month(), occurrenceDate.Day(), 12, 1, 0, 0, location)
	_, generated, err := scheduleusecase.NewBackfillUseCase(backfillUOW, (projectActionItemID{}).Generate).Execute(ctx, ownerID, cutoff, nil)
	if err != nil || generated != 1 {
		t.Fatalf("backfill today's recurring Schedule generated %d occurrences, error=%v; want one saved occurrence", generated, err)
	}
	if status, _ := readProjectState(t, pool, projectID); status != "open" {
		t.Fatalf("Project status after Schedule backfill = %q; want unchanged open status", status)
	}
	project, err = projectCases.Get.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID))
	if err != nil || project.Progress != 50 {
		t.Fatalf("Project progress after saving today's incomplete recurrence = %d, error=%v; want 50%%", project.Progress, err)
	}
}

func TestProjectTaskAndScheduleCreationRaceWithParentOnlyDeletion(t *testing.T) {
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
	taskID, scheduleBlueprintID := ulid.Make().String(), ""
	t.Cleanup(func() {
		_ = testdb.DeleteScheduleFixture(context.Background(), pool, scheduleBlueprintID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, editorID})
	})
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project, store.Schedule)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project)
	projectUOW := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	taskIDValue := taskdomain.TaskID(taskID)
	start := time.Now().UTC().AddDate(0, 0, 2).Truncate(time.Hour)
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
		created, err := scheduleusecase.NewCreateBlueprintUseCase(scheduleUOW, (projectActionItemID{}).Generate, store.Auth.Users, nil).Execute(raceCtx, editorID, scheduleusecase.CreateBlueprintInput{Title: "Racing Schedule", ProjectID: projectID, Timezone: "UTC", LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "once"})
		if err == nil {
			scheduleBlueprintID = created.ID
		}
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
	if taskErr != nil && !errors.Is(taskErr, taskusecase.ErrTaskProjectNotFound) {
		t.Fatalf("Task creation race error = %v; want success or deleted Project", taskErr)
	}
	if scheduleErr != nil && !errors.Is(scheduleErr, scheduleusecase.ErrScheduleProjectNotFound) {
		t.Fatalf("Schedule creation race error = %v; want success or deleted Project", scheduleErr)
	}
	var projectDeleted bool
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM projects WHERE id=$1`, projectID).Scan(&projectDeleted); err != nil {
		t.Fatal(err)
	}
	if !projectDeleted {
		t.Fatal("Project is live after successful deletion")
	}
	var activeTasks, activeSchedules int32
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM tasks WHERE project_id=$1 AND deleted_at IS NULL`, projectID).Scan(&activeTasks); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM schedules WHERE project_id=$1 AND deleted_at IS NULL`, projectID).Scan(&activeSchedules); err != nil {
		t.Fatal(err)
	}
	wantTasks, wantSchedules := int32(0), int32(0)
	if taskErr == nil {
		wantTasks = 1
	}
	if scheduleErr == nil {
		wantSchedules = 1
	}
	if activeTasks != wantTasks || activeSchedules != wantSchedules {
		t.Fatalf("parent-only delete race child states: Tasks=%d/%d Schedules=%d/%d (create errors Task=%v Schedule=%v)", activeTasks, wantTasks, activeSchedules, wantSchedules, taskErr, scheduleErr)
	}
}

func TestProjectDeletionLeavesActionItemCreatedBeforeParentLockUntouched(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, editorID, projectID, taskID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, id := range []string{ownerID, editorID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','ActionItem race',$2,'unused','UTC')`, id, id+"@project-flow.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','ActionItem create/delete race','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3)`, projectID, editorID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,changed_by) VALUES($1,$2,$3,$2,'ActionItem race task',$2)`, taskID, ownerID, projectID); err != nil {
		t.Fatal(err)
	}
	actionItemID := ulid.Make().String()
	functionName := "project_pause_actionItem_" + strings.ToLower(actionItemID)
	triggerName := "project_pause_actionItem_trigger_" + strings.ToLower(actionItemID)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(hashtext(NEW.task_id));
			RETURN NEW;
		END;
		$$
	`, functionName)); err != nil {
		t.Fatalf("create ActionItem insert synchronization function: %v", err)
	}
	triggerCreated := true
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if triggerCreated {
			_, _ = pool.Exec(cleanupCtx, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON action_items`, triggerName))
			_, _ = pool.Exec(cleanupCtx, fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, functionName))
		}
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM action_items WHERE id=$1`, actionItemID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, editorID})
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON action_items FOR EACH ROW EXECUTE FUNCTION %s()`, triggerName, functionName)); err != nil {
		t.Fatalf("create ActionItem insert synchronization trigger: %v", err)
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
		t.Fatalf("hold ActionItem insert advisory lock: %v", err)
	}

	childApp, deleteApp := "project-actionItem-child-"+taskID, "project-actionItem-delete-"+taskID
	childPool := projectRaceNamedPool(t, childApp)
	deletePool := projectRaceNamedPool(t, deleteApp)
	childStore, deleteStore := application.NewStore(childPool), application.NewStore(deletePool)
	create := taskusecase.NewCreateActionItemUseCase(application.NewTaskUOW(childPool, childStore.Task, childStore.Project, childStore.Schedule), nil)
	remove := projectusecase.NewDeleteProjectUseCase(application.NewProjectUOW(deletePool, deleteStore.Project, deleteStore.Task, deleteStore.Schedule), nil)
	raceCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	createResult := make(chan error, 1)
	go func() {
		_, err := create.Execute(raceCtx, taskusecase.CreateActionItemInput{
			ID: taskdomain.ActionItemID(actionItemID), UserID: taskdomain.UserID(editorID), TaskID: taskdomain.TaskID(taskID), Title: "Created during Project deletion",
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
		t.Fatalf("release ActionItem insert advisory lock: %v", err)
	}
	lockHeld = false

	for _, operation := range []struct {
		name string
		ch   <-chan error
	}{{"ActionItem creation", createResult}, {"Project deletion", deleteResult}} {
		select {
		case err := <-operation.ch:
			if err != nil {
				t.Fatalf("%s during parent-only Project deletion race: %v", operation.name, err)
			}
		case <-raceCtx.Done():
			t.Fatalf("%s did not finish before race deadline: %v", operation.name, raceCtx.Err())
		}
	}
	for table, id := range map[string]string{"projects": projectID, "tasks": taskID, "action_items": actionItemID} {
		var deleted bool
		if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM `+table+` WHERE id=$1`, id).Scan(&deleted); err != nil {
			t.Fatalf("read %s after parent-only Project delete/ActionItem create race: %v", table, err)
		}
		wantDeleted := table == "projects"
		if deleted != wantDeleted {
			t.Errorf("%s %s deleted=%t after Project deletion race; want %t", table, id, deleted, wantDeleted)
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

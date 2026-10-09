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
	scheduledomain "github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func projectCrossContextTestPool(t *testing.T) *pgxpool.Pool {
	return testdb.Open(t)
}

type projectActionItemID struct{}

func (projectActionItemID) Generate() string { return ulid.Make().String() }

type projectCrossContextTimezoneReader struct{}

func (projectCrossContextTimezoneReader) GetTimezone(context.Context, string) (string, error) {
	return "UTC", nil
}

func readProjectState(t *testing.T, pool *pgxpool.Pool, projectID string) (string, int32) {
	t.Helper()
	var status string
	var revision int32
	if err := pool.QueryRow(t.Context(), `SELECT status, revision FROM projects WHERE id=$1`, projectID).Scan(&status, &revision); err != nil {
		t.Fatalf("read Project status: %v", err)
	}
	return status, revision
}

func TestDoneProjectHidesActionItemVirtualsButKeepsSavedOccurrenceEditable(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID, taskID, actionItemID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Done',$2,'unused','UTC')`, ownerID, ownerID+"@project-done-actionItem.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Done Project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,priority,status,changed_by) VALUES($1,$2,$3,$2,'Recurring Task','low','open',$2)`, taskID, ownerID, projectID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM action_item_frequencies WHERE action_item_id IN (SELECT id FROM action_items WHERE task_id=$1)`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM action_items WHERE task_id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})

	monday, err := taskdomain.NewTaskFrequency("mon")
	if err != nil {
		t.Fatal(err)
	}
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	root, err := taskdomain.NewActionItemWithDetails(taskdomain.ActionItemID(actionItemID), taskdomain.TaskID(taskID), "Weekly work", "", rootDate, false, 0, 1, taskdomain.TaskFrequencies{monday})
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.WithRecurrence(taskdomain.RecurrenceMetadata{SeriesID: actionItemID, OccurrenceDate: rootDate, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project)
	if _, err := taskrepo.NewActionItemRepository(pool).CreateForOwnedTask(ctx, taskdomain.UserID(ownerID), root, false); err != nil {
		t.Fatalf("create recurring root: %v", err)
	}
	complete := taskusecase.NewCompleteActionItemUseCase(taskUOW, nil, projectActionItemID{})
	if err := complete.ExecuteOccurrence(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(taskID), taskdomain.ActionItemID(actionItemID), "2026-10-12"); err != nil {
		t.Fatalf("save a future occurrence before completing Project: %v", err)
	}
	projectUOW := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	if _, err := projectusecase.NewChangeProjectStatusUseCase(projectUOW, store.Project.Projects, nil).Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), 1, "done"); err != nil {
		t.Fatalf("mark Project done: %v", err)
	}

	list := taskusecase.NewListActionItemsUseCase(taskUOW, nil, func() time.Time { return time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC) })
	page, err := list.ExecutePage(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(taskID), taskusecase.CursorPageRequest{Size: 10, FromDate: "2026-10-07"})
	if err != nil {
		t.Fatalf("list ActionItems for done Project: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID == taskusecase.VirtualOccurrenceID || page.Items[0].OccurrenceDate != "2026-10-12" || !page.Items[0].Completed {
		t.Fatalf("done Project ActionItem page = %+v; want only the saved completed occurrence", page.Items)
	}

	title := "Saved work edited after Project completion"
	updated, err := taskusecase.NewUpdateActionItemUseCase(taskUOW, nil, projectActionItemID{}).ExecuteOccurrence(
		ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(taskID), taskdomain.ActionItemID(actionItemID), "2026-10-12", "current",
		taskusecase.PatchField[string]{Present: true, Value: &title}, taskusecase.PatchField[string]{}, taskusecase.PatchField[time.Time]{},
	)
	if err != nil || updated.Title != title {
		t.Fatalf("edit saved occurrence under done Project = %+v, error=%v", updated, err)
	}
	if err := complete.ExecuteOccurrence(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(taskID), taskdomain.ActionItemID(actionItemID), "2026-10-19"); !errors.Is(err, taskusecase.ErrOccurrenceInactive) {
		t.Fatalf("complete unsaved virtual occurrence under done Project = %v, want occurrence inactive", err)
	}
}

func TestProjectLifecycleTransitionsAcrossTaskAndScheduleMutations(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, actorID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	firstTaskID, firstScheduleID := ulid.Make().String(), ulid.Make().String()
	secondTaskID, secondScheduleID := ulid.Make().String(), ulid.Make().String()
	roleID := "project_child_only_" + strings.ToLower(ulid.Make().String())
	for _, id := range []string{ownerID, actorID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Lifecycle',$2,'unused','UTC')`, id, id+"@project-lifecycle.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Lifecycle Project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO roles(role_id,name) VALUES($1,$1)`, roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, projectID, actorID, roleID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO role_permissions(role_id,permission_id)
		SELECT $1, permission_id FROM permissions
		WHERE effect='allow' AND ((resource_id='task' AND action='update') OR (resource_id='schedule' AND action='update'))
	`, roleID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id=ANY($1::text[])`, []string{firstScheduleID, secondScheduleID})
		_, _ = pool.Exec(cleanup, `DELETE FROM task_revisions WHERE id=ANY($1::text[])`, []string{firstTaskID, secondTaskID})
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE id=ANY($1::text[])`, []string{firstScheduleID, secondScheduleID})
		_, _ = pool.Exec(cleanup, `DELETE FROM tasks WHERE id=ANY($1::text[])`, []string{firstTaskID, secondTaskID})
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM roles WHERE role_id=$1`, roleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, actorID})
	})
	var canUpdateProject, canUpdateTask, canUpdateSchedule bool
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project)
	projectUOW := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	task, err := taskusecase.NewCreateTaskInProjectUseCase(taskUOW, nil).Execute(ctx, taskusecase.CreateTaskInProjectInput{
		ID: taskdomain.TaskID(firstTaskID), UserID: taskdomain.UserID(ownerID), ProjectID: taskdomain.ProjectID(projectID), Title: "Initial Task",
	})
	if err != nil {
		t.Fatalf("create initial Task: %v", err)
	}
	_, err = scheduleusecase.NewCreateScheduleUseCase(scheduleUOW, projectCrossContextTimezoneReader{}, nil).Execute(ctx, scheduleusecase.CreateScheduleInput{
		ID: scheduledomain.ScheduleID(firstScheduleID), UserID: scheduledomain.UserID(ownerID), ProjectID: scheduledomain.ProjectID(projectID),
		Title: "Initial Schedule", StartAt: time.Now().UTC().Add(time.Hour), EndAt: time.Now().UTC().Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create initial Schedule: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT project_has_permission($1,$2,'project','update'),task_has_permission($3,$2,'task','update'),schedule_has_permission($4,$2,'schedule','update')`, projectID, actorID, firstTaskID, firstScheduleID).Scan(&canUpdateProject, &canUpdateTask, &canUpdateSchedule); err != nil {
		t.Fatal(err)
	}
	if canUpdateProject || !canUpdateTask || !canUpdateSchedule {
		t.Fatalf("custom role capabilities project.update=%t task.update=%t schedule.update=%t; want false,true,true", canUpdateProject, canUpdateTask, canUpdateSchedule)
	}
	_, err = taskusecase.NewCompleteTaskUseCase(taskUOW, time.Now, nil).Execute(ctx, taskdomain.UserID(actorID), taskdomain.TaskID(firstTaskID), task.Revision)
	if err != nil {
		t.Fatalf("complete child Task without project.update: %v", err)
	}
	if status, _ := readProjectState(t, pool, projectID); status != "open" {
		t.Fatalf("Project status after only Task completion = %q; want open while Schedule unfinished", status)
	}
	if err := scheduleusecase.NewCompleteScheduleUseCase(scheduleUOW, nil).Execute(ctx, scheduledomain.UserID(actorID), scheduledomain.ScheduleID(firstScheduleID)); err != nil {
		t.Fatalf("complete child Schedule without project.update: %v", err)
	}
	status, revision := readProjectState(t, pool, projectID)
	if status != "done" {
		t.Fatalf("Project status after all direct work completed = %q; want done", status)
	}
	projectRead := projectusecase.NewGetProjectUseCase(store.Project.Projects, store.Project.Projects, nil)
	project, err := projectRead.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID))
	if err != nil || project.Progress != 100 {
		t.Fatalf("completed Project progress = %d, error=%v; want 100", project.Progress, err)
	}
	projectStatus := projectusecase.NewChangeProjectStatusUseCase(projectUOW, store.Project.Projects, nil)
	reopened, err := projectStatus.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), revision, "open")
	if err != nil || reopened.Status != "open" || reopened.Progress != 100 {
		t.Fatalf("manual reopen = status %q progress %d error=%v; want open at 100%%", reopened.Status, reopened.Progress, err)
	}
	title := "Count-neutral Schedule edit"
	if _, err := scheduleusecase.NewUpdateScheduleUseCase(scheduleUOW, nil).Execute(ctx, scheduledomain.UserID(actorID), scheduledomain.ScheduleID(firstScheduleID), "current", scheduleusecase.PatchField[string]{Present: true, Value: &title}, scheduleusecase.PatchField[string]{}, scheduleusecase.PatchField[string]{}); err != nil {
		t.Fatalf("count-neutral edit with child-only capability: %v", err)
	}
	if status, _ := readProjectState(t, pool, projectID); status != "open" {
		t.Fatalf("count-neutral edit re-completed manually reopened Project: status=%q", status)
	}
	_, revision = readProjectState(t, pool, projectID)
	if _, err := projectStatus.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), revision, "done"); err != nil {
		t.Fatalf("mark Project done before adding Task: %v", err)
	}
	secondTask, err := taskusecase.NewCreateTaskInProjectUseCase(taskUOW, nil).Execute(ctx, taskusecase.CreateTaskInProjectInput{
		ID: taskdomain.TaskID(secondTaskID), UserID: taskdomain.UserID(ownerID), ProjectID: taskdomain.ProjectID(projectID), Title: "Added after completion",
	})
	if err != nil {
		t.Fatalf("add unfinished Task to done Project: %v", err)
	}
	if status, _ := readProjectState(t, pool, projectID); status != "open" {
		t.Fatalf("Project status after adding unfinished Task = %q; want open", status)
	}
	if _, err := taskusecase.NewCompleteTaskUseCase(taskUOW, time.Now, nil).Execute(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(secondTaskID), secondTask.Revision); err != nil {
		t.Fatalf("complete newly added Task: %v", err)
	}
	status, revision = readProjectState(t, pool, projectID)
	if status != "done" {
		t.Fatalf("Project status after completing newly added Task = %q; want done", status)
	}
	_, err = scheduleusecase.NewCreateScheduleUseCase(scheduleUOW, projectCrossContextTimezoneReader{}, nil).Execute(ctx, scheduleusecase.CreateScheduleInput{
		ID: scheduledomain.ScheduleID(secondScheduleID), UserID: scheduledomain.UserID(ownerID), ProjectID: scheduledomain.ProjectID(projectID),
		Title: "Added after completion", StartAt: time.Now().UTC().Add(3 * time.Hour), EndAt: time.Now().UTC().Add(4 * time.Hour),
	})
	if err != nil {
		t.Fatalf("add unfinished Schedule to done Project: %v", err)
	}
	if status, _ := readProjectState(t, pool, projectID); status != "open" {
		t.Fatalf("Project status after adding unfinished Schedule = %q; want open", status)
	}
	_, revision = readProjectState(t, pool, projectID)
	if err := projectusecase.NewDeleteProjectUseCase(projectUOW, nil).Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), revision); err != nil {
		t.Fatalf("trash Project: %v", err)
	}
	if _, err := taskusecase.NewReopenTaskUseCase(taskUOW, nil).Execute(ctx, taskdomain.UserID(actorID), taskdomain.TaskID(secondTaskID), 2); !errors.Is(err, taskusecase.ErrTaskNotFound) {
		t.Fatalf("reopen child Task after Project trash = %v; want hidden/not found", err)
	}
	if err := scheduleusecase.NewCompleteScheduleUseCase(scheduleUOW, nil).Execute(ctx, scheduledomain.UserID(actorID), scheduledomain.ScheduleID(secondScheduleID)); !errors.Is(err, scheduledomain.ErrScheduleNotFound) {
		t.Fatalf("complete child Schedule after Project trash = %v; want hidden/not found", err)
	}
	var taskStatus string
	var scheduleCompleted bool
	if err := pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, secondTaskID).Scan(&taskStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT completed FROM schedules WHERE id=$1`, secondScheduleID).Scan(&scheduleCompleted); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "done" || scheduleCompleted {
		t.Fatalf("child writes survived trashed-parent denial: Task=%q Schedule.completed=%t", taskStatus, scheduleCompleted)
	}
}

func TestOpenTaskAtFullActionItemProgressDoesNotCompleteProject(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID, taskID, actionItemID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Open task',$2,'unused','UTC')`, ownerID, ownerID+"@project-open-task.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Open Task Project','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,status,changed_by) VALUES($1,$2,$3,$2,'Open Task','open',$2)`, taskID, ownerID, projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO action_items(id,task_id,title,position,series_id,occurrence_date,timezone,completed) VALUES($1,$2,'Already finished',0,$1,$3::date,'UTC',true)`, actionItemID, taskID, time.Now().UTC().Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM action_item_frequencies WHERE action_item_id=$1`, actionItemID)
		_, _ = pool.Exec(cleanup, `DELETE FROM action_items WHERE id=$1`, actionItemID)
		_, _ = pool.Exec(cleanup, `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanup, `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	project, err := projectusecase.NewGetProjectUseCase(store.Project.Projects, store.Project.Projects, nil).Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID))
	if err != nil || project.Progress != 100 || project.Status != "open" {
		t.Fatalf("open Task Project = status %q progress %d error=%v; want open at 100%%", project.Status, project.Progress, err)
	}
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project)
	title := "Count-neutral ActionItem edit"
	if _, err := taskusecase.NewUpdateActionItemUseCase(taskUOW, nil).Execute(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(taskID), taskdomain.ActionItemID(actionItemID), "current", taskusecase.PatchField[string]{Present: true, Value: &title}, taskusecase.PatchField[string]{}, taskusecase.PatchField[time.Time]{}); err != nil {
		t.Fatalf("edit completed ActionItem under open Task: %v", err)
	}
	status, _ := readProjectState(t, pool, projectID)
	if status != "open" {
		t.Fatalf("100%% ActionItem progress auto-completed an open Task's Project: status=%q", status)
	}
}

func TestActionItemCompletionAndReopenTransitionTaskAndProject(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID, taskID, actionItemID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'ActionItem','Lifecycle',$2,'unused','UTC')`, ownerID, ownerID+"@actionItem-lifecycle.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','ActionItem Lifecycle','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM action_item_frequencies WHERE action_item_id=$1`, actionItemID)
		_, _ = pool.Exec(cleanup, `DELETE FROM action_items WHERE id=$1`, actionItemID)
		_, _ = pool.Exec(cleanup, `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanup, `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project)
	_, err := taskusecase.NewCreateTaskInProjectUseCase(taskUOW, nil).Execute(ctx, taskusecase.CreateTaskInProjectInput{
		ID: taskdomain.TaskID(taskID), UserID: taskdomain.UserID(ownerID), ProjectID: taskdomain.ProjectID(projectID), Title: "ActionItem-backed Task",
	})
	if err != nil {
		t.Fatalf("create linked Task: %v", err)
	}
	_, err = taskusecase.NewCreateActionItemUseCase(taskUOW, projectCrossContextTimezoneReader{}, nil).Execute(ctx, taskusecase.CreateActionItemInput{
		ID: taskdomain.ActionItemID(actionItemID), UserID: taskdomain.UserID(ownerID), TaskID: taskdomain.TaskID(taskID), Title: "Complete me",
	})
	if err != nil {
		t.Fatalf("create ActionItem: %v", err)
	}
	if err := taskusecase.NewCompleteActionItemUseCase(taskUOW, nil).Execute(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(taskID), taskdomain.ActionItemID(actionItemID)); err != nil {
		t.Fatalf("complete final ActionItem: %v", err)
	}
	status, _ := readProjectState(t, pool, projectID)
	var taskStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, taskID).Scan(&taskStatus); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "done" || status != "done" {
		t.Fatalf("after final ActionItem completion Task=%q Project=%q; want both done", taskStatus, status)
	}
	if err := taskusecase.NewReopenActionItemUseCase(taskUOW, nil).Execute(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(taskID), taskdomain.ActionItemID(actionItemID)); err != nil {
		t.Fatalf("reopen completed ActionItem: %v", err)
	}
	status, _ = readProjectState(t, pool, projectID)
	if err := pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, taskID).Scan(&taskStatus); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "open" || status != "open" {
		t.Fatalf("after ActionItem reopen Task=%q Project=%q; want both open", taskStatus, status)
	}
}

func TestMovingCompletedTaskReconcilesBothProjects(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, sourceID, targetID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	firstTaskID, secondTaskID, targetTaskID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Project','Move',$2,'unused','UTC')`, ownerID, ownerID+"@project-move.test"); err != nil {
		t.Fatal(err)
	}
	for id, title := range map[string]string{sourceID: "Source", targetID: "Target"} {
		if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other',$3,'low',$2)`, id, ownerID, title); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM task_revisions WHERE id=ANY($1::text[])`, []string{firstTaskID, secondTaskID, targetTaskID})
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=ANY($1::text[])`, []string{sourceID, targetID})
		_, _ = pool.Exec(cleanup, `DELETE FROM tasks WHERE id=ANY($1::text[])`, []string{firstTaskID, secondTaskID, targetTaskID})
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=ANY($1::text[])`, []string{sourceID, targetID})
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project)
	newTask := func(id, projectID, title string) {
		t.Helper()
		_, err := taskusecase.NewCreateTaskInProjectUseCase(taskUOW, nil).Execute(ctx, taskusecase.CreateTaskInProjectInput{
			ID: taskdomain.TaskID(id), UserID: taskdomain.UserID(ownerID), ProjectID: taskdomain.ProjectID(projectID), Title: title,
		})
		if err != nil {
			t.Fatalf("create Task %s: %v", id, err)
		}
	}
	newTask(firstTaskID, sourceID, "Move me")
	newTask(secondTaskID, sourceID, "Remain in source")
	newTask(targetTaskID, targetID, "Remain in target")
	complete := taskusecase.NewCompleteTaskUseCase(taskUOW, time.Now, nil)
	completed, err := complete.Execute(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(firstTaskID), 1)
	if err != nil {
		t.Fatalf("complete Task to move: %v", err)
	}
	if _, err := complete.Execute(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(secondTaskID), 1); err != nil {
		t.Fatalf("complete remaining source Task: %v", err)
	}
	if _, err := complete.Execute(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(targetTaskID), 1); err != nil {
		t.Fatalf("complete existing target Task: %v", err)
	}
	storeProject := projectusecase.NewChangeProjectStatusUseCase(application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule), store.Project.Projects, nil)
	for _, projectID := range []string{sourceID, targetID} {
		_, revision := readProjectState(t, pool, projectID)
		if _, err := storeProject.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), revision, "open"); err != nil {
			t.Fatalf("manually reopen %s at 100%%: %v", projectID, err)
		}
	}
	if _, err := taskusecase.NewAddTaskToProjectUseCase(taskUOW, store.Task.Tasks, nil).Execute(
		ctx, taskdomain.UserID(ownerID), taskdomain.ProjectID(targetID), taskdomain.TaskID(firstTaskID), completed.Revision,
	); err != nil {
		t.Fatalf("move completed Task between Projects: %v", err)
	}
	for _, projectID := range []string{sourceID, targetID} {
		status, _ := readProjectState(t, pool, projectID)
		if status != "done" {
			t.Fatalf("Project %s after Task move status=%q, want done after recalculating its changed eligible work", projectID, status)
		}
	}
}

func TestDoneProjectScheduleVirtualSuppressionAndSkipRestoreReopen(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Lifecycle',$2,'unused','UTC')`, ownerID, ownerID+"@schedule-lifecycle.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Schedule Lifecycle','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id IN (SELECT id FROM schedules WHERE project_id=$1)`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_frequencies WHERE schedule_id=$1`, scheduleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	today := time.Now().UTC().Truncate(24 * time.Hour)
	weekday := strings.ToLower(today.Weekday().String())[:3]
	rootDate := today.AddDate(0, 0, -14)
	futureDate := today.AddDate(0, 0, 7)
	store := application.NewStore(pool)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project)
	created, err := scheduleusecase.NewCreateScheduleUseCase(scheduleUOW, projectCrossContextTimezoneReader{}, nil).Execute(ctx, scheduleusecase.CreateScheduleInput{
		ID: scheduledomain.ScheduleID(scheduleID), UserID: scheduledomain.UserID(ownerID), ProjectID: scheduledomain.ProjectID(projectID),
		Title: "Weekly schedule", StartAt: rootDate.Add(9 * time.Hour), EndAt: rootDate.Add(10 * time.Hour), IntervalWeeks: 1, Frequencies: []string{weekday},
	})
	if err != nil {
		t.Fatalf("create recurring Project Schedule: %v", err)
	}
	complete := scheduleusecase.NewCompleteScheduleUseCase(scheduleUOW, nil, projectActionItemID{})
	if err := complete.ExecuteOccurrence(ctx, scheduledomain.UserID(ownerID), scheduledomain.ScheduleID(scheduleID), futureDate.Format("2006-01-02")); err != nil {
		t.Fatalf("save completed future occurrence: %v", err)
	}
	if err := scheduleusecase.NewSkipScheduleUseCase(scheduleUOW, nil).Execute(ctx, scheduledomain.UserID(ownerID), scheduledomain.ScheduleID(scheduleID), today.Format("2006-01-02")); err != nil {
		t.Fatalf("skip today's unsaved occurrence: %v", err)
	}
	status, revision := readProjectState(t, pool, projectID)
	if status != "open" {
		t.Fatalf("Project status after skipping today's occurrence = %q; want open while saved root remains unfinished", status)
	}
	if _, err := projectusecase.NewChangeProjectStatusUseCase(application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule), store.Project.Projects, nil).Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), revision, "done"); err != nil {
		t.Fatalf("manually complete Project before virtual suppression check: %v", err)
	}
	page, err := scheduleusecase.NewListProjectSchedulesUseCase(scheduleUOW, nil).ExecutePage(ctx, scheduledomain.UserID(ownerID), scheduledomain.ProjectID(projectID), scheduleusecase.CursorPageRequest{Size: 10, FromDate: today.Format("2006-01-02")})
	if err != nil {
		t.Fatalf("list done Project Schedule occurrences: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].OccurrenceDate != futureDate.Format("2006-01-02") || !page.Items[0].Completed {
		t.Fatalf("done Project Schedule page=%+v; want only the saved completed future occurrence", page.Items)
	}
	if err := scheduleusecase.NewRestoreScheduleUseCase(scheduleUOW, nil).Execute(ctx, scheduledomain.UserID(ownerID), scheduledomain.ScheduleID(scheduleID), today.Format("2006-01-02")); err != nil {
		t.Fatalf("restore previously saved skipped occurrence under done Project: %v", err)
	}
	status, _ = readProjectState(t, pool, projectID)
	if status != "open" {
		t.Fatalf("Project status after restoring unfinished skipped occurrence: %q; want open", status)
	}
	if created.ID != scheduleID {
		t.Fatalf("created Schedule id=%q, want %q", created.ID, scheduleID)
	}
}

func TestDeletedScheduleOverrideRemainsSuppressedFromProjectVirtualProgress(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID, scheduleID, deletedOverrideID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Deleted',$2,'unused','UTC')`, ownerID, ownerID+"@schedule-deleted-override.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Deleted Schedule Override','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	rootDate, currentDate := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,is_exception,repeat_state,frequency_anchor_date,interval_weeks,changed_by)
		VALUES($1,$2,$3,$2,'Weekly root',$4,$5,$1,$6::date,'UTC',false,'active',$6::date,1,$2)
	`, scheduleID, ownerID, projectID, rootDate.Add(9*time.Hour), rootDate.Add(10*time.Hour), rootDate.Format("2006-01-02")); err != nil {
		t.Fatalf("insert recurring Schedule root: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schedule_frequencies(schedule_id,frequency) VALUES($1,'fri')`, scheduleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO schedules(id,user_id,project_id,assignee_id,title,start_at,end_at,series_id,occurrence_date,timezone,is_exception,repeat_state,interval_weeks,deleted_at,changed_by)
		VALUES($1,$2,$3,$2,'Deleted override',$4,$5,$6,$7::date,'UTC',true,NULL,0,now(),$2)
	`, deletedOverrideID, ownerID, projectID, currentDate.Add(9*time.Hour), currentDate.Add(10*time.Hour), scheduleID, currentDate.Format("2006-01-02")); err != nil {
		t.Fatalf("insert deleted Schedule override: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_revisions WHERE id=ANY($1::text[])`, []string{scheduleID, deletedOverrideID})
		_, _ = pool.Exec(cleanup, `DELETE FROM schedule_frequencies WHERE schedule_id=$1`, scheduleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM schedules WHERE id=ANY($1::text[])`, []string{scheduleID, deletedOverrideID})
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	lifecycle := projectusecase.NewProjectLifecycleUseCase(store.Project.Projects, store.Project.Projects)
	state, err := lifecycle.CaptureWorkState(ctx, projectID, time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("capture Project progress with deleted override: %v", err)
	}
	if state.Eligible != 1 || state.Completed != 0 {
		t.Fatalf("deleted override Project work state = %+v; want only the saved root occurrence (no virtual re-add)", state)
	}
}

func TestProjectDeleteLeavesTaskAndScheduleChildrenUntouched(t *testing.T) {
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
	if err := deleteProject.Execute(ctx, projectdomain.UserID(actorID), projectdomain.ProjectID(projectID), 1); err != nil {
		t.Fatalf("DeleteProject() should only mutate the parent: %v", err)
	}
	var projectDeleted, taskDeleted, scheduleDeleted bool
	var projectRevision, taskRevision, scheduleRevision int
	var projectChangedBy, taskChangedBy, scheduleChangedBy string
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,revision,changed_by FROM projects WHERE id=$1`, projectID).Scan(&projectDeleted, &projectRevision, &projectChangedBy); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,revision,changed_by FROM tasks WHERE id=$1`, taskID).Scan(&taskDeleted, &taskRevision, &taskChangedBy); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,revision,changed_by FROM schedules WHERE id=$1`, scheduleID).Scan(&scheduleDeleted, &scheduleRevision, &scheduleChangedBy); err != nil {
		t.Fatal(err)
	}
	if !projectDeleted || taskDeleted || scheduleDeleted {
		t.Fatalf("Project/child deletion states = project:%t task:%t schedule:%t; want only parent deleted", projectDeleted, taskDeleted, scheduleDeleted)
	}
	if projectRevision != 2 || taskRevision != 1 || scheduleRevision != 1 || projectChangedBy != actorID || taskChangedBy != actorID || scheduleChangedBy != actorID {
		t.Fatalf("Project/child snapshots: project=(%d,%q) task=(%d,%q) schedule=(%d,%q); want parent revision 2 and unchanged children at revision 1", projectRevision, projectChangedBy, taskRevision, taskChangedBy, scheduleRevision, scheduleChangedBy)
	}
}

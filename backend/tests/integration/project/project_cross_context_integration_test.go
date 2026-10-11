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

func projectCrossContextTestPool(t *testing.T) *pgxpool.Pool {
	return testdb.Open(t)
}

// newProjectUseCases builds the same cross-context progress reader as production.
func newProjectUseCases(t *testing.T, pool *pgxpool.Pool, store application.Store) *projectusecase.UseCases {
	t.Helper()
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule)
	usecases, stopUseCases := application.NewUseCase(store.Auth, store.Project, store.Task, store.Tag, store.Admin,
		uow.Project, uow.Task, uow.Schedule, projectActionItemID{}, nil)
	testdb.RegisterShutdown(t, stopUseCases)
	return usecases.Project
}

type projectCrossContextTimezoneReader struct{}

type projectActionItemID struct{}

func (projectActionItemID) Generate() string { return ulid.Make().String() }

func (projectCrossContextTimezoneReader) GetTimezone(context.Context, string) (string, error) {
	return "UTC", nil
}

func (projectCrossContextTimezoneReader) TimezoneExists(context.Context, string) (bool, error) {
	return true, nil
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

func TestProjectLifecycleTransitionsAcrossTaskAndScheduleMutations(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, actorID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	firstTaskID, secondTaskID := ulid.Make().String(), ulid.Make().String()
	firstBlueprintID, secondBlueprintID := "", ""
	firstScheduleID, secondScheduleID := "", ""
	firstOccurrenceDate, secondOccurrenceDate := "", ""
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
		_ = testdb.DeleteScheduleFixture(cleanup, pool, firstBlueprintID)
		_ = testdb.DeleteScheduleFixture(cleanup, pool, secondBlueprintID)
		_, _ = pool.Exec(cleanup, `DELETE FROM task_revisions WHERE id=ANY($1::text[])`, []string{firstTaskID, secondTaskID})
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM tasks WHERE id=ANY($1::text[])`, []string{firstTaskID, secondTaskID})
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM roles WHERE role_id=$1`, roleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, actorID})
	})
	var canUpdateProject, canUpdateTask, canUpdateSchedule bool
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project, store.Schedule)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project, store.Task)
	projectUOW := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	task, err := taskusecase.NewCreateTaskInProjectUseCase(taskUOW, nil).Execute(ctx, taskusecase.CreateTaskInProjectInput{
		ID: taskdomain.TaskID(firstTaskID), UserID: taskdomain.UserID(ownerID), ProjectID: taskdomain.ProjectID(projectID), Title: "Initial Task",
	})
	if err != nil {
		t.Fatalf("create initial Task: %v", err)
	}
	firstStart := time.Now().UTC().Add(time.Hour).Truncate(time.Minute)
	firstOccurrenceDate = firstStart.Format("2006-01-02")
	firstBlueprint, err := scheduleusecase.NewCreateBlueprintUseCase(scheduleUOW, projectActionItemID{}, projectCrossContextTimezoneReader{}, nil).Execute(ctx, ownerID, scheduleusecase.CreateBlueprintInput{
		Title: "Initial Schedule", ProjectID: projectID, Timezone: "UTC", LocalStart: firstStart, LocalEnd: firstStart.Add(time.Hour), Frequency: "once",
	})
	if err != nil {
		t.Fatalf("create initial Schedule: %v", err)
	}
	firstBlueprintID = firstBlueprint.ID
	firstOccurrence, err := scheduleusecase.NewGetOccurrenceUseCase(scheduleUOW).Execute(ctx, ownerID, firstBlueprintID, firstOccurrenceDate)
	if err != nil || firstOccurrence.ID == nil {
		t.Fatalf("read initial Schedule occurrence: %+v error=%v", firstOccurrence, err)
	}
	firstScheduleID = *firstOccurrence.ID
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
	if err := scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "complete", nil, projectActionItemID{}).Execute(ctx, actorID, firstBlueprintID, firstOccurrenceDate, firstOccurrence.Revision); err != nil {
		t.Fatalf("complete child Schedule without project.update: %v", err)
	}
	status, revision := readProjectState(t, pool, projectID)
	if status != "done" {
		t.Fatalf("Project status after all direct work completed = %q; want done", status)
	}
	projectCases := newProjectUseCases(t, pool, store)
	projectRead := projectCases.Get
	project, err := projectRead.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID))
	if err != nil || project.Progress != 100 {
		t.Fatalf("completed Project progress = %d, error=%v; want 100", project.Progress, err)
	}
	projectStatus := projectCases.ChangeStatus
	reopened, err := projectStatus.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), revision, "open")
	if err != nil || reopened.Status != "open" || reopened.Progress != 100 {
		t.Fatalf("manual reopen = status %q progress %d error=%v; want open at 100%%", reopened.Status, reopened.Progress, err)
	}
	currentOccurrence, err := scheduleusecase.NewGetOccurrenceUseCase(scheduleUOW).Execute(ctx, ownerID, firstBlueprintID, firstOccurrenceDate)
	if err != nil || currentOccurrence.ID == nil {
		t.Fatalf("read completed occurrence before count-neutral edit: %+v error=%v", currentOccurrence, err)
	}
	title := "Count-neutral Schedule edit"
	if _, err := scheduleusecase.NewUpdateOccurrenceUseCase(scheduleUOW, nil, projectActionItemID{}).Execute(ctx, actorID, firstBlueprintID, firstOccurrenceDate, scheduleusecase.UpdateScheduleInput{Title: &title}, currentOccurrence.Revision); err != nil {
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
	secondStart := time.Now().UTC().Add(3 * time.Hour).Truncate(time.Minute)
	secondOccurrenceDate = secondStart.Format("2006-01-02")
	secondBlueprint, err := scheduleusecase.NewCreateBlueprintUseCase(scheduleUOW, projectActionItemID{}, projectCrossContextTimezoneReader{}, nil).Execute(ctx, ownerID, scheduleusecase.CreateBlueprintInput{
		Title: "Added after completion", ProjectID: projectID, Timezone: "UTC", LocalStart: secondStart, LocalEnd: secondStart.Add(time.Hour), Frequency: "once",
	})
	if err != nil {
		t.Fatalf("add unfinished Schedule to done Project: %v", err)
	}
	secondBlueprintID = secondBlueprint.ID
	secondOccurrence, err := scheduleusecase.NewGetOccurrenceUseCase(scheduleUOW).Execute(ctx, ownerID, secondBlueprintID, secondOccurrenceDate)
	if err != nil || secondOccurrence.ID == nil {
		t.Fatalf("read second Schedule occurrence: %+v error=%v", secondOccurrence, err)
	}
	secondScheduleID = *secondOccurrence.ID
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
	if err := scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "complete", nil, projectActionItemID{}).Execute(ctx, actorID, secondBlueprintID, secondOccurrenceDate, secondOccurrence.Revision); err == nil {
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
	if _, err := pool.Exec(ctx, `INSERT INTO action_items(id,task_id,title,position,completed) VALUES($1,$2,'Already finished',0,true)`, actionItemID, taskID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM action_items WHERE id=$1`, actionItemID)
		_, _ = pool.Exec(cleanup, `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanup, `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	project, err := newProjectUseCases(t, pool, store).Get.Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID))
	if err != nil || project.Progress != 100 || project.Status != "open" {
		t.Fatalf("open Task Project = status %q progress %d error=%v; want open at 100%%", project.Status, project.Progress, err)
	}
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project, store.Schedule)
	title := "Count-neutral ActionItem edit"
	if _, err := taskusecase.NewUpdateActionItemUseCase(taskUOW, nil).Execute(ctx, taskdomain.UserID(ownerID), taskdomain.TaskID(taskID), taskdomain.ActionItemID(actionItemID), taskusecase.PatchField[string]{Present: true, Value: &title}, taskusecase.PatchField[string]{}, taskusecase.PatchField[time.Time]{}); err != nil {
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
		_, _ = pool.Exec(cleanup, `DELETE FROM action_items WHERE id=$1`, actionItemID)
		_, _ = pool.Exec(cleanup, `DELETE FROM task_revisions WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanup, `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project, store.Schedule)
	_, err := taskusecase.NewCreateTaskInProjectUseCase(taskUOW, nil).Execute(ctx, taskusecase.CreateTaskInProjectInput{
		ID: taskdomain.TaskID(taskID), UserID: taskdomain.UserID(ownerID), ProjectID: taskdomain.ProjectID(projectID), Title: "ActionItem-backed Task",
	})
	if err != nil {
		t.Fatalf("create linked Task: %v", err)
	}
	_, err = taskusecase.NewCreateActionItemUseCase(taskUOW, nil).Execute(ctx, taskusecase.CreateActionItemInput{
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
	taskUOW := application.NewTaskUOW(pool, store.Task, store.Project, store.Schedule)
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
	ownerID, projectID := ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Lifecycle',$2,'unused','UTC')`, ownerID, ownerID+"@schedule-lifecycle.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Schedule Lifecycle','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	today := time.Now().UTC().Truncate(24 * time.Hour)
	weekday := strings.ToLower(today.Weekday().String())[:3]
	store := application.NewStore(pool)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project, store.Task)
	created, err := scheduleusecase.NewCreateBlueprintUseCase(scheduleUOW, projectActionItemID{}, projectCrossContextTimezoneReader{}, nil).Execute(ctx, ownerID, scheduleusecase.CreateBlueprintInput{
		ProjectID: projectID, Title: "Weekly schedule", Timezone: "UTC", LocalStart: today.Add(9 * time.Hour), LocalEnd: today.Add(10 * time.Hour),
		Frequency: "weekly", Interval: 1, Weekdays: []string{weekday},
	})
	if err != nil {
		t.Fatalf("create recurring Project Schedule blueprint: %v", err)
	}
	if err := scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "skip", nil, projectActionItemID{}).Execute(ctx, ownerID, created.ID, today.Format("2006-01-02"), 0); err != nil {
		t.Fatalf("skip today's unsaved occurrence: %v", err)
	}
	status, revision := readProjectState(t, pool, projectID)
	if status != "open" {
		t.Fatalf("Project status after skipping today's occurrence = %q; want open while saved root remains unfinished", status)
	}
	if _, err := projectusecase.NewChangeProjectStatusUseCase(application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule), store.Project.Projects, nil).Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), revision, "done"); err != nil {
		t.Fatalf("manually complete Project before virtual suppression check: %v", err)
	}
	page, err := scheduleusecase.NewListOccurrencesUseCase(scheduleUOW).ExecutePage(ctx, ownerID, scheduleusecase.CursorPageRequest{
		Size: 10, From: today, To: today.AddDate(0, 0, 14), ProjectID: projectID,
	})
	if err != nil {
		t.Fatalf("list done Project Schedule occurrences: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("done Project Schedule page=%+v; want paused and skipped occurrences suppressed", page.Items)
	}
	if err := scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "restore", nil, projectActionItemID{}).Execute(ctx, ownerID, created.ID, today.Format("2006-01-02"), 1); err != nil {
		t.Fatalf("restore previously saved skipped occurrence under done Project: %v", err)
	}
	status, _ = readProjectState(t, pool, projectID)
	if status != "open" {
		t.Fatalf("Project status after restoring unfinished skipped occurrence: %q; want open", status)
	}
}

func TestAutoCompletingProjectPausesRecurringBlueprintAndManualReopenResumesIt(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID := ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Pause',$2,'unused','UTC')`, ownerID, ownerID+"@schedule-pause.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Schedule Pause','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	blueprintID := ""
	t.Cleanup(func() {
		cleanup := context.Background()
		_ = testdb.DeleteScheduleFixture(cleanup, pool, blueprintID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	today := time.Now().UTC().Truncate(24 * time.Hour)
	weekday := strings.ToLower(today.Weekday().String())[:3]
	store := application.NewStore(pool)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project, store.Task)
	blueprint, err := scheduleusecase.NewCreateBlueprintUseCase(scheduleUOW, projectActionItemID{}, projectCrossContextTimezoneReader{}, nil).Execute(ctx, ownerID, scheduleusecase.CreateBlueprintInput{
		ProjectID: projectID, Title: "Recurring work", Timezone: "UTC", LocalStart: today.Add(9 * time.Hour), LocalEnd: today.Add(10 * time.Hour),
		Frequency: "weekly", Interval: 1, Weekdays: []string{weekday},
	})
	if err != nil {
		t.Fatalf("create recurring Schedule blueprint: %v", err)
	}
	blueprintID = blueprint.ID
	if err := scheduleusecase.NewSetOccurrenceStateUseCase(scheduleUOW, "complete", nil, projectActionItemID{}).Execute(ctx, ownerID, blueprintID, today.Format("2006-01-02"), 0); err != nil {
		t.Fatalf("complete today's recurring occurrence: %v", err)
	}
	status, revision := readProjectState(t, pool, projectID)
	if status != "done" {
		t.Fatalf("Project after completing its only eligible occurrence = %q; want done", status)
	}
	var activePauses int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedule_project_pause_intervals WHERE project_id=$1 AND blueprint_id=$2 AND resumed_at IS NULL`, projectID, blueprintID).Scan(&activePauses); err != nil {
		t.Fatal(err)
	}
	if activePauses != 1 {
		t.Fatalf("active Project pause intervals = %d; want 1 after automatic completion", activePauses)
	}
	list := scheduleusecase.NewListOccurrencesUseCase(scheduleUOW)
	request := scheduleusecase.CursorPageRequest{Size: 10, From: today, To: today.AddDate(0, 0, 8), ProjectID: projectID}
	paused, err := list.ExecutePage(ctx, ownerID, request)
	if err != nil || len(paused.Items) != 1 || paused.Items[0].OccurrenceDate != today.Format("2006-01-02") || !paused.Items[0].Completed {
		t.Fatalf("done Project occurrence page=%+v error=%v; want saved completed occurrence only", paused.Items, err)
	}
	if _, err := projectusecase.NewChangeProjectStatusUseCase(application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule), store.Project.Projects, nil).Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), revision, "open"); err != nil {
		t.Fatalf("manually reopen Project: %v", err)
	}
	var stillActive int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedule_project_pause_intervals WHERE project_id=$1 AND blueprint_id=$2 AND resumed_at IS NULL`, projectID, blueprintID).Scan(&stillActive); err != nil {
		t.Fatal(err)
	}
	if stillActive != 0 {
		t.Fatalf("active Project pause intervals after manual reopen = %d; want 0", stillActive)
	}
	resumed, err := list.ExecutePage(ctx, ownerID, request)
	if err != nil || len(resumed.Items) != 2 {
		t.Fatalf("reopened Project occurrence page=%+v error=%v; want saved today plus future virtual", resumed.Items, err)
	}
	if resumed.Items[0].OccurrenceDate != today.Format("2006-01-02") || resumed.Items[1].OccurrenceDate != today.AddDate(0, 0, 7).Format("2006-01-02") {
		t.Fatalf("reopened Project occurrence dates=%v,%v; want today and next weekly date", resumed.Items[0].OccurrenceDate, resumed.Items[1].OccurrenceDate)
	}
}

func TestDeletedScheduleOverrideRemainsSuppressedFromProjectVirtualProgress(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, projectID, blueprintID, deletedOverrideID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Deleted',$2,'unused','UTC')`, ownerID, ownerID+"@schedule-deleted-override.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Deleted Schedule Override','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	rootDate := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	currentDate := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	if err := testdb.InsertScheduleFixture(ctx, pool, testdb.ScheduleFixture{
		BlueprintID: blueprintID, ScheduleID: deletedOverrideID, UserID: ownerID, ProjectID: projectID,
		Title: "Weekly rule with deleted occurrence", Frequency: "weekly", Interval: 1, Weekdays: []string{"fri"},
		AnchorDate: rootDate.Format("2006-01-02"), OccurrenceDate: currentDate.Format("2006-01-02"),
		StartAt: currentDate.Add(9 * time.Hour), EndAt: currentDate.Add(10 * time.Hour), Deleted: true,
	}); err != nil {
		t.Fatalf("insert deleted Schedule override: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_ = testdb.DeleteScheduleFixture(cleanup, pool, blueprintID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, ownerID)
	})
	store := application.NewStore(pool)
	lifecycle := projectusecase.NewProjectLifecycleUseCase(store.Project.Projects, store.Project.Projects, store.Schedule.Schedules)
	state, err := lifecycle.CaptureWorkState(ctx, projectID, time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("capture Project progress with deleted override: %v", err)
	}
	if state.Eligible != 0 || state.Completed != 0 {
		t.Fatalf("deleted override Project work state = %+v; want no unsaved past occurrence and no tombstone resurrection", state)
	}
}

func TestProjectMemberRemovalReassignsCurrentAndFutureBlueprintOccurrences(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	ownerID, memberID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, userID := range []string{ownerID, memberID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Schedule','Member',$2,'unused','UTC')`, userID, userID+"@schedule-member.test"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Member reassignment','low',$2)`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3)`, projectID, memberID, ownerID); err != nil {
		t.Fatal(err)
	}
	blueprintID := ""
	t.Cleanup(func() {
		cleanup := context.Background()
		_ = testdb.DeleteScheduleFixture(cleanup, pool, blueprintID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=ANY($1::text[])`, []string{ownerID, memberID})
	})
	today := time.Now().UTC().Truncate(24 * time.Hour)
	anchorDate := today.AddDate(0, 0, -3)
	store := application.NewStore(pool)
	scheduleUOW := application.NewScheduleUOW(pool, store.Schedule, store.Project, store.Task)
	bl, err := scheduleusecase.NewCreateBlueprintUseCase(scheduleUOW, projectActionItemID{}, projectCrossContextTimezoneReader{}, nil).Execute(ctx, ownerID, scheduleusecase.CreateBlueprintInput{
		ProjectID: projectID, AssigneeID: memberID, Title: "Member's recurring work", Timezone: "UTC",
		LocalStart: anchorDate.Add(9 * time.Hour), LocalEnd: anchorDate.Add(10 * time.Hour), Frequency: "daily", Interval: 2,
	})
	if err != nil {
		t.Fatalf("create member-assigned blueprint: %v", err)
	}
	blueprintID = bl.ID
	plannedDate := today.AddDate(0, 0, 30)
	plannedRevisionID := ulid.Make().String()
	plannedStart := plannedDate.Add(9 * time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO schedule_blueprint_revisions(
			id,blueprint_id,user_id,revision,effective_from,recurrence_anchor_date,local_start,local_end,
		timezone,frequency,interval_count,weekdays,title,project_id,assignee_id,changed_by
		) VALUES($1,$2,$3,2,$4::date,$5::date,$6::timestamp,$7::timestamp,'UTC','daily',2,'{}','Planned future title',$8,$9,$3)
	`, plannedRevisionID, blueprintID, ownerID, plannedDate.Format("2006-01-02"), anchorDate.Format("2006-01-02"),
		plannedStart.Format("2006-01-02 15:04:05"), plannedStart.Add(time.Hour).Format("2006-01-02 15:04:05"), projectID, memberID); err != nil {
		t.Fatalf("insert future planned revision: %v", err)
	}
	list := scheduleusecase.NewListOccurrencesUseCase(scheduleUOW)
	request := scheduleusecase.CursorPageRequest{Size: 10, From: today, To: today.AddDate(0, 0, 3), ProjectID: projectID}
	before, err := list.ExecutePage(ctx, ownerID, request)
	if err != nil || len(before.Items) != 1 || before.Items[0].OccurrenceDate != today.AddDate(0, 0, 1).Format("2006-01-02") {
		t.Fatalf("member-assigned occurrence page=%+v error=%v; want the original interval-two cadence on tomorrow and no occurrence today", before.Items, err)
	}
	for _, occurrence := range before.Items {
		if occurrence.AssigneeID != memberID || occurrence.ID != nil {
			t.Fatalf("pre-removal occurrence=%+v; want virtual row assigned to removed member", occurrence)
		}
	}
	projectUOW := application.NewProjectUOW(pool, store.Project, store.Task, store.Schedule)
	if err := projectusecase.NewDeleteProjectMemberUseCase(projectUOW, nil).Execute(ctx, projectdomain.UserID(ownerID), projectdomain.ProjectID(projectID), projectdomain.UserID(memberID)); err != nil {
		t.Fatalf("remove Project member: %v", err)
	}
	after, err := list.ExecutePage(ctx, ownerID, request)
	if err != nil || len(after.Items) != 1 || after.Items[0].OccurrenceDate != today.AddDate(0, 0, 1).Format("2006-01-02") {
		t.Fatalf("reassigned occurrence page=%+v error=%v; want tomorrow on the original interval-two cadence and no occurrence today", after.Items, err)
	}
	for _, occurrence := range after.Items {
		if occurrence.AssigneeID != ownerID || occurrence.ID != nil {
			t.Fatalf("post-removal occurrence=%+v; want virtual row reassigned to Project owner", occurrence)
		}
	}
	history, err := scheduleusecase.NewListBlueprintRevisionsUseCase(scheduleUOW).Execute(ctx, ownerID, blueprintID, 10, 0)
	if err != nil || len(history) < 3 || history[0].Revision <= 2 {
		t.Fatalf("blueprint history after member removal=%+v error=%v; want an owner revision numbered above the preexisting planned revision", history, err)
	}
	revisionsByNumber := make(map[int32]struct {
		AssigneeID string
		Superseded bool
	}, len(history))
	for _, revision := range history {
		revisionsByNumber[revision.Revision] = struct {
			AssigneeID string
			Superseded bool
		}{AssigneeID: revision.AssigneeID, Superseded: revision.Superseded}
		if revision.Revision > 2 && revision.AssigneeID != ownerID {
			t.Fatalf("new member-removal revision=%+v; want Project owner assignment", revision)
		}
	}
	planned, plannedExists := revisionsByNumber[2]
	original, originalExists := revisionsByNumber[1]
	if !plannedExists || !planned.Superseded || planned.AssigneeID != memberID || !originalExists || original.Superseded || original.AssigneeID != memberID {
		t.Fatalf("blueprint history after member removal=%+v; want planned revision 2 superseded, original revision 1 retained, and both historical assignments unchanged", history)
	}
	if after.Items[0].Title != "Member's recurring work" {
		t.Fatalf("tomorrow title after member removal=%q; want planned future revision superseded", after.Items[0].Title)
	}
}

func TestProjectDeleteLeavesTaskAndScheduleChildrenUntouched(t *testing.T) {
	pool := projectCrossContextTestPool(t)
	ctx := t.Context()
	actorID, projectID, taskID, scheduleID, blueprintID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
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
	if err := testdb.InsertScheduleFixture(ctx, pool, testdb.ScheduleFixture{BlueprintID: blueprintID, ScheduleID: scheduleID, UserID: actorID, ProjectID: projectID, Title: "Schedule child", Timezone: "Asia/Tokyo", OccurrenceDate: "2026-10-07", StartAt: start, EndAt: start.Add(time.Hour)}); err != nil {
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
		_ = testdb.DeleteScheduleFixture(context.Background(), pool, blueprintID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM task_revisions WHERE id = $1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id = $1`, projectID)
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

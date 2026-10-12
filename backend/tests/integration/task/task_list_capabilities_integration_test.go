//go:build integration

package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	inboxusecase "github.com/Najah7/task2todaytodo/internal/application/inbox/usecase"
	taskdao "github.com/Najah7/task2todaytodo/internal/application/task/dao"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/oklog/ulid/v2"
)

func TestTaskTodayCapabilitySurvivesFilteredProjectAndInboxProjections(t *testing.T) {
	pool := integrationPool(t)
	userID, childlessTaskID := seedTask(t, pool, "UTC")
	projectID := ulid.Make().String()
	otherUserID := ulid.Make().String()
	startedTaskID, otherAssigneeTaskID := ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone) VALUES($1,'Other','Assignee',$2,'unused',now(),'UTC')`, otherUserID, string(otherUserID)+"@task-capability.test"); err != nil {
		t.Fatalf("insert other assignee: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Capability project','low',$2)`, projectID, userID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE tasks SET project_id=$1,assignee_id=$2,manual_estimated_minutes=30,status='open' WHERE id=$3`, projectID, userID, childlessTaskID); err != nil {
		t.Fatalf("prepare childless self Task: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,project_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$3,$2,'Started self Task',30,'low','open',$2),($4,$2,$3,$5,'Other-assignee Task',30,'low','open',$2)`, startedTaskID, userID, projectID, otherAssigneeTaskID, otherUserID); err != nil {
		t.Fatalf("insert comparison Tasks: %v", err)
	}
	previousDate := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	todoID, frameID := ulid.Make().String(), ulid.Make().String()
	if _, err := pool.Exec(t.Context(), `INSERT INTO todos(id,user_id,todo_date,timezone) VALUES($1,$2,$3,'UTC')`, todoID, userID, previousDate); err != nil {
		t.Fatalf("insert Task history list: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO todo_items(id,todo_id,kind,task_id,planned_minutes,completed,position) VALUES($1,$2,'task',$3,10,true,0)`, frameID, todoID, startedTaskID); err != nil {
		t.Fatalf("insert Task history frame: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM todos WHERE id=$1`, todoID)
		_, _ = pool.Exec(ctx, `DELETE FROM task_revisions WHERE id=ANY($1::text[])`, []string{string(childlessTaskID), startedTaskID, otherAssigneeTaskID})
		_, _ = pool.Exec(ctx, `DELETE FROM tasks WHERE id=ANY($1::text[])`, []string{string(childlessTaskID), startedTaskID, otherAssigneeTaskID})
		_, _ = pool.Exec(ctx, `DELETE FROM project_revisions WHERE id=$1`, projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, otherUserID)
	})

	store := application.NewStore(pool)
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule, store.Allocation)
	usecases, closeUseCases := application.NewUseCase(store, uow, adapter.NewIDIssuer(), telemetry.Nop())
	testdb.RegisterShutdown(t, closeUseCases)

	asOf := time.Now().UTC()
	filtered, err := usecases.Task.List.ExecuteFilteredPage(t.Context(), taskdomain.UserID(userID), taskusecase.TaskListRequest{
		Size: 20, Status: "open", DueFilter: "all", SortBy: "created_at", SortOrder: "desc", AsOf: asOf,
	})
	if err != nil {
		t.Fatalf("filtered Task list: %v", err)
	}
	filteredByID := make(map[string]taskdao.Task, len(filtered.Items))
	for _, item := range filtered.Items {
		filteredByID[item.ID] = item
	}
	self := filteredByID[string(childlessTaskID)]
	if !self.CanAddToToday || !self.EstimateEditable || self.Started {
		t.Errorf("filtered Task list dropped childless/unstarted self capability: %+v", self)
	}
	started := filteredByID[string(startedTaskID)]
	if !started.Started || !started.CanAddToToday || started.EstimateEditable || started.CanAddActionItem {
		t.Errorf("filtered Task list capabilities for completed-history Task=%+v; want manual Today add allowed but estimate/child add locked", started)
	}

	projectPage, err := usecases.Task.ListByProject.Execute(t.Context(), taskdomain.UserID(userID), taskdomain.ProjectID(projectID), taskusecase.CursorPageRequest{Size: 20})
	if err != nil {
		t.Fatalf("ProjectTask list: %v", err)
	}
	projectByID := make(map[string]taskdao.Task, len(projectPage.Items))
	for _, item := range projectPage.Items {
		projectByID[item.ID] = item
	}
	if !projectByID[string(childlessTaskID)].CanAddToToday {
		t.Errorf("ProjectTask list lost self-assigned childless Task capability: %+v", projectByID[string(childlessTaskID)])
	}
	if projectByID[string(otherAssigneeTaskID)].CanAddToToday {
		t.Errorf("ProjectTask list allowed Today add for other assignee: %+v", projectByID[string(otherAssigneeTaskID)])
	}

	inbox := application.NewInboxUseCases(store.Auth, store.Task, usecases.Schedule, store.Inbox, telemetry.Nop())
	inboxPage, err := inbox.List.Execute(t.Context(), string(userID), inboxusecase.ListRequest{
		Size: 20, TypeFilter: "task", ProjectFilter: inboxusecase.ProjectFilter{Mode: inboxusecase.ProjectFilterAll},
	})
	if err != nil {
		t.Fatalf("Inbox Task projection: %v", err)
	}
	inboxByID := make(map[string]bool, len(inboxPage.Items))
	for _, item := range inboxPage.Items {
		if item.Task != nil {
			inboxByID[item.ID] = item.Task.CanAddToToday
		}
	}
	if !inboxByID[string(childlessTaskID)] {
		t.Errorf("Inbox lost self-assigned childless Task capability; items=%+v", inboxPage.Items)
	}
	if !inboxByID[string(startedTaskID)] {
		t.Errorf("Inbox lost manual Today-add capability for historical remaining work; items=%+v", inboxPage.Items)
	}
}

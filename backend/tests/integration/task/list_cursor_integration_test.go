//go:build integration

package task_test

import (
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
)

func TestCursorListRepositories(t *testing.T) {
	ctx := t.Context()
	pool := testdb.Open(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	const user = "99999999999999999999999901"
	const project = "99999999999999999999999902"
	const task = "99999999999999999999999903"
	if _, err := tx.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password) VALUES($1,'Test','User',$2,'x')`, user, "list-cursor-test@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,start_date,end_date,created_at) VALUES
		($1,$2,'other','Project','2026-01-01','2026-01-02','2026-01-01T00:00:00.000001Z'),
		('99999999999999999999999915',$2,'other','Project 2','2026-01-01','2026-01-02','2026-01-01T00:00:00.000002Z'),
		('99999999999999999999999916',$2,'other','Project 3','2026-01-01','2026-01-02','2026-01-01T00:00:00.000003Z')`, project, user); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tasks(id,user_id,project_id,title,created_at) VALUES
		('99999999999999999999999903',$1,$2,'task-1','2026-01-01T00:00:00.000002Z'),
		('99999999999999999999999904',$1,$2,'task-2','2026-01-01T00:00:00.000002Z'),
		('99999999999999999999999905',$1,$2,'task-3','2026-01-01T00:00:00.000003Z')`, user, project); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO action_items(id,task_id,title,position,series_id,occurrence_date,timezone) VALUES
		('99999999999999999999999909',$1,'actionItem-1',1,'99999999999999999999999909','2026-01-01','UTC'),
		('99999999999999999999999910',$1,'actionItem-2',2,'99999999999999999999999910','2026-01-01','UTC'),
		('99999999999999999999999911',$1,'actionItem-3',3,'99999999999999999999999911','2026-01-01','UTC')`, task); err != nil {
		t.Fatal(err)
	}

	tasks := repository.NewTaskRepository(tx)
	firstTasks, err := tasks.ListByUserIDCursor(ctx, domain.UserID(user), 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	nextTasks, err := tasks.ListByUserIDCursor(ctx, domain.UserID(user), 2, &usecase.CursorAnchor{At: firstTasks[1].CursorCreatedAt, ID: firstTasks[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(firstTasks) != 2 || len(nextTasks) != 1 || nextTasks[0].ID != "99999999999999999999999903" || firstTasks[1].ID != "99999999999999999999999904" || firstTasks[0].ID != "99999999999999999999999905" {
		t.Fatalf("task page boundary: first=%+v next=%+v", firstTasks, nextTasks)
	}
	projectTasks, err := tasks.ListByProjectAndUserIDCursor(ctx, domain.UserID(user), domain.ProjectID(project), 2, nil)
	if err != nil || len(projectTasks) != 2 || projectTasks[0].ID != "99999999999999999999999905" || projectTasks[1].ID != "99999999999999999999999904" {
		t.Fatalf("project tasks first page=%+v err=%v", projectTasks, err)
	}
	projectTasksNext, err := tasks.ListByProjectAndUserIDCursor(ctx, domain.UserID(user), domain.ProjectID(project), 2, &usecase.CursorAnchor{At: projectTasks[1].CursorCreatedAt, ID: projectTasks[1].ID})
	if err != nil || len(projectTasksNext) != 1 || projectTasksNext[0].ID != "99999999999999999999999903" {
		t.Fatalf("project tasks next page=%+v err=%v", projectTasksNext, err)
	}

	actionItem := repository.NewActionItemRepository(tx)
	actionItemRows, err := actionItem.ListByTaskCursor(ctx, domain.UserID(user), domain.TaskID(task), 2, nil)
	if err != nil || len(actionItemRows) != 2 || actionItemRows[0].Position != 1 || actionItemRows[1].Position != 2 {
		t.Fatalf("action item order=%+v err=%v", actionItemRows, err)
	}
	actionItemNext, err := actionItem.ListByTaskCursor(ctx, domain.UserID(user), domain.TaskID(task), 2, &usecase.CursorAnchor{Position: actionItemRows[1].Position, Date: actionItemRows[1].OccurrenceDate, ID: actionItemRows[1].ID})
	if err != nil || len(actionItemNext) != 1 || actionItemNext[0].ID == actionItemRows[1].ID {
		t.Fatalf("action item boundary=%+v err=%v", actionItemNext, err)
	}

}

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCursorListRepositories(t *testing.T) {
	dsn := os.Getenv("TASK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TASK_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
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
	if _, err := tx.Exec(ctx, `INSERT INTO task_tags(id,user_id,name) VALUES
		('99999999999999999999999906',$1,'alpha'),('99999999999999999999999907',$1,'bEta'),('99999999999999999999999908',$1,'GAMma')`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO todo_items(id,task_id,title,position,series_id,occurrence_date,timezone) VALUES
		('99999999999999999999999909',$1,'todo-1',1,'99999999999999999999999909','2026-01-01','UTC'),
		('99999999999999999999999910',$1,'todo-2',2,'99999999999999999999999910','2026-01-01','UTC'),
		('99999999999999999999999911',$1,'todo-3',3,'99999999999999999999999911','2026-01-01','UTC')`, task); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO task_schedules(id,task_id,title,start_at,end_at,series_id,occurrence_date,timezone) VALUES
		('99999999999999999999999912',$1,'schedule-1','2026-01-01T09:00:00.000001Z','2026-01-01T10:00:00Z','99999999999999999999999912','2026-01-01','UTC'),
		('99999999999999999999999913',$1,'schedule-2','2026-01-01T09:00:00.000002Z','2026-01-01T10:00:00Z','99999999999999999999999913','2026-01-01','UTC'),
		('99999999999999999999999914',$1,'schedule-3','2026-01-01T09:00:00.000003Z','2026-01-01T10:00:00Z','99999999999999999999999914','2026-01-01','UTC')`, task); err != nil {
		t.Fatal(err)
	}

	projects := repository.NewProjectRepository(tx)
	firstProjects, err := projects.ListByUserIDCursor(ctx, domain.UserID(user), 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstProjects) != 2 || firstProjects[0].CursorCreatedAt != "2026-01-01T00:00:00.000003Z" {
		t.Fatalf("project cursor precision/order: %+v", firstProjects)
	}
	projectNext, err := projects.ListByUserIDCursor(ctx, domain.UserID(user), 2, &usecase.CursorAnchor{At: firstProjects[1].CursorCreatedAt, ID: firstProjects[1].ID})
	if err != nil || len(projectNext) != 1 || projectNext[0].ID == firstProjects[1].ID {
		t.Fatalf("project page boundary=%+v err=%v", projectNext, err)
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

	tags := repository.NewTaskTagRepository(tx)
	tagRows, err := tags.ListByUserIDCursor(ctx, domain.UserID(user), 2, nil)
	if err != nil || len(tagRows) != 2 || tagRows[0].Name != "alpha" || tagRows[1].Name != "bEta" {
		t.Fatalf("tag order=%+v err=%v", tagRows, err)
	}
	tagNext, err := tags.ListByUserIDCursor(ctx, domain.UserID(user), 2, &usecase.CursorAnchor{Name: tagRows[1].Name, ID: tagRows[1].ID})
	if err != nil || len(tagNext) != 1 || tagNext[0].Name != "GAMma" {
		t.Fatalf("tag boundary=%+v err=%v", tagNext, err)
	}

	todo := repository.NewTodoItemRepository(tx)
	todoRows, err := todo.ListByTaskCursor(ctx, domain.UserID(user), domain.TaskID(task), 2, nil)
	if err != nil || len(todoRows) != 2 || todoRows[0].Position != 1 || todoRows[1].Position != 2 {
		t.Fatalf("todo order=%+v err=%v", todoRows, err)
	}
	todoNext, err := todo.ListByTaskCursor(ctx, domain.UserID(user), domain.TaskID(task), 2, &usecase.CursorAnchor{Position: todoRows[1].Position, Date: todoRows[1].OccurrenceDate, ID: todoRows[1].ID})
	if err != nil || len(todoNext) != 1 || todoNext[0].ID == todoRows[1].ID {
		t.Fatalf("todo boundary=%+v err=%v", todoNext, err)
	}

	schedules := repository.NewTaskScheduleRepository(tx)
	scheduleRows, err := schedules.ListByTaskAndUserIDCursor(ctx, domain.UserID(user), domain.TaskID(task), 1, nil)
	if err != nil || len(scheduleRows) != 1 || scheduleRows[0].CursorStartAt != "2026-01-01T09:00:00.000001Z" {
		t.Fatalf("schedule precision/order=%+v err=%v", scheduleRows, err)
	}
	scheduleNext, err := schedules.ListByTaskAndUserIDCursor(ctx, domain.UserID(user), domain.TaskID(task), 2, &usecase.CursorAnchor{At: scheduleRows[0].CursorStartAt, ID: scheduleRows[0].ID})
	if err != nil || len(scheduleNext) != 2 || scheduleNext[0].ID == scheduleRows[0].ID {
		t.Fatalf("schedule boundary=%+v err=%v", scheduleNext, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, scheduleRows[0].CursorStartAt); err != nil {
		t.Fatal(err)
	}
}

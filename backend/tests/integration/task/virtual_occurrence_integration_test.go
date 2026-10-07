//go:build integration

package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type occurrenceTestID struct{}

func (occurrenceTestID) Generate() string { return ulid.Make().String() }

type occurrenceTestRepositories struct {
	usecase.Repositories
	task usecase.TaskRepository
	todo usecase.TodoItemRepository
}

func (r occurrenceTestRepositories) Tasks() usecase.TaskRepository         { return r.task }
func (r occurrenceTestRepositories) TodoItems() usecase.TodoItemRepository { return r.todo }

type occurrenceTestUOW struct{ pool *pgxpool.Pool }

func (u occurrenceTestUOW) Do(ctx context.Context, fn func(context.Context, usecase.Repositories) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repos := occurrenceTestRepositories{
		task: taskrepo.NewTaskRepository(tx), todo: taskrepo.NewTodoItemRepository(tx),
	}
	if err := fn(ctx, repos); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestVirtualOccurrenceMutationsPersistRealULIDs(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	uow := occurrenceTestUOW{pool: pool}
	id := occurrenceTestID{}
	monday, err := domain.NewTaskFrequency("mon")
	if err != nil {
		t.Fatal(err)
	}
	rootDate := recurrenceDate("2026-10-05")

	todoID := domain.TodoItemID(id.Generate())
	todo, err := domain.NewTodoItemWithDetails(todoID, taskID, "Weekly work", "", rootDate, false, 0, 1, domain.TaskFrequencies{monday})
	if err != nil {
		t.Fatal(err)
	}
	todo, err = todo.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(todoID), OccurrenceDate: rootDate, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := taskrepo.NewTodoItemRepository(pool).CreateForOwnedTask(ctx, userID, todo, false); err != nil {
		t.Fatal(err)
	}

	title := "Edited weekly work"
	updated, err := usecase.NewUpdateTodoItemUseCase(uow, nil, id).ExecuteOccurrence(ctx, userID, taskID, todoID, "2026-10-12", "current", usecase.PatchField[string]{Present: true, Value: &title}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{})
	if err != nil {
		t.Fatalf("update virtual todo: %v", err)
	}
	assertRealOccurrenceID(t, updated.ID)
	if err := usecase.NewCompleteTodoItemUseCase(uow, nil, id).ExecuteOccurrence(ctx, userID, taskID, todoID, "2026-10-19"); err != nil {
		t.Fatalf("complete virtual todo: %v", err)
	}

}

func assertRealOccurrenceID(t *testing.T, value string) {
	t.Helper()
	if value == usecase.VirtualOccurrenceID {
		t.Fatalf("virtual response ID was persisted: %s", value)
	}
	if _, err := ulid.ParseStrict(value); err != nil {
		t.Fatalf("occurrence ID %q is not a ULID: %v", value, err)
	}
}

func assertSavedOccurrence(t *testing.T, rows []dao.TodoItem, date string, completed bool) {
	t.Helper()
	for _, row := range rows {
		if row.OccurrenceDate == date {
			assertRealOccurrenceID(t, row.ID)
			if row.Completed != completed {
				t.Errorf("todo %s completed = %v, want %v", date, row.Completed, completed)
			}
			return
		}
	}
	t.Errorf("todo %s was not saved", date)
}

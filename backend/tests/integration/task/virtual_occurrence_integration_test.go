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
	task       usecase.TaskRepository
	actionItem usecase.ActionItemRepository
}

func (r occurrenceTestRepositories) Tasks() usecase.TaskRepository             { return r.task }
func (r occurrenceTestRepositories) ActionItems() usecase.ActionItemRepository { return r.actionItem }

type occurrenceTestUOW struct{ pool *pgxpool.Pool }

func (u occurrenceTestUOW) Do(ctx context.Context, fn func(context.Context, usecase.Repositories) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repos := occurrenceTestRepositories{
		task: taskrepo.NewTaskRepository(tx), actionItem: taskrepo.NewActionItemRepository(tx),
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

	actionItemID := domain.ActionItemID(id.Generate())
	actionItem, err := domain.NewActionItemWithDetails(actionItemID, taskID, "Weekly work", "", rootDate, false, 0, 1, domain.TaskFrequencies{monday})
	if err != nil {
		t.Fatal(err)
	}
	actionItem, err = actionItem.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(actionItemID), OccurrenceDate: rootDate, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := taskrepo.NewActionItemRepository(pool).CreateForOwnedTask(ctx, userID, actionItem, false); err != nil {
		t.Fatal(err)
	}

	title := "Edited weekly work"
	updated, err := usecase.NewUpdateActionItemUseCase(uow, nil, id).ExecuteOccurrence(ctx, userID, taskID, actionItemID, "2026-10-12", "current", usecase.PatchField[string]{Present: true, Value: &title}, usecase.PatchField[string]{}, usecase.PatchField[time.Time]{})
	if err != nil {
		t.Fatalf("update virtual actionItem: %v", err)
	}
	assertRealOccurrenceID(t, updated.ID)
	if err := usecase.NewCompleteActionItemUseCase(uow, nil, id).ExecuteOccurrence(ctx, userID, taskID, actionItemID, "2026-10-19"); err != nil {
		t.Fatalf("complete virtual actionItem: %v", err)
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

func assertSavedOccurrence(t *testing.T, rows []dao.ActionItem, date string, completed bool) {
	t.Helper()
	for _, row := range rows {
		if row.OccurrenceDate == date {
			assertRealOccurrenceID(t, row.ID)
			if row.Completed != completed {
				t.Errorf("actionItem %s completed = %v, want %v", date, row.Completed, completed)
			}
			return
		}
	}
	t.Errorf("actionItem %s was not saved", date)
}

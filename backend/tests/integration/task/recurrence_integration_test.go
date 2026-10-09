//go:build integration

package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

// These tests only connect to an explicitly configured, dedicated database.
// They never fall back to the application's default task2todaytodo database.
func recurrenceIntegrationPool(t *testing.T) *pgxpool.Pool {
	return testdb.Open(t)
}

func seedRecurrenceTask(t *testing.T, pool *pgxpool.Pool, timezone string) (domain.UserID, domain.TaskID) {
	t.Helper()
	ctx := t.Context()
	userID := domain.UserID(ulid.Make().String())
	taskID := domain.TaskID(ulid.Make().String())
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, first_name, last_name, email, password, timezone)
		VALUES ($1, 'Test', 'User', $2, 'not-used', $3)
	`, string(userID), string(userID)+"@example.test", timezone); err != nil {
		t.Fatalf("insert recurrence test user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tasks (id, user_id, title, priority, status)
		VALUES ($1, $2, 'Integration task', 'low', 'open')
	`, string(taskID), string(userID)); err != nil {
		t.Fatalf("insert recurrence test task: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM tasks WHERE id = $1`, string(taskID))
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, string(userID))
	})
	return userID, taskID
}

func recurrenceDate(value string) time.Time {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return date
}

func TestActionItemsAllowSameDayPositionTies(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	repo := taskrepo.NewActionItemRepository(pool)
	ctx := t.Context()
	date := recurrenceDate("2026-10-03")
	wantPosition := 4
	ids := []domain.ActionItemID{domain.ActionItemID(ulid.Make().String()), domain.ActionItemID(ulid.Make().String())}
	for index, id := range ids {
		item, err := domain.NewActionItemWithDetails(id, taskID, "Same-position item", "", date, false, wantPosition, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		item, err = item.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(id), OccurrenceDate: date, Timezone: "Asia/Tokyo"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateForOwnedTask(ctx, userID, item, false); err != nil {
			t.Fatalf("create tied item %d: %v", index, err)
		}
	}
	rows, err := repo.ListByTask(ctx, userID, taskID)
	if err != nil {
		t.Fatalf("list tied items: %v", err)
	}
	positions := make(map[string]int)
	for _, row := range rows {
		positions[row.ID] = row.Position
	}
	for _, id := range ids {
		if positions[string(id)] != wantPosition {
			t.Errorf("item %s position = %d, want tied position %d", id, positions[string(id)], wantPosition)
		}
	}
}

func TestCreateActionItemPositionStartsAtZeroForEachOccurrenceDate(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "UTC")
	repo := taskrepo.NewActionItemRepository(pool)
	ctx := t.Context()
	for index, dateText := range []string{"2026-10-03", "2026-10-04"} {
		date := recurrenceDate(dateText)
		id := domain.ActionItemID(ulid.Make().String())
		item, err := domain.NewActionItemWithDetails(id, taskID, "Day item", "", date, false, 0, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		item, err = item.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(id), OccurrenceDate: date, Timezone: "UTC"})
		if err != nil {
			t.Fatal(err)
		}
		created, err := repo.CreateForOwnedTask(ctx, userID, item, true)
		if err != nil {
			t.Fatalf("create item %d: %v", index, err)
		}
		if created.Position != 0 {
			t.Errorf("date %s position = %d, want 0 for its first item", dateText, created.Position)
		}
	}
}

func TestStopActionItemRecurrenceRetainsStoppedRootState(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	repo := taskrepo.NewActionItemRepository(pool)
	ctx := t.Context()
	seriesID := domain.ActionItemID(ulid.Make().String())
	rootDate := recurrenceDate("2026-10-03")
	frequency, err := domain.NewTaskFrequency("sat")
	if err != nil {
		t.Fatal(err)
	}
	root, err := domain.NewActionItemWithDetails(seriesID, taskID, "Weekly review", "", rootDate, false, 0, 1, domain.TaskFrequencies{frequency})
	if err != nil {
		t.Fatal(err)
	}
	root, err = root.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(seriesID), OccurrenceDate: rootDate, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateForOwnedTask(ctx, userID, root, false); err != nil {
		t.Fatalf("create root: %v", err)
	}
	if err := repo.StopActionItemRecurrence(ctx, userID, taskID, seriesID); err != nil {
		t.Fatalf("stop series: %v", err)
	}
	rootRow, err := repo.GetForOwnedTask(ctx, userID, taskID, seriesID)
	if err != nil {
		t.Fatalf("get stopped root: %v", err)
	}
	if rootRow.IntervalWeeks != 0 || rootRow.FrequencyAnchorDate != rootDate.Unix() || rootRow.RepeatState != "stopped" || len(rootRow.Frequencies) != 0 {
		t.Errorf("stopped root = %+v; want stopped recurring root with preserved anchor and no weekdays", rootRow)
	}
	var roots int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM action_items WHERE id = $1 AND series_id = id AND deleted_at IS NULL`, string(seriesID)).Scan(&roots); err != nil {
		t.Fatalf("read saved root occurrence: %v", err)
	}
	if roots != 1 {
		t.Errorf("saved first occurrence rows = %d, want 1", roots)
	}
}

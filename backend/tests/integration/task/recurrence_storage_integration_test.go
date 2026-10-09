//go:build integration

package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func TestInlineRecurrenceStorageAndActionItemSkippedChild(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	actionItemRepo := taskrepo.NewActionItemRepository(pool)
	actionItemID := domain.ActionItemID(ulid.Make().String())
	rootDate := recurrenceDate("2026-10-05")
	monday := domain.TaskFrequencies{{Value: "mon"}}
	actionItem, err := domain.NewActionItemWithDetails(actionItemID, taskID, "Weekly review", "notes", rootDate, false, 1, 1, monday)
	if err != nil {
		t.Fatal(err)
	}
	actionItem, err = actionItem.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(actionItemID), OccurrenceDate: rootDate, Timezone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := actionItemRepo.CreateForOwnedTask(ctx, userID, actionItem, true); err != nil {
		t.Fatalf("create actionItem root: %v", err)
	}
	root, err := actionItemRepo.GetForOwnedTask(ctx, userID, taskID, actionItemID)
	if err != nil || root.RepeatState != "active" || root.FrequencyAnchorDate != rootDate.Unix() {
		t.Fatalf("initial actionItem root = %+v, error %v", root, err)
	}
	newAnchor := recurrenceDate("2026-10-19")
	if err := actionItemRepo.SetActionItemRecurrence(ctx, userID, taskID, actionItemID, newAnchor, 2, []dao.TaskFrequency{{Value: "wed"}}); err != nil {
		t.Fatalf("replace actionItem recurrence: %v", err)
	}
	root, err = actionItemRepo.GetForOwnedTask(ctx, userID, taskID, actionItemID)
	if err != nil || root.FrequencyAnchorDate != newAnchor.Unix() || root.IntervalWeeks != 2 || len(root.Frequencies) != 1 || root.Frequencies[0].Value != "wed" {
		t.Fatalf("replaced actionItem root = %+v, error %v", root, err)
	}
	if err := actionItemRepo.SetActionItemSkippedOccurrence(ctx, userID, taskID, actionItemID, rootDate, true); err != nil {
		t.Fatalf("skip actionItem root occurrence: %v", err)
	}
	var rootDeleted, rootDateChildDeleted, rootDateChildSkipped bool
	if err := pool.QueryRow(ctx, `SELECT root.deleted_at IS NOT NULL, child.deleted_at IS NOT NULL, child.skipped_at IS NOT NULL FROM action_items root JOIN action_items child ON child.series_id = root.id AND child.occurrence_date = root.occurrence_date AND child.id <> child.series_id WHERE root.id = $1 AND root.id = root.series_id`, string(actionItemID)).Scan(&rootDeleted, &rootDateChildDeleted, &rootDateChildSkipped); err != nil {
		t.Fatalf("read skipped first actionItem occurrence: %v", err)
	}
	if rootDeleted || rootDateChildDeleted || !rootDateChildSkipped {
		t.Fatalf("skipping first actionItem occurrence set root deleted=%v, child deleted=%v, child skipped=%v", rootDeleted, rootDateChildDeleted, rootDateChildSkipped)
	}
	if err := actionItemRepo.SetActionItemSkippedOccurrence(ctx, userID, taskID, actionItemID, rootDate, false); err != nil {
		t.Fatalf("restore actionItem root occurrence: %v", err)
	}
	var firstActionItemChildCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM action_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(actionItemID), rootDate).Scan(&firstActionItemChildCount); err != nil {
		t.Fatal(err)
	}
	if firstActionItemChildCount != 0 {
		t.Fatalf("restored first actionItem skip marker count = %d, want 0", firstActionItemChildCount)
	}

	updated := actionItem
	updated.Title = "New weekly review"
	updated.Description = "future template"
	updated.DueDate = recurrenceDate("2026-10-19")
	firstSnapshotID := domain.ActionItemID(ulid.Make().String())
	if _, err := actionItemRepo.UpdateActionItemSeriesTemplate(ctx, userID, taskID, actionItemID, firstSnapshotID, updated); err != nil {
		t.Fatalf("update actionItem root template: %v", err)
	}
	updated.Title = "Latest template"
	secondSnapshotID := domain.ActionItemID(ulid.Make().String())
	if _, err := actionItemRepo.UpdateActionItemSeriesTemplate(ctx, userID, taskID, actionItemID, secondSnapshotID, updated); err != nil {
		t.Fatalf("update actionItem root template again: %v", err)
	}
	var snapshotID, snapshotTitle, rootTitle string
	if err := pool.QueryRow(ctx, `SELECT child.id, child.title, root.title FROM action_items root JOIN action_items child ON child.series_id = root.id AND child.occurrence_date = root.occurrence_date AND child.id <> child.series_id WHERE root.id = $1 AND root.id = root.series_id`, string(actionItemID)).Scan(&snapshotID, &snapshotTitle, &rootTitle); err != nil {
		t.Fatalf("read first actionItem snapshot: %v", err)
	}
	if snapshotID != string(firstSnapshotID) || snapshotTitle != "Weekly review" || rootTitle != "Latest template" {
		t.Fatalf("actionItem root/snapshot = %q/%q/%q; snapshot should stay original while root template changes", snapshotID, snapshotTitle, rootTitle)
	}

	skipDate := recurrenceDate("2026-10-26")
	if err := actionItemRepo.SetActionItemSkippedOccurrence(ctx, userID, taskID, actionItemID, skipDate, true); err != nil {
		t.Fatalf("skip actionItem occurrence: %v", err)
	}
	assertActionItemSkipped(t, ctx, pool, actionItemRepo, userID, taskID, actionItemID, skipDate, true)
	var skippedException bool
	if err := pool.QueryRow(ctx, `SELECT is_exception FROM action_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(actionItemID), skipDate).Scan(&skippedException); err != nil {
		t.Fatalf("read pure actionItem skip child: %v", err)
	}
	if skippedException {
		t.Fatal("pure actionItem skip marker is_exception = true, want false")
	}
	if err := actionItemRepo.SetActionItemSkippedOccurrence(ctx, userID, taskID, actionItemID, skipDate, false); err != nil {
		t.Fatalf("restore actionItem occurrence: %v", err)
	}
	assertActionItemSkipped(t, ctx, pool, actionItemRepo, userID, taskID, actionItemID, skipDate, false)
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM action_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(actionItemID), skipDate).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("restored pure actionItem marker count = %d, want 0", remaining)
	}

	editedDate := recurrenceDate("2026-11-02")
	editedID := domain.ActionItemID(ulid.Make().String())
	edited, err := domain.NewActionItemWithDetails(editedID, taskID, "Saved edit", "keep me", editedDate, false, 3, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	edited, err = edited.WithRecurrence(domain.RecurrenceMetadata{SeriesID: string(actionItemID), OccurrenceDate: editedDate, Timezone: "Asia/Tokyo", IsException: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := actionItemRepo.UpsertActionItemOverride(ctx, userID, edited); err != nil {
		t.Fatalf("save edited actionItem occurrence: %v", err)
	}
	if err := actionItemRepo.SetActionItemSkippedOccurrence(ctx, userID, taskID, actionItemID, editedDate, true); err != nil {
		t.Fatalf("skip edited actionItem occurrence: %v", err)
	}
	if err := actionItemRepo.SetActionItemSkippedOccurrence(ctx, userID, taskID, actionItemID, editedDate, false); err != nil {
		t.Fatalf("restore edited actionItem occurrence: %v", err)
	}
	var restoredTitle string
	var restoredDeleted, restoredSkipped bool
	if err := pool.QueryRow(ctx, `SELECT title, deleted_at IS NOT NULL, skipped_at IS NOT NULL FROM action_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(actionItemID), editedDate).Scan(&restoredTitle, &restoredDeleted, &restoredSkipped); err != nil {
		t.Fatalf("read restored edited actionItem: %v", err)
	}
	if restoredTitle != "Saved edit" || restoredDeleted || restoredSkipped {
		t.Fatalf("restored edited actionItem = %q deleted=%v skipped=%v, want saved content visible", restoredTitle, restoredDeleted, restoredSkipped)
	}
}

func TestRestoreCannotResurrectDeletedOccurrenceOrTask(t *testing.T) {
	pool := recurrenceIntegrationPool(t)
	userID, taskID := seedRecurrenceTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := localDate(time.Now(), location)
	rootDate := today.AddDate(0, 0, -7)
	futureDate := today.AddDate(0, 0, 7)
	actionItemID := domain.ActionItemID(ulid.Make().String())
	createWeeklyActionItemRoot(t, ctx, pool, userID, taskID, actionItemID, rootDate, location)
	actionItemRepo := taskrepo.NewActionItemRepository(pool)
	if err := actionItemRepo.SetActionItemSkippedOccurrence(ctx, userID, taskID, actionItemID, futureDate, true); err != nil {
		t.Fatalf("skip actionItem occurrence before deletion: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE action_items SET deleted_at = now() WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(actionItemID), futureDate); err != nil {
		t.Fatalf("delete skipped actionItem occurrence: %v", err)
	}
	if err := actionItemRepo.SetActionItemSkippedOccurrence(ctx, userID, taskID, actionItemID, futureDate, false); err != nil {
		t.Fatalf("restore deleted action item occurrence: %v", err)
	}
	var actionItemDeleted, actionItemSkipped bool
	if err := pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL, skipped_at IS NOT NULL FROM action_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id`, string(actionItemID), futureDate).Scan(&actionItemDeleted, &actionItemSkipped); err != nil {
		t.Fatalf("read deleted action item occurrence after restore: %v", err)
	}
	if !actionItemDeleted || !actionItemSkipped {
		t.Fatalf("deleted action item occurrence after restore: deleted=%v skipped=%v, want both markers retained", actionItemDeleted, actionItemSkipped)
	}
	if dates, err := actionItemRepo.ListActionItemSkippedOccurrences(ctx, userID, taskID, actionItemID); err != nil || len(dates) != 0 {
		t.Fatalf("list skipped actionItem dates after deletion: dates=%v error=%v, want no active skips", dates, err)
	}

}

func assertActionItemSkipped(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *taskrepo.ActionItemRepository, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, date time.Time, want bool) {
	t.Helper()
	dates, err := repo.ListActionItemSkippedOccurrences(ctx, userID, taskID, seriesID)
	if err != nil {
		t.Fatalf("list skipped actionItem dates: %v", err)
	}
	found := false
	for _, value := range dates {
		if value == date.Unix() {
			found = true
			break
		}
	}
	if found != want {
		t.Fatalf("skipped actionItem dates = %#v, contains %s = %v, want %v", dates, date.Format("2006-01-02"), found, want)
	}
	var rowCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM action_items WHERE series_id = $1 AND occurrence_date = $2 AND id <> series_id AND deleted_at IS NULL AND skipped_at IS NOT NULL`, string(seriesID), date).Scan(&rowCount); err != nil {
		t.Fatalf("count actionItem skip marker: %v", err)
	}
	if (rowCount > 0) != want {
		t.Fatalf("actionItem active skipped child rows = %d, want skipped=%v", rowCount, want)
	}
}

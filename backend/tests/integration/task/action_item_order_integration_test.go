//go:build integration

package task_test

import (
	"reflect"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
)

func TestActionItemReorderAndAppendAfterDeletingLastRow(t *testing.T) {
	pool := integrationPool(t)
	userID, taskID := seedTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	repo := taskrepo.NewActionItemRepository(pool)
	ids := []domain.ActionItemID{
		domain.ActionItemID(actionItemTestID{}.Generate()),
		domain.ActionItemID(actionItemTestID{}.Generate()),
		domain.ActionItemID(actionItemTestID{}.Generate()),
	}
	for index, id := range ids {
		item, err := domain.NewActionItem(id, taskID, "Ordered work", 30)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateForOwnedTask(ctx, userID, item, true); err != nil {
			t.Fatalf("create ActionItem %d: %v", index, err)
		}
	}
	assertItems := func(want []domain.ActionItemID) {
		t.Helper()
		items, err := repo.ListByTask(ctx, userID, taskID)
		if err != nil {
			t.Fatalf("list ActionItems: %v", err)
		}
		got := make([]domain.ActionItemID, len(items))
		for index, item := range items {
			got[index] = domain.ActionItemID(item.ID)
			if item.Position != index {
				t.Errorf("ActionItem %s position = %d, want %d", item.ID, item.Position, index)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ordered ActionItem IDs = %v, want %v", got, want)
		}
	}

	// Moving the first item to the end and then back to the beginning exercises
	// both directions of the position rewrite.
	if _, err := repo.ReorderForOwnedTask(ctx, userID, taskID, ids[0], 2); err != nil {
		t.Fatalf("move first ActionItem to the end: %v", err)
	}
	assertItems([]domain.ActionItemID{ids[1], ids[2], ids[0]})
	if _, err := repo.ReorderForOwnedTask(ctx, userID, taskID, ids[0], 0); err != nil {
		t.Fatalf("move last ActionItem to the beginning: %v", err)
	}
	assertItems(ids)

	if err := repo.DeleteForOwnedTask(ctx, userID, taskID, ids[2]); err != nil {
		t.Fatalf("soft-delete last ActionItem: %v", err)
	}
	appendedID := domain.ActionItemID(actionItemTestID{}.Generate())
	appended, err := domain.NewActionItem(appendedID, taskID, "Appended after delete", 30)
	if err != nil {
		t.Fatal(err)
	}
	appendedRow, err := repo.CreateForOwnedTask(ctx, userID, appended, true)
	if err != nil {
		t.Fatalf("append after deleting the last ActionItem: %v", err)
	}
	if appendedRow.Position != 2 {
		t.Fatalf("appended ActionItem position = %d, want 2", appendedRow.Position)
	}
	assertItems([]domain.ActionItemID{ids[0], ids[1], appendedID})
}

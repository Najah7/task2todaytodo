package domain

import (
	"testing"
	"time"
)

func TestNewActionItem(t *testing.T) {
	item, err := NewActionItem("action-item-1", "task-1", "Buy milk")
	if err != nil {
		t.Fatalf("NewActionItem() error = %v", err)
	}

	if item.ID != "action-item-1" || item.TaskID != "task-1" || item.Position != 0 {
		t.Errorf("action item = %+v, want ID, task ID, and position to be set", item)
	}
	if item.Description != "" || item.Completed || item.Position != 0 || item.IntervalWeeks != OnceIntervalWeeks {
		t.Errorf("action item = %+v, want only required values to be set", item)
	}
}

func TestNewActionItemWithDetails(t *testing.T) {
	frequencies := TaskFrequencies{mustTaskFrequency(t, "mon")}
	dueDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	item, err := NewActionItemWithDetails("action-item-1", "task-1", "Buy milk", "At the store", dueDate, true, 2, 3, frequencies)
	if err != nil {
		t.Fatalf("NewActionItemWithDetails() error = %v", err)
	}

	if item.Description != "At the store" || item.DueDate != dueDate || !item.Completed || item.Position != 2 || item.IntervalWeeks != 3 || !item.Frequencies.IsWeekday() {
		t.Errorf("action item = %+v, want details to be set", item)
	}
}

func TestNewActionItemValidation(t *testing.T) {
	tests := []struct {
		name          string
		id            ActionItemID
		taskID        TaskID
		title         string
		position      int
		intervalWeeks int
		wantErr       error
	}{
		{name: "empty ID", taskID: "task-1", title: "Action item", position: 0, intervalWeeks: 1, wantErr: ErrActionItemIDEmpty},
		{name: "empty task ID", id: "action-item-1", title: "Action item", position: 0, intervalWeeks: 1, wantErr: ErrActionItemTaskIDEmpty},
		{name: "blank title", id: "action-item-1", taskID: "task-1", title: " ", position: 0, intervalWeeks: 1, wantErr: ErrActionItemTitleEmpty},
		{name: "negative position", id: "action-item-1", taskID: "task-1", title: "Action item", position: -1, intervalWeeks: 1, wantErr: ErrActionItemPositionLess},
		{name: "interval weeks less than 0", id: "action-item-1", taskID: "task-1", title: "Action item", position: 0, intervalWeeks: -1, wantErr: ErrActionItemIntervalWeeksLess},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewActionItemWithDetails(tt.id, tt.taskID, tt.title, "", time.Time{}, false, tt.position, tt.intervalWeeks, nil)
			assertTaskDomainErrorIs(t, err, tt.wantErr)
			if got.ID != tt.id || got.TaskID != tt.taskID || got.Title != tt.title {
				t.Errorf("action item = %+v, want input values to be preserved", got)
			}
		})
	}
}

func TestActionItemRepeatPattern(t *testing.T) {
	once, err := NewActionItemWithDetails("action-item-1", "task-1", "Buy milk", "", time.Time{}, false, 0, 0, nil)
	if err != nil {
		t.Fatalf("NewActionItemWithDetails() once error = %v", err)
	}
	if !once.IsOnce() || once.IsWeekly() {
		t.Errorf("action item = %+v, want interval 0 to mean once", once)
	}

	weekly, err := NewActionItemWithDetails("action-item-1", "task-1", "Buy milk", "", time.Time{}, false, 0, 1, TaskFrequencies{mustTaskFrequency(t, "mon")})
	if err != nil {
		t.Fatalf("NewActionItemWithDetails() weekly error = %v", err)
	}
	if weekly.IsOnce() || !weekly.IsWeekly() || !weekly.IsEveryWeekday() {
		t.Errorf("action item = %+v, want interval 1 with weekday frequency to mean weekly weekday", weekly)
	}
}

func TestActionItemRecurrenceMetadata(t *testing.T) {
	occurrenceDate := time.Date(2026, 10, 6, 18, 30, 0, 0, time.FixedZone("JST", 9*60*60))
	item, err := NewActionItemWithRecurrence(ActionItem{
		ID: "action-item-1", TaskID: "task-1", Title: "Buy milk", DueDate: occurrenceDate,
		SeriesID: "action-item-1", OccurrenceDate: occurrenceDate, Timezone: "Asia/Tokyo", IntervalWeeks: 1,
	})
	if err != nil {
		t.Fatalf("NewActionItemWithRecurrence() error = %v", err)
	}
	if item.OccurrenceDate != time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC) {
		t.Errorf("occurrence date = %v, want normalized UTC calendar date", item.OccurrenceDate)
	}
	if item.SeriesID != item.ID || item.Timezone != "Asia/Tokyo" {
		t.Errorf("recurrence metadata = %+v, want root ID and series timezone", item)
	}
}

func TestNewExistingActionItemWithRecurrenceMetadata(t *testing.T) {
	createdAt := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	item, err := NewExistingActionItem(
		"action-item-1", "task-1", "Buy milk", "", time.Time{}, false, 0, 1, nil,
		createdAt, createdAt,
		RecurrenceMetadata{SeriesID: "series-1", OccurrenceDate: createdAt, Timezone: "Asia/Tokyo", IsException: true},
	)
	if err != nil {
		t.Fatalf("NewExistingActionItem() error = %v", err)
	}
	if item.SeriesID != "series-1" || item.Timezone != "Asia/Tokyo" || !item.IsException {
		t.Errorf("action item = %+v, want persisted recurrence metadata", item)
	}
}

func TestActionItemsSortByPosition(t *testing.T) {
	items := ActionItems{
		mustActionItemWithPosition(t, "action-item-3", 3, false),
		mustActionItemWithPosition(t, "action-item-1", 1, false),
		mustActionItemWithPosition(t, "action-item-2", 2, false),
	}

	got := items.SortByPosition()

	if got[0].ID != "action-item-1" || got[1].ID != "action-item-2" || got[2].ID != "action-item-3" {
		t.Fatalf("sorted action items = %+v, want ascending position", got)
	}
	if items[0].ID != "action-item-3" {
		t.Fatalf("original action items = %+v, want SortByPosition not to mutate receiver", items)
	}
}

func TestActionItemsHasIncomplete(t *testing.T) {
	tests := []struct {
		name  string
		items ActionItems
		want  bool
	}{
		{name: "empty", items: nil, want: false},
		{name: "all complete", items: ActionItems{
			mustActionItemWithPosition(t, "action-item-1", 1, true),
			mustActionItemWithPosition(t, "action-item-2", 2, true),
		}, want: false},
		{name: "has incomplete", items: ActionItems{
			mustActionItemWithPosition(t, "action-item-1", 1, true),
			mustActionItemWithPosition(t, "action-item-2", 2, false),
		}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.items.HasIncomplete(); got != tt.want {
				t.Fatalf("HasIncomplete() = %t, want %t", got, tt.want)
			}
		})
	}
}

func mustActionItemWithPosition(t *testing.T, id ActionItemID, position int, completed bool) ActionItem {
	t.Helper()
	item, err := NewActionItemWithDetails(id, "task-1", "Action item", "", time.Time{}, completed, position, 1, nil)
	if err != nil {
		t.Fatalf("NewActionItemWithDetails() error = %v", err)
	}
	return item
}

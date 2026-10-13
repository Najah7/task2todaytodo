package domain

import (
	"testing"
	"time"
)

func TestInsertScheduleByStartUsesNearestInternalTimeAndCurrentDisplayPosition(t *testing.T) {
	base := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	items := []TodoItem{
		{ID: "manual", TodoID: "todo", Kind: FrameActionItem, ActionItemID: "a", PlannedMinutes: ptr(30), Position: 0},
		{ID: "later-display", TodoID: "todo", Kind: FrameSchedule, ScheduleRuleID: "later", OccurrenceDate: base, Start: timePtr(base.Add(5 * time.Hour)), End: timePtr(base.Add(6 * time.Hour)), Position: 1},
		{ID: "nearest-time", TodoID: "todo", Kind: FrameTask, TaskID: "task", PlannedMinutes: ptr(30), Start: timePtr(base.Add(4 * time.Hour)), End: timePtr(base.Add(4*time.Hour + 30*time.Minute)), Position: 2},
		{ID: "new", TodoID: "todo", Kind: FrameSchedule, ScheduleRuleID: "new", OccurrenceDate: base, Start: timePtr(base.Add(time.Hour)), End: timePtr(base.Add(2 * time.Hour)), Position: 3},
		{ID: "last", TodoID: "todo", Kind: FrameActionItem, ActionItemID: "b", PlannedMinutes: ptr(30), Start: timePtr(base.Add(6 * time.Hour)), End: timePtr(base.Add(6*time.Hour + 30*time.Minute)), Position: 4},
	}
	newStart := base.Add(3 * time.Hour)
	got, err := InsertScheduleByStart(items, "new", newStart)
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, "manual", "later-display", "new", "nearest-time", "last")
	if !got[2].Start.Equal(newStart) {
		t.Fatalf("inserted Schedule start = %v, want %v", got[2].Start, newStart)
	}
	if !got[1].Start.Equal(*items[1].Start) || !got[3].Start.Equal(*items[2].Start) {
		t.Fatal("insertion changed another item's internal time")
	}
}

func TestInsertScheduleByStartAppendsWhenThereIsNoTimedAnchor(t *testing.T) {
	base := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	items := []TodoItem{
		{ID: "work", TodoID: "todo", Kind: FrameActionItem, ActionItemID: "a", PlannedMinutes: ptr(30), Position: 0},
		{ID: "new", TodoID: "todo", Kind: FrameSchedule, ScheduleRuleID: "new", OccurrenceDate: base, Start: timePtr(base), End: timePtr(base.Add(time.Hour)), Position: 1},
	}
	got, err := InsertScheduleByStart(items, "new", base.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, "work", "new")
}

func TestInsertScheduleByStartIgnoresUntimedAndHiddenAnchors(t *testing.T) {
	base := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	items := []TodoItem{
		{ID: "untimed", TodoID: "todo", Kind: FrameTask, TaskID: "task", PlannedMinutes: ptr(30), Position: 0},
		{ID: "hidden", TodoID: "todo", Kind: FrameSchedule, ScheduleRuleID: "hidden", OccurrenceDate: base, Start: timePtr(base.Add(2 * time.Hour)), End: timePtr(base.Add(3 * time.Hour)), Hidden: true, Position: 1},
		{ID: "new", TodoID: "todo", Kind: FrameSchedule, ScheduleRuleID: "new", OccurrenceDate: base, Start: timePtr(base), End: timePtr(base.Add(time.Hour)), Position: 2},
	}
	got, err := InsertScheduleByStart(items, "new", base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, "untimed", "new", "hidden")
}

func TestInsertSchedulesByStartRemovesPendingTargetsFromAnchors(t *testing.T) {
	base := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	items := []TodoItem{
		{ID: "work", TodoID: "todo", Kind: FrameActionItem, ActionItemID: "a", PlannedMinutes: ptr(30), Start: timePtr(base.Add(2 * time.Hour)), End: timePtr(base.Add(3 * time.Hour)), Position: 0},
		{ID: "first", TodoID: "todo", Kind: FrameSchedule, ScheduleRuleID: "first", OccurrenceDate: base, Start: timePtr(base), End: timePtr(base.Add(time.Hour)), Position: 1},
		{ID: "second", TodoID: "todo", Kind: FrameSchedule, ScheduleRuleID: "second", OccurrenceDate: base, Start: timePtr(base.Add(time.Hour)), End: timePtr(base.Add(2 * time.Hour)), Position: 2},
	}
	start := base.Add(time.Hour)
	got, err := InsertSchedulesByStart(items, []ScheduleInsertion{{ItemID: "first", Start: start}, {ItemID: "second", Start: start}})
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, "first", "second", "work")
}

func assertIDs(t *testing.T, items []TodoItem, want ...string) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("IDs length=%d, want %d: %+v", len(items), len(want), items)
	}
	for index, item := range items {
		if item.ID != want[index] || item.Position != index {
			t.Fatalf("item[%d]=(%s, position %d), want (%s, position %d)", index, item.ID, item.Position, want[index], index)
		}
	}
}

func timePtr(value time.Time) *time.Time { return &value }

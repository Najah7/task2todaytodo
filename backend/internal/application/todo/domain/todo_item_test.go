package domain

import (
	"errors"
	"testing"
	"time"
)

func TestTodoItemFactoriesEnforcePersistedShape(t *testing.T) {
	start := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	minutes := 60
	tests := []struct {
		name string
		item TodoItem
		want error
	}{
		{"valid action", TodoItem{ID: "a", TodoID: "l", Kind: FrameActionItem, ActionItemID: "source", PlannedMinutes: &minutes, Start: &start, End: &end}, nil},
		{"valid Task", TodoItem{ID: "task", TodoID: "l", Kind: FrameTask, TaskID: "source", PlannedMinutes: &minutes, Start: &start, End: &end}, nil},
		{"Task cannot also reference ActionItem", TodoItem{ID: "task", TodoID: "l", Kind: FrameTask, TaskID: "source", ActionItemID: "action", PlannedMinutes: &minutes}, ErrInvalidTodoItem},
		{"action needs positive minutes", TodoItem{ID: "a", TodoID: "l", Kind: FrameActionItem, ActionItemID: "source", PlannedMinutes: ptr(0)}, ErrInvalidTodoItem},
		{"action cannot be hidden", TodoItem{ID: "a", TodoID: "l", Kind: FrameActionItem, ActionItemID: "source", PlannedMinutes: &minutes, Hidden: true}, ErrInvalidTodoItem},
		{"schedule has null work amount", TodoItem{ID: "s", TodoID: "l", Kind: FrameSchedule, ScheduleRuleID: "r", OccurrenceDate: start, Start: &start, End: &end}, nil},
		{"schedule requires time", TodoItem{ID: "s", TodoID: "l", Kind: FrameSchedule, ScheduleRuleID: "r", OccurrenceDate: start}, ErrInvalidTodoItem},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.item.Validate()
			if err != test.want {
				t.Fatalf("Validate()=%v, want %v", err, test.want)
			}
		})
	}
}

func TestTodoScheduleCannotHideWhenCompleted(t *testing.T) {
	start := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	item, err := NewScheduleTodoItem("s", "l", "rule", start, start, start.Add(time.Hour), true, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := item.SetHidden(true); err != ErrInvalidTodoItem {
		t.Fatalf("SetHidden() error=%v, want ErrInvalidTodoItem", err)
	}
}

func TestTodoTimezoneChangeRequiresValidTimezone(t *testing.T) {
	date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	todo, err := NewTodo("todo", "user", date, "America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := todo.WithTimezone("America/Vancouver")
	if err != nil || updated.Timezone != "America/Vancouver" || todo.Timezone != "America/Los_Angeles" {
		t.Fatalf("WithTimezone()=(%+v,%v), want a validated copy", updated, err)
	}
	if _, err := todo.WithTimezone("invalid-zone"); !errors.Is(err, ErrInvalidTodo) {
		t.Fatalf("WithTimezone(invalid) error=%v, want ErrInvalidTodo", err)
	}
}

func ptr(value int) *int { return &value }

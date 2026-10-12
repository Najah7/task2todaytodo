package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
)

func TestPlanTodoItemsRestoresOrPreservesHiddenScheduleWithoutDuplicates(t *testing.T) {
	date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	start := date.Add(9 * time.Hour)
	end := start.Add(time.Hour)
	saved := []dao.Item{{
		ID: "schedule-frame", TodoID: "todo", Kind: string(domain.FrameSchedule),
		ScheduleRuleID: "rule", OccurrenceDate: date, ScheduledStart: &start,
		ScheduledEnd: &end, Hidden: true, Position: 0,
	}}
	newID := func() string { return "new" }

	t.Run("restore replaces hidden row with visible projection", func(t *testing.T) {
		plan := domain.Plan{Frames: []domain.Frame{{
			Kind: domain.FrameSchedule, RuleID: "rule", OccurrenceDate: date,
			Start: &start, End: &end,
		}}}
		items, err := planTodoItems("todo", saved, plan, newID)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].ID != "schedule-frame" || items[0].Hidden {
			t.Fatalf("items=%+v, want one visible occurrence retaining its ID", items)
		}
	})

	t.Run("regeneration preserves hidden row", func(t *testing.T) {
		items, err := planTodoItems("todo", saved, domain.Plan{}, newID)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].ID != "schedule-frame" || !items[0].Hidden {
			t.Fatalf("items=%+v, want one preserved hidden occurrence", items)
		}
	})
}

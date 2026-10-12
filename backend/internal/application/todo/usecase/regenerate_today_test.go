package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
)

func TestSavedScheduleVisibilityIsPreservedOnlyForManualReflow(t *testing.T) {
	date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	sources := dao.PlanningSources{Schedules: []dao.ScheduleOccurrence{{RuleID: "rule", OccurrenceDate: date, Hidden: true}}}
	saved := []dao.Item{{Kind: "schedule", ScheduleRuleID: "rule", OccurrenceDate: date, Hidden: true}}

	manual := withSavedScheduleVisibility(sources, saved, true)
	if !manual.Schedules[0].Hidden {
		t.Fatal("manual reorder did not preserve hidden Schedule")
	}
	full := withSavedScheduleVisibility(sources, saved, false)
	if full.Schedules[0].Hidden {
		t.Fatal("full regeneration retained hidden Schedule")
	}
}

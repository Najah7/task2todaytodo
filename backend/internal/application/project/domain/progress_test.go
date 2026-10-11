package domain

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

func TestCalculateProjectProgressWeightsTaskPercentAndScheduleOccurrences(t *testing.T) {
	asOf := time.Date(2026, time.October, 4, 16, 0, 0, 0, time.UTC) // Tokyo local date is Oct 5.
	virtual := ScheduleProgressRule{Rule: recurrence.BlueprintRule{
		Start:      time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC),
		End:        time.Date(2026, 10, 5, 11, 0, 0, 0, time.UTC),
		AnchorDate: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
		Timezone:   "Asia/Tokyo", Frequency: "weekly", Interval: 1, Weekdays: []time.Weekday{time.Monday},
	}}
	got, err := CalculateProjectProgress(
		[]TaskProgressFacts{
			{Progress: 50},
			{Progress: 100}, // Done Task remains a 100% item.
		},
		[]ScheduleProgressFacts{
			{Total: 1, Completed: 1, Roots: []ScheduleProgressRule{virtual}},
		},
		asOf,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != 62 { // (50 + 100 + 100 + 0) / 4.
		t.Fatalf("project progress = %d, want 62", got)
	}
}

func TestCalculateProjectProgressReturnsScheduleRecurrenceErrors(t *testing.T) {
	malformed := ScheduleProgressRule{Rule: recurrence.BlueprintRule{Timezone: "Asia/Tokyo", Frequency: "unsupported", Interval: 1}}
	if _, err := CalculateProjectProgress(nil, []ScheduleProgressFacts{{Roots: []ScheduleProgressRule{malformed}}}, time.Now()); err == nil {
		t.Fatal("Schedule malformed root returned nil error")
	}
}

func TestCalculateProjectProgressReturnsZeroForNoTasksOrEligibleSchedules(t *testing.T) {
	got, err := CalculateProjectProgress(nil, []ScheduleProgressFacts{{Total: 0}}, time.Now())
	if err != nil || got != 0 {
		t.Fatalf("empty project progress = %d, error %v; want 0, nil", got, err)
	}
}

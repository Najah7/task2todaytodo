package domain

import (
	"testing"
	"time"

	sharedprogress "github.com/Najah7/task2todaytodo/internal/application/shared/progress"
)

func TestCalculateProjectProgressWeightsTaskPercentAndScheduleOccurrences(t *testing.T) {
	asOf := time.Date(2026, time.October, 4, 16, 0, 0, 0, time.UTC) // Tokyo local date is Oct 5.
	virtual := sharedprogress.RecurrenceRule{
		OccurrenceDate: "2026-10-05", FrequencyAnchorDate: "2026-10-05",
		Timezone: "Asia/Tokyo", IntervalWeeks: 1, Frequencies: []string{"mon"},
	}
	got, err := CalculateProjectProgress(
		[]TaskProgressFacts{
			{Done: false, Total: 2, Completed: 1}, // 50% after flooring.
			{Done: true, Total: 0, Completed: 0},  // Done Task remains a 100% item.
		},
		[]ScheduleProgressFacts{
			{Total: 1, Completed: 1, Roots: []sharedprogress.RecurrenceRule{virtual}},
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

func TestCalculateProjectProgressReturnsZeroForNoTasksOrEligibleSchedules(t *testing.T) {
	got, err := CalculateProjectProgress(nil, []ScheduleProgressFacts{{Total: 0}}, time.Now())
	if err != nil || got != 0 {
		t.Fatalf("empty project progress = %d, error %v; want 0, nil", got, err)
	}
}

func TestCalculateProjectProgressIgnoresMalformedTaskRootsButReturnsScheduleErrors(t *testing.T) {
	malformed := sharedprogress.RecurrenceRule{OccurrenceDate: "bad-date", Timezone: "Asia/Tokyo", IntervalWeeks: 1}
	got, err := CalculateProjectProgress([]TaskProgressFacts{{Done: false, Roots: []sharedprogress.RecurrenceRule{malformed}}}, nil, time.Now())
	if err != nil || got != 0 {
		t.Fatalf("Task malformed root progress = %d, error %v; want 0, nil", got, err)
	}
	if _, err := CalculateProjectProgress(nil, []ScheduleProgressFacts{{Roots: []sharedprogress.RecurrenceRule{malformed}}}, time.Now()); err == nil {
		t.Fatal("Schedule malformed root returned nil error")
	}
}

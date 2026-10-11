package domain

import "testing"

func TestCalculateProjectProgressWeightsTasksAndPersistedScheduleOccurrences(t *testing.T) {
	got := CalculateProjectProgress(
		[]TaskProgressFacts{
			{Progress: 50},
			{Progress: 100}, // Done Task remains a 100% item.
		},
		[]ScheduleProgressFacts{{Total: 2, Completed: 1}},
	)
	if got != 62 { // (50 + 100 + 100 + 0) / 4.
		t.Fatalf("project progress = %d, want 62", got)
	}
}

func TestCalculateProjectProgressReturnsZeroForNoPersistedWork(t *testing.T) {
	if got := CalculateProjectProgress(nil, []ScheduleProgressFacts{{Total: 0}}); got != 0 {
		t.Fatalf("empty project progress = %d, want 0", got)
	}
}

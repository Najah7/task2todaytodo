package progress

import (
	"testing"
	"time"
)

func TestTaskPercent(t *testing.T) {
	for _, test := range []struct {
		name      string
		done      bool
		completed int
		total     int
		want      int
	}{
		{name: "done exposes one hundred without children", done: true, want: 100},
		{name: "floor partial percentage", completed: 1, total: 3, want: 33},
		{name: "full saved work", completed: 4, total: 4, want: 100},
		{name: "no eligible work", want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := TaskPercent(test.done, test.completed, test.total); got != test.want {
				t.Fatalf("TaskPercent() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestVirtualOccurrenceOccursTodayUsesLocalDateAndSavedOverride(t *testing.T) {
	asOf := time.Date(2026, time.October, 5, 16, 0, 0, 0, time.UTC) // Tokyo local date is Oct 6.
	rule := RecurrenceRule{
		OccurrenceDate: "2026-10-06", Timezone: "Asia/Tokyo", IntervalWeeks: 1,
		FrequencyAnchorDate: "2026-10-06", Frequencies: []string{"tue"},
	}
	got, err := VirtualOccurrenceOccursToday(rule, asOf)
	if err != nil || !got {
		t.Fatalf("local-today virtual occurrence = %v, error %v; want true, nil", got, err)
	}

	rule.OccurrenceSavedToday = true
	got, err = VirtualOccurrenceOccursToday(rule, asOf)
	if err != nil || got {
		t.Fatalf("saved local-today occurrence = %v, error %v; want false, nil", got, err)
	}
}

func TestVirtualOccurrenceOccursTodayExcludesFutureDates(t *testing.T) {
	rule := RecurrenceRule{
		OccurrenceDate: "2026-10-12", Timezone: "Asia/Tokyo", IntervalWeeks: 1,
		FrequencyAnchorDate: "2026-10-05", Frequencies: []string{"mon"},
	}
	got, err := VirtualOccurrenceOccursToday(rule, time.Date(2026, time.October, 5, 3, 0, 0, 0, time.UTC))
	if err != nil || got {
		t.Fatalf("future virtual occurrence = %v, error %v; want false, nil", got, err)
	}
}

func TestVirtualScheduleOccurrenceRejectsNonexistentDaylightSavingWallTime(t *testing.T) {
	rule := RecurrenceRule{
		OccurrenceDate: "2026-03-01", Timezone: "America/New_York", IntervalWeeks: 1,
		FrequencyAnchorDate: "2026-03-01", Frequencies: []string{"sun"},
		StartAt: "2026-03-01T07:30:00Z", EndAt: "2026-03-01T08:30:00Z",
	}
	asOf := time.Date(2026, time.March, 8, 16, 0, 0, 0, time.UTC) // Mar 8 is the spring clock change.
	got, err := VirtualOccurrenceOccursToday(rule, asOf)
	if err != nil || got {
		t.Fatalf("DST-gap virtual Schedule = %v, error %v; want false, nil", got, err)
	}
}

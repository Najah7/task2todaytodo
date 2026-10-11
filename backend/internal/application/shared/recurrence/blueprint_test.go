package recurrence

import (
	"testing"
	"time"
)

func TestGenerateBlueprintOccurrencesCalendarRules(t *testing.T) {
	tests := []struct {
		name      string
		rule      BlueprintRule
		from      string
		through   string
		wantDates []string
	}{
		{
			name: "daily interval",
			rule: BlueprintRule{Start: local("2026-01-01T09:00:00"), End: local("2026-01-01T10:00:00"), Timezone: "UTC", Frequency: "daily", Interval: 2},
			from: "2026-01-01", through: "2026-01-06", wantDates: []string{"2026-01-01", "2026-01-03", "2026-01-05"},
		},
		{
			name: "weekly interval and weekdays",
			rule: BlueprintRule{Start: local("2026-01-05T09:00:00"), End: local("2026-01-05T10:00:00"), Timezone: "UTC", Frequency: "weekly", Interval: 2, Weekdays: []time.Weekday{time.Monday, time.Wednesday}},
			from: "2026-01-05", through: "2026-01-21", wantDates: []string{"2026-01-05", "2026-01-07", "2026-01-19", "2026-01-21"},
		},
		{
			name: "weekly anchor date does not bypass weekday selection",
			rule: BlueprintRule{Start: local("2026-01-06T09:00:00"), End: local("2026-01-06T10:00:00"), Timezone: "UTC", Frequency: "weekly", Interval: 1, Weekdays: []time.Weekday{time.Monday, time.Wednesday}},
			from: "2026-01-06", through: "2026-01-08", wantDates: []string{"2026-01-07"},
		},
		{
			name: "monthly keeps original day anchor",
			rule: BlueprintRule{Start: local("2026-01-31T09:00:00"), End: local("2026-01-31T10:00:00"), Timezone: "UTC", Frequency: "monthly", Interval: 1},
			from: "2026-01-01", through: "2026-04-30", wantDates: []string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"},
		},
		{
			name: "yearly leap day clamps without drifting",
			rule: BlueprintRule{Start: local("2024-02-29T09:00:00"), End: local("2024-02-29T10:00:00"), Timezone: "UTC", Frequency: "yearly", Interval: 1},
			from: "2024-01-01", through: "2028-12-31", wantDates: []string{"2024-02-29", "2025-02-28", "2026-02-28", "2027-02-28", "2028-02-29"},
		},
		{
			name: "once is a single inclusive occurrence",
			rule: BlueprintRule{Start: local("2026-01-02T09:00:00"), End: local("2026-01-02T10:00:00"), Timezone: "UTC", Frequency: "once", Interval: 1},
			from: "2026-01-01", through: "2026-01-04", wantDates: []string{"2026-01-02"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			occurrences, err := GenerateBlueprintOccurrences(test.rule, date(test.from), date(test.through))
			if err != nil {
				t.Fatal(err)
			}
			if len(occurrences) != len(test.wantDates) {
				t.Fatalf("got dates %v; want %v", occurrenceDates(occurrences), test.wantDates)
			}
			for i, occurrence := range occurrences {
				if got := occurrence.Date.Format("2006-01-02"); got != test.wantDates[i] {
					t.Fatalf("date[%d]=%s; want %s", i, got, test.wantDates[i])
				}
			}
		})
	}
}

func TestGenerateBlueprintOccurrencesInclusiveEndAndMultiDay(t *testing.T) {
	end := date("2026-01-05")
	rule := BlueprintRule{Start: local("2026-01-01T23:00:00"), End: local("2026-01-02T01:00:00"), Timezone: "UTC", Frequency: "daily", Interval: 1, EndDate: &end}
	occurrences, err := GenerateBlueprintOccurrences(rule, date("2026-01-01"), date("2026-01-10"))
	if err != nil {
		t.Fatal(err)
	}
	if len(occurrences) != 5 {
		t.Fatalf("got %d occurrences; want inclusive five-day range", len(occurrences))
	}
	if got := occurrences[4].EndAt.Sub(occurrences[4].StartAt); got != 2*time.Hour {
		t.Fatalf("multi-day duration=%s; want 2h", got)
	}
}

func TestGenerateBlueprintOccurrencesDSTGapAndEarlierFold(t *testing.T) {
	gap := BlueprintRule{Start: local("2026-03-08T02:30:00"), End: local("2026-03-08T03:30:00"), Timezone: "America/New_York", Frequency: "once", Interval: 1}
	occurrences, err := GenerateBlueprintOccurrences(gap, date("2026-03-08"), date("2026-03-08"))
	if err != nil || len(occurrences) != 0 {
		t.Fatalf("spring gap: occurrences=%v err=%v; want none", occurrences, err)
	}
	fold := BlueprintRule{Start: local("2026-11-01T01:30:00"), End: local("2026-11-01T02:30:00"), Timezone: "America/New_York", Frequency: "once", Interval: 1}
	occurrences, err = GenerateBlueprintOccurrences(fold, date("2026-11-01"), date("2026-11-01"))
	if err != nil || len(occurrences) != 1 {
		t.Fatalf("fall fold: occurrences=%v err=%v; want one", occurrences, err)
	}
	if got, want := occurrences[0].StartAt.UTC().Format(time.RFC3339), "2026-11-01T05:30:00Z"; got != want {
		t.Fatalf("fold start=%s; want earlier instant %s", got, want)
	}
}

func TestGenerateBlueprintOccurrencesBeforeAnchorRangeReturnsEmpty(t *testing.T) {
	rule := BlueprintRule{Start: local("2026-04-10T09:00:00"), End: local("2026-04-10T10:00:00"), Timezone: "UTC", Frequency: "daily", Interval: 1}
	occurrences, err := GenerateBlueprintOccurrences(rule, date("2026-04-01"), date("2026-04-09"))
	if err != nil {
		t.Fatal(err)
	}
	if len(occurrences) != 0 {
		t.Fatalf("got %d occurrences before anchor; want none", len(occurrences))
	}
}

func TestGenerateBlueprintOccurrencesFollowingRevisionKeepsClampAnchor(t *testing.T) {
	rule := BlueprintRule{
		Start: local("2026-02-28T09:00:00"), AnchorDate: date("2026-01-31"),
		End: local("2026-02-28T10:00:00"), Timezone: "UTC", Frequency: "monthly", Interval: 1,
	}
	occurrences, err := GenerateBlueprintOccurrences(rule, date("2026-02-01"), date("2026-04-30"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := occurrenceDates(occurrences), []string{"2026-02-28", "2026-03-31", "2026-04-30"}; !equalStrings(got, want) {
		t.Fatalf("dates=%v; want %v", got, want)
	}
	rule.Interval = 2
	occurrences, err = GenerateBlueprintOccurrences(rule, date("2026-02-01"), date("2026-06-30"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := occurrenceDates(occurrences), []string{"2026-02-28", "2026-04-30", "2026-06-30"}; !equalStrings(got, want) {
		t.Fatalf("two-month interval dates=%v; want %v", got, want)
	}
}

func TestGenerateBlueprintOccurrencesResolvesLocalClockAndMultiDayEnd(t *testing.T) {
	rule := BlueprintRule{Start: local("2026-06-01T23:00:00"), End: local("2026-06-02T01:30:00"), Timezone: "America/Los_Angeles", Frequency: "daily", Interval: 1}
	occurrences, err := GenerateBlueprintOccurrences(rule, date("2026-06-01"), date("2026-06-01"))
	if err != nil || len(occurrences) != 1 {
		t.Fatalf("occurrences=%v err=%v; want one", occurrences, err)
	}
	if got, want := occurrences[0].StartAt.UTC().Format(time.RFC3339), "2026-06-02T06:00:00Z"; got != want {
		t.Fatalf("start=%s; want %s", got, want)
	}
	if got := occurrences[0].EndAt.Sub(occurrences[0].StartAt); got != 150*time.Minute {
		t.Fatalf("duration=%s; want 2h30m", got)
	}
}

func local(value string) time.Time {
	parsed, err := time.Parse("2006-01-02T15:04:05", value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func date(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func occurrenceDates(occurrences []BlueprintOccurrence) []string {
	dates := make([]string, len(occurrences))
	for i, occurrence := range occurrences {
		dates[i] = occurrence.Date.Format("2006-01-02")
	}
	return dates
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

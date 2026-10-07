package recurrence

import (
	"reflect"
	"testing"
	"time"
)

func TestGenerateRecurrenceDatesUsesSeriesTimezoneAndFourWeekIntervals(t *testing.T) {
	first := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 5, 16, 30, 0, 0, time.UTC)
	got, err := GenerateRecurrenceDates(first, now, FourWeekIntervalWeeks, nil, "America/Los_Angeles", 30)
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Time{time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)}
	gotDates := make([]time.Time, len(got))
	for i, occurrence := range got {
		gotDates[i] = occurrence.Date
	}
	if !reflect.DeepEqual(gotDates, want) || !got[0].Root || got[1].Root {
		t.Fatalf("dates/root flags = %v/%t/%t, want %v/true/false", gotDates, got[0].Root, got[1].Root, want)
	}
}

func TestGenerateRecurrenceDatesHonorsSelectedWeekdays(t *testing.T) {
	first := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	got, err := GenerateRecurrenceDates(first, first, WeeklyIntervalWeeks, Frequencies{{Value: "mon"}, {Value: "wed"}}, "UTC", RecurrenceHorizonDays)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 || !got[0].Root || got[1].Date.Format("2006-01-02") != "2026-10-07" {
		t.Fatalf("occurrences = %+v, want Monday and Wednesday weekly dates", got)
	}
}

func TestGenerateRecurrenceDatesValidatesInput(t *testing.T) {
	first := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	if _, err := GenerateRecurrenceDates(first, first, -1, nil, "UTC", 30); err != ErrRecurrenceIntervalWeeksLess {
		t.Errorf("negative interval error = %v", err)
	}
	if _, err := GenerateRecurrenceDates(first, first, 1, nil, "invalid/zone", 30); err != ErrRecurrenceTimezoneInvalid {
		t.Errorf("invalid timezone error = %v", err)
	}
	if _, err := GenerateRecurrenceDates(first, first, 1, nil, "UTC", -1); err != ErrRecurrenceHorizonInvalid {
		t.Errorf("negative horizon error = %v", err)
	}
}

func TestGenerateRecurrenceDatesFromAnchorLimitAndCount(t *testing.T) {
	anchor := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	frequencies := Frequencies{{Value: "mon"}, {Value: "wed"}}
	from := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	got, err := GenerateRecurrenceDatesFromAnchorLimit(anchor, from, WeeklyIntervalWeeks, frequencies, "UTC", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Date.Format("2006-01-02") != "2026-10-14" || got[1].Date.Format("2006-01-02") != "2026-10-19" {
		t.Fatalf("dates = %+v, want October 14 and 19", got)
	}
	count, err := CountRecurrenceDatesFromAnchor(anchor, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), WeeklyIntervalWeeks, frequencies, "UTC")
	if err != nil || count != 5 {
		t.Fatalf("CountRecurrenceDatesFromAnchor() = %d, %v; want 5, nil", count, err)
	}
}

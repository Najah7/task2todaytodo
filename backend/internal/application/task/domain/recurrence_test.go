package domain

import (
	"reflect"
	"testing"
	"time"
)

func TestGenerateRecurrenceDatesUsesSeriesTimezoneAndFourWeekIntervals(t *testing.T) {
	first := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC) // Stored series-local calendar date.
	now := time.Date(2026, 10, 5, 16, 30, 0, 0, time.UTC) // Still Monday in Los Angeles.
	got, err := GenerateRecurrenceDates(first, now, FourWeekIntervalWeeks, nil, "America/Los_Angeles", 30)
	if err != nil {
		t.Fatalf("GenerateRecurrenceDates() error = %v", err)
	}
	want := []time.Time{
		time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
	}
	gotDates := make([]time.Time, len(got))
	for index, occurrence := range got {
		gotDates[index] = occurrence.Date
	}
	if !reflect.DeepEqual(gotDates, want) {
		t.Fatalf("occurrence dates = %v, want %v", gotDates, want)
	}
	if !got[0].Root || got[1].Root {
		t.Fatalf("root markers = [%t %t], want [true false]", got[0].Root, got[1].Root)
	}
}

func TestGenerateRecurrenceDatesHonorsSelectedWeekdays(t *testing.T) {
	first := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC) // Monday.
	frequencies := TaskFrequencies{mustTaskFrequency(t, "mon"), mustTaskFrequency(t, "wed")}
	got, err := GenerateRecurrenceDates(first, first, WeeklyIntervalWeeks, frequencies, "UTC", RecurrenceHorizonDays)
	if err != nil {
		t.Fatalf("GenerateRecurrenceDates() error = %v", err)
	}
	var gotDates []time.Time
	for _, occurrence := range got {
		gotDates = append(gotDates, occurrence.Date)
	}
	want := []time.Time{
		time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC),
	}
	if !reflect.DeepEqual(gotDates, want) {
		t.Fatalf("occurrence dates = %v, want %v", gotDates, want)
	}
}

func TestGenerateRecurrenceDatesAnchorsIntervalsToCalendarWeek(t *testing.T) {
	first := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC) // Friday.
	frequencies := TaskFrequencies{mustTaskFrequency(t, "mon")}
	got, err := GenerateRecurrenceDates(first, first, FourWeekIntervalWeeks, frequencies, "UTC", 30)
	if err != nil {
		t.Fatalf("GenerateRecurrenceDates() error = %v", err)
	}
	want := []time.Time{
		first,
		time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
	}
	var dates []time.Time
	for _, occurrence := range got {
		dates = append(dates, occurrence.Date)
	}
	if !reflect.DeepEqual(dates, want) {
		t.Fatalf("occurrence dates = %v, want %v", dates, want)
	}
}

func TestGenerateRecurrenceDatesValidatesInput(t *testing.T) {
	first := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	if _, err := GenerateRecurrenceDates(first, first, -1, nil, "UTC", 30); err != ErrRecurrenceIntervalWeeksLess {
		t.Errorf("negative interval error = %v, want %v", err, ErrRecurrenceIntervalWeeksLess)
	}
	if _, err := GenerateRecurrenceDates(first, first, 1, nil, "invalid/zone", 30); err != ErrRecurrenceTimezoneInvalid {
		t.Errorf("invalid timezone error = %v, want %v", err, ErrRecurrenceTimezoneInvalid)
	}
	if _, err := GenerateRecurrenceDates(first, first, 1, nil, "UTC", -1); err != ErrRecurrenceHorizonInvalid {
		t.Errorf("negative horizon error = %v, want %v", err, ErrRecurrenceHorizonInvalid)
	}
}

func TestGenerateRecurrenceDatesLimitFillsBeyondThirtyDays(t *testing.T) {
	first := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	got, err := GenerateRecurrenceDatesLimit(first, first, WeeklyIntervalWeeks, TaskFrequencies{mustTaskFrequency(t, "mon")}, "UTC", 55)
	if err != nil {
		t.Fatalf("GenerateRecurrenceDatesLimit() error = %v", err)
	}
	if len(got) != 55 {
		t.Fatalf("occurrences = %d, want 55", len(got))
	}
	if !got[0].Root || got[54].Date.Format("2006-01-02") != "2027-10-18" {
		t.Errorf("first/last = %s/%s (root %t), want root 2026-10-05 through 2027-10-18", got[0].Date.Format("2006-01-02"), got[54].Date.Format("2006-01-02"), got[0].Root)
	}
}

func TestGenerateRecurrenceDatesLimitHonorsFromDateAndOneOff(t *testing.T) {
	first := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	from := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	got, err := GenerateRecurrenceDatesLimit(first, from, WeeklyIntervalWeeks, TaskFrequencies{mustTaskFrequency(t, "mon"), mustTaskFrequency(t, "wed")}, "UTC", 2)
	if err != nil {
		t.Fatalf("GenerateRecurrenceDatesLimit() error = %v", err)
	}
	if len(got) != 2 || got[0].Date.Format("2006-01-02") != "2026-10-14" || got[1].Date.Format("2006-01-02") != "2026-10-19" {
		t.Fatalf("dates = %#v, want October 14 and 19", got)
	}
	if got[0].Root {
		t.Error("later occurrence marked as root")
	}
	got, err = GenerateRecurrenceDatesLimit(first, from, OnceIntervalWeeks, nil, "UTC", 5)
	if err != nil {
		t.Fatalf("one-off GenerateRecurrenceDatesLimit() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("one-off before from_date returned %d occurrences", len(got))
	}
}

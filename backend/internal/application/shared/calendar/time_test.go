package calendar

import (
	"testing"
	"time"
)

func TestCalendarDateAndOffsetUseCalendarFields(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	instant := time.Date(2026, 10, 5, 23, 30, 0, 0, time.UTC)
	if got, want := CalendarDate(instant, location), time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("CalendarDate() = %s, want %s", got, want)
	}
	start := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	if got := CalendarDayOffset(start, end); got != 1 {
		t.Fatalf("CalendarDayOffset() = %d, want 1", got)
	}
}

func TestResolveWallTimeRejectsDSTGap(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	wallTime := time.Date(2000, 1, 1, 2, 30, 0, 0, time.UTC)
	if _, ok := ResolveWallTime(date, wallTime, 0, location); ok {
		t.Fatal("ResolveWallTime() accepted nonexistent local time 02:30")
	}
}

func TestNormalizeCalendarDatePreservesZeroAndDate(t *testing.T) {
	if got := NormalizeCalendarDate(time.Time{}); !got.IsZero() {
		t.Fatalf("NormalizeCalendarDate(zero) = %v, want zero", got)
	}
	value := time.Date(2026, 11, 3, 15, 40, 0, 0, time.FixedZone("offset", 9*60*60))
	want := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	if got := NormalizeCalendarDate(value); !got.Equal(want) {
		t.Fatalf("NormalizeCalendarDate() = %s, want %s", got, want)
	}
}

func TestAddCalendarMonthClamped(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want time.Time
	}{
		{
			name: "preserves day",
			date: time.Date(2026, time.March, 10, 14, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "clamps to non leap February",
			date: time.Date(2026, time.January, 31, 14, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "clamps to leap February",
			date: time.Date(2024, time.January, 31, 14, 0, 0, 0, time.UTC),
			want: time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "clamps to 30 day month",
			date: time.Date(2026, time.March, 31, 14, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := AddCalendarMonthClamped(test.date); !got.Equal(test.want) {
				t.Fatalf("AddCalendarMonthClamped(%s) = %s, want %s", test.date, got, test.want)
			}
		})
	}
	if got := AddCalendarMonthClamped(time.Time{}); !got.IsZero() {
		t.Fatalf("AddCalendarMonthClamped(zero) = %s, want zero", got)
	}
}

package domain

import (
	"testing"
	"time"
)

func TestActiveWindowUsesWakingDateAndMovesDSTGapForward(t *testing.T) {
	t.Parallel()
	date := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	wake, _ := time.Parse("15:04", "02:30")
	activeEndTime, _ := time.Parse("15:04", "04:00")
	interval, err := ActiveWindow(date, wake, activeEndTime, "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	if got := interval.Start.In(mustLocation(t, "America/New_York")).Format("2006-01-02 15:04"); got != "2026-03-08 03:00" {
		t.Fatalf("wake gap resolved to %s", got)
	}
	if got := interval.End.In(mustLocation(t, "America/New_York")).Format("2006-01-02 15:04"); got != "2026-03-08 04:00" {
		t.Fatalf("activeEndTime resolved to %s", got)
	}
}

func TestActiveWindowActiveEndTimeCanBeOnFollowingDate(t *testing.T) {
	t.Parallel()
	date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	wake, _ := time.Parse("15:04", "09:00")
	activeEndTime, _ := time.Parse("15:04", "01:00")
	interval, err := ActiveWindow(date, wake, activeEndTime, "Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	location := mustLocation(t, "Asia/Tokyo")
	if got := interval.End.In(location).Format("2006-01-02 15:04"); got != "2026-10-13 01:00" {
		t.Fatalf("activeEndTime resolved to %s", got)
	}
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return location
}

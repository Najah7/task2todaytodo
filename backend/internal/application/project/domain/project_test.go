package domain

import (
	"testing"
	"time"
)

func TestNewProjectScheduleNormalizesCalendarDatesAndPreservesAbsentDates(t *testing.T) {
	zero := time.Time{}
	input := time.Date(2026, time.November, 3, 15, 40, 0, 0, time.FixedZone("offset", 9*60*60))
	want := time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)

	schedule, err := NewProjectSchedule(&zero, &input)
	if err != nil {
		t.Fatalf("NewProjectSchedule() error = %v", err)
	}
	if schedule.StartDate == nil || !schedule.StartDate.IsZero() {
		t.Errorf("StartDate = %v, want preserved zero date", schedule.StartDate)
	}
	if schedule.EndDate == nil || !schedule.EndDate.Equal(want) || schedule.EndDate.Location() != time.UTC {
		t.Errorf("EndDate = %v, want UTC midnight %v", schedule.EndDate, want)
	}

	empty, err := NewProjectSchedule(nil, nil)
	if err != nil {
		t.Fatalf("NewProjectSchedule(nil, nil) error = %v", err)
	}
	if empty.StartDate != nil || empty.EndDate != nil {
		t.Errorf("empty Schedule dates = (%v, %v), want nil dates", empty.StartDate, empty.EndDate)
	}
}

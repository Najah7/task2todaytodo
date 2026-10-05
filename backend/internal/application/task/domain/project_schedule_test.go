package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewProjectSchedule(t *testing.T) {
	start := time.Date(2026, 7, 18, 16, 30, 0, 0, time.FixedZone("JST", 9*60*60))
	sameDayEarlier := time.Date(2026, 7, 18, 1, 0, 0, 0, time.UTC)
	previousDay := time.Date(2026, 7, 17, 23, 59, 0, 0, time.UTC)

	tests := []struct {
		name      string
		startDate *time.Time
		endDate   *time.Time
		wantStart *time.Time
		wantEnd   *time.Time
		wantErr   error
	}{
		{name: "both dates absent"},
		{name: "start date only", startDate: &start, wantStart: datePtr(2026, time.July, 18)},
		{name: "end date only", endDate: &start, wantEnd: datePtr(2026, time.July, 18)},
		{name: "same day allowed", startDate: &start, endDate: &sameDayEarlier, wantStart: datePtr(2026, time.July, 18), wantEnd: datePtr(2026, time.July, 18)},
		{name: "end before start", startDate: &start, endDate: &previousDay, wantErr: ErrProjectEndDateBeforeStartDate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProjectSchedule(tt.startDate, tt.endDate)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewProjectSchedule() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got != (ProjectSchedule{}) {
					t.Errorf("schedule = %+v, want zero value", got)
				}
				return
			}
			assertDatePointer(t, got.StartDate, tt.wantStart)
			assertDatePointer(t, got.EndDate, tt.wantEnd)
		})
	}
}

func datePtr(year int, month time.Month, day int) *time.Time {
	date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return &date
}

func assertDatePointer(t *testing.T, got, want *time.Time) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("date = %v, want %v", got, want)
	}
	if got != nil && !got.Equal(*want) {
		t.Errorf("date = %v, want %v", got, want)
	}
}

package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

func TestNewScheduleKeepsOwnershipAndDefaultsToOneOff(t *testing.T) {
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	got, err := NewSchedule("schedule-1", "owner-1", "project-1", "owner-1", "Focus", start, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewSchedule() error = %v", err)
	}
	if got.UserID != "owner-1" || got.ProjectID != "project-1" || got.AssigneeID != "owner-1" || !got.IsOnce() {
		t.Fatalf("schedule = %+v, want owner, project, assignee and one-off interval", got)
	}
}

func TestScheduleValidation(t *testing.T) {
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		id        ScheduleID
		userID    UserID
		assignee  UserID
		title     string
		startAt   time.Time
		endAt     time.Time
		wantError error
	}{
		{name: "empty ID", userID: "owner", assignee: "owner", title: "Focus", startAt: start, endAt: start.Add(time.Hour), wantError: ErrScheduleIDEmpty},
		{name: "empty owner", id: "schedule", assignee: "owner", title: "Focus", startAt: start, endAt: start.Add(time.Hour), wantError: ErrScheduleUserIDEmpty},
		{name: "empty assignee", id: "schedule", userID: "owner", title: "Focus", startAt: start, endAt: start.Add(time.Hour), wantError: ErrScheduleAssigneeIDEmpty},
		{name: "blank title", id: "schedule", userID: "owner", assignee: "owner", title: " ", startAt: start, endAt: start.Add(time.Hour), wantError: ErrScheduleTitleEmpty},
		{name: "missing start", id: "schedule", userID: "owner", assignee: "owner", title: "Focus", endAt: start.Add(time.Hour), wantError: ErrScheduleStartAtEmpty},
		{name: "missing end", id: "schedule", userID: "owner", assignee: "owner", title: "Focus", startAt: start, wantError: ErrScheduleEndAtEmpty},
		{name: "end not after start", id: "schedule", userID: "owner", assignee: "owner", title: "Focus", startAt: start, endAt: start, wantError: ErrScheduleEndAtMustBeAfterStartAt},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewSchedule(test.id, test.userID, "", test.assignee, test.title, test.startAt, test.endAt)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("NewSchedule() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func TestNewScheduleWithRecurrenceRequiresCalendarDateAndValidTimezone(t *testing.T) {
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	schedule, err := NewSchedule("schedule-1", "owner-1", "", "owner-1", "Focus", start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		metadata RecurrenceMetadata
		wantErr  error
	}{
		{name: "missing date", metadata: RecurrenceMetadata{SeriesID: "schedule-1", Timezone: "UTC"}, wantErr: recurrence.ErrRecurrenceMetadataInvalid},
		{name: "invalid timezone", metadata: RecurrenceMetadata{SeriesID: "schedule-1", OccurrenceDate: start, Timezone: "invalid/zone"}, wantErr: recurrence.ErrRecurrenceTimezoneInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := schedule.WithRecurrence(test.metadata)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("WithRecurrence() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

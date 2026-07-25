package task

import (
	"testing"
	"time"
)

func TestNewTaskSchedule(t *testing.T) {
	startAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)

	schedule, err := NewTaskSchedule("schedule-1", "task-1", "Focus block", startAt, endAt)
	if err != nil {
		t.Fatalf("NewTaskSchedule() error = %v", err)
	}

	if schedule.ID != "schedule-1" || schedule.TaskID != "task-1" || schedule.Title != "Focus block" {
		t.Errorf("task schedule = %+v, want required values to be set", schedule)
	}
	if schedule.StartAt != startAt || schedule.EndAt != endAt || schedule.Description != "" {
		t.Errorf("task schedule = %+v, want only required values to be set", schedule)
	}
}

func TestNewTaskScheduleWithDetails(t *testing.T) {
	startAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)
	dueAt := startAt.Add(2 * time.Hour)

	schedule, err := NewTaskScheduleWithDetails("schedule-1", "task-1", "Focus block", "Deep work", "Home", startAt, endAt, dueAt)
	if err != nil {
		t.Fatalf("NewTaskScheduleWithDetails() error = %v", err)
	}

	if schedule.ID != "schedule-1" || schedule.TaskID != "task-1" || schedule.DueAt != dueAt {
		t.Errorf("task schedule = %+v, want IDs and due time to be set", schedule)
	}
}

func TestNewTaskScheduleValidation(t *testing.T) {
	startAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)

	tests := []struct {
		name    string
		id      TaskScheduleID
		taskID  TaskID
		title   string
		startAt time.Time
		endAt   time.Time
		wantErr error
	}{
		{name: "empty ID", taskID: "task-1", title: "Schedule", startAt: startAt, endAt: endAt, wantErr: ErrTaskScheduleIDEmpty},
		{name: "empty task ID", id: "schedule-1", title: "Schedule", startAt: startAt, endAt: endAt, wantErr: ErrTaskScheduleTaskIDEmpty},
		{name: "blank title", id: "schedule-1", taskID: "task-1", title: " ", startAt: startAt, endAt: endAt, wantErr: ErrTaskScheduleTitleEmpty},
		{name: "empty start time", id: "schedule-1", taskID: "task-1", title: "Schedule", endAt: endAt, wantErr: ErrTaskScheduleStartAtEmpty},
		{name: "empty end time", id: "schedule-1", taskID: "task-1", title: "Schedule", startAt: startAt, wantErr: ErrTaskScheduleEndAtEmpty},
		{name: "end before start", id: "schedule-1", taskID: "task-1", title: "Schedule", startAt: startAt, endAt: startAt, wantErr: ErrTaskScheduleEndAtMustBeAfterStartAt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTaskSchedule(tt.id, tt.taskID, tt.title, tt.startAt, tt.endAt)
			assertTaskDomainErrorIs(t, err, tt.wantErr)
			if got.ID != tt.id || got.TaskID != tt.taskID || got.Title != tt.title {
				t.Errorf("task schedule = %+v, want input values to be preserved", got)
			}
		})
	}
}

func TestNewTaskScheduleWithDetailsValidation(t *testing.T) {
	startAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		startAt time.Time
		endAt   time.Time
		wantErr error
	}{
		{name: "empty start time", endAt: startAt.Add(time.Hour), wantErr: ErrTaskScheduleStartAtEmpty},
		{name: "empty end time", startAt: startAt, wantErr: ErrTaskScheduleEndAtEmpty},
		{name: "end before start", startAt: startAt, endAt: startAt, wantErr: ErrTaskScheduleEndAtMustBeAfterStartAt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTaskScheduleWithDetails("schedule-1", "task-1", "Schedule", "", "", tt.startAt, tt.endAt, time.Time{})
			assertTaskDomainErrorIs(t, err, tt.wantErr)
			if got.StartAt != tt.startAt || got.EndAt != tt.endAt {
				t.Errorf("task schedule = %+v, want input details to be preserved", got)
			}
		})
	}
}

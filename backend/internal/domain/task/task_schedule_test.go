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
	if schedule.StartAt != startAt || schedule.EndAt != endAt || schedule.Description != "" || schedule.IntervalWeeks != 1 {
		t.Errorf("task schedule = %+v, want only required values to be set", schedule)
	}
}

func TestNewTaskScheduleWithDetails(t *testing.T) {
	startAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)
	frequencies := TaskFrequencies{mustTaskFrequencyForService(t, "mon")}

	schedule, err := NewTaskScheduleWithDetails("schedule-1", "task-1", "Focus block", "Deep work", "Home", 2, frequencies, startAt, endAt)
	if err != nil {
		t.Fatalf("NewTaskScheduleWithDetails() error = %v", err)
	}

	if schedule.ID != "schedule-1" || schedule.TaskID != "task-1" || schedule.IntervalWeeks != 2 || !schedule.Frequencies.IsWeekday() {
		t.Errorf("task schedule = %+v, want IDs, interval, and frequencies to be set", schedule)
	}
}

func TestNewTaskScheduleValidation(t *testing.T) {
	startAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)

	tests := []struct {
		name          string
		id            TaskScheduleID
		taskID        TaskID
		title         string
		intervalWeeks int
		startAt       time.Time
		endAt         time.Time
		wantErr       error
	}{
		{name: "empty ID", taskID: "task-1", title: "Schedule", intervalWeeks: 1, startAt: startAt, endAt: endAt, wantErr: ErrTaskScheduleIDEmpty},
		{name: "empty task ID", id: "schedule-1", title: "Schedule", intervalWeeks: 1, startAt: startAt, endAt: endAt, wantErr: ErrTaskScheduleTaskIDEmpty},
		{name: "blank title", id: "schedule-1", taskID: "task-1", title: " ", intervalWeeks: 1, startAt: startAt, endAt: endAt, wantErr: ErrTaskScheduleTitleEmpty},
		{name: "interval weeks less than 0", id: "schedule-1", taskID: "task-1", title: "Schedule", intervalWeeks: -1, startAt: startAt, endAt: endAt, wantErr: ErrTaskScheduleIntervalWeeksLess},
		{name: "empty start time", id: "schedule-1", taskID: "task-1", title: "Schedule", intervalWeeks: 1, endAt: endAt, wantErr: ErrTaskScheduleStartAtEmpty},
		{name: "empty end time", id: "schedule-1", taskID: "task-1", title: "Schedule", intervalWeeks: 1, startAt: startAt, wantErr: ErrTaskScheduleEndAtEmpty},
		{name: "end before start", id: "schedule-1", taskID: "task-1", title: "Schedule", intervalWeeks: 1, startAt: startAt, endAt: startAt, wantErr: ErrTaskScheduleEndAtMustBeAfterStartAt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTaskScheduleWithDetails(tt.id, tt.taskID, tt.title, "", "", tt.intervalWeeks, nil, tt.startAt, tt.endAt)
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
			got, err := NewTaskScheduleWithDetails("schedule-1", "task-1", "Schedule", "", "", 1, nil, tt.startAt, tt.endAt)
			assertTaskDomainErrorIs(t, err, tt.wantErr)
			if got.StartAt != tt.startAt || got.EndAt != tt.endAt {
				t.Errorf("task schedule = %+v, want input details to be preserved", got)
			}
		})
	}
}

func TestTaskScheduleRepeatPattern(t *testing.T) {
	startAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)
	weekday := TaskFrequencies{mustTaskFrequencyForService(t, "mon")}

	once, err := NewTaskScheduleWithDetails("schedule-1", "task-1", "Focus block", "", "", 0, nil, startAt, endAt)
	if err != nil {
		t.Fatalf("NewTaskScheduleWithDetails() once error = %v", err)
	}
	if !once.IsOnce() || once.IsWeekly() {
		t.Errorf("task schedule = %+v, want interval 0 to mean once", once)
	}

	weekly, err := NewTaskScheduleWithDetails("schedule-1", "task-1", "Focus block", "", "", 1, weekday, startAt, endAt)
	if err != nil {
		t.Fatalf("NewTaskScheduleWithDetails() weekly error = %v", err)
	}
	if weekly.IsOnce() || !weekly.IsWeekly() || !weekly.IsEveryWeekday() {
		t.Errorf("task schedule = %+v, want interval 1 with weekday frequency to mean weekly weekday", weekly)
	}
}

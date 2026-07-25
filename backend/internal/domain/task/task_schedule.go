package task

import (
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2schedule/internal/domain/shared"
)

var (
	ErrTaskScheduleIDEmpty                 = errors.New("task schedule ID cannot be empty")
	ErrTaskScheduleTaskIDEmpty             = errors.New("task schedule task ID cannot be empty")
	ErrTaskScheduleTitleEmpty              = errors.New("task schedule title cannot be empty")
	ErrTaskScheduleStartAtEmpty            = errors.New("task schedule start time must be set")
	ErrTaskScheduleEndAtEmpty              = errors.New("task schedule end time must be set")
	ErrTaskScheduleEndAtMustBeAfterStartAt = errors.New("task schedule end time must be after start time")
)

type TaskScheduleID shared.ID

type TaskSchedule struct {
	ID          TaskScheduleID
	TaskID      TaskID
	Title       string
	Description string
	Location    string
	StartAt     time.Time
	EndAt       time.Time
	DueAt       time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewTaskSchedule(
	id TaskScheduleID,
	taskID TaskID,
	title string,
	startAt time.Time,
	endAt time.Time,
) (TaskSchedule, error) {
	schedule := TaskSchedule{
		ID:      id,
		TaskID:  taskID,
		Title:   title,
		StartAt: startAt,
		EndAt:   endAt,
	}
	return schedule, schedule.Validate()
}

func NewTaskScheduleWithDetails(
	id TaskScheduleID,
	taskID TaskID,
	title string,
	description string,
	location string,
	startAt time.Time,
	endAt time.Time,
	dueAt time.Time,
) (TaskSchedule, error) {
	schedule := TaskSchedule{
		ID:          id,
		TaskID:      taskID,
		Title:       title,
		Description: description,
		Location:    location,
		StartAt:     startAt,
		EndAt:       endAt,
		DueAt:       dueAt,
	}
	return schedule, schedule.Validate()
}

func NewExistingTaskSchedule(
	id TaskScheduleID,
	taskID TaskID,
	title string,
	description string,
	location string,
	startAt time.Time,
	endAt time.Time,
	dueAt time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) (TaskSchedule, error) {
	schedule := TaskSchedule{
		ID:          id,
		TaskID:      taskID,
		Title:       title,
		Description: description,
		Location:    location,
		StartAt:     startAt,
		EndAt:       endAt,
		DueAt:       dueAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
	if err := schedule.Validate(); err != nil {
		return NewZeroTaskSchedule(), err
	}

	return schedule, nil
}

func NewZeroTaskSchedule() TaskSchedule {
	return TaskSchedule{}
}

func (s TaskSchedule) IsZero() bool {
	return s.ID == ""
}

func (s TaskSchedule) Validate() error {
	if s.ID == "" {
		return ErrTaskScheduleIDEmpty
	}
	if s.TaskID == "" {
		return ErrTaskScheduleTaskIDEmpty
	}
	if strings.TrimSpace(s.Title) == "" {
		return ErrTaskScheduleTitleEmpty
	}
	if s.StartAt.IsZero() {
		return ErrTaskScheduleStartAtEmpty
	}
	if s.EndAt.IsZero() {
		return ErrTaskScheduleEndAtEmpty
	}
	if !s.EndAt.After(s.StartAt) {
		return ErrTaskScheduleEndAtMustBeAfterStartAt
	}

	return nil
}

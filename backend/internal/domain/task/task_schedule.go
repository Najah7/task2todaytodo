package task

import (
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
)

var (
	ErrTaskScheduleIDEmpty                 = errors.New("task schedule ID cannot be empty")
	ErrTaskScheduleTaskIDEmpty             = errors.New("task schedule task ID cannot be empty")
	ErrTaskScheduleTitleEmpty              = errors.New("task schedule title cannot be empty")
	ErrTaskScheduleIntervalWeeksLess       = errors.New("task schedule interval weeks must be greater than or equal to 0")
	ErrTaskScheduleStartAtEmpty            = errors.New("task schedule start time must be set")
	ErrTaskScheduleEndAtEmpty              = errors.New("task schedule end time must be set")
	ErrTaskScheduleEndAtMustBeAfterStartAt = errors.New("task schedule end time must be after start time")
	ErrTaskScheduleTaskNotFound            = errors.New("task not found")
	ErrTaskScheduleNotFound                = errors.New("task schedule not found")
)

type TaskScheduleID shared.ID

type TaskSchedule struct {
	ID            TaskScheduleID
	TaskID        TaskID
	Title         string
	Description   string
	Location      string
	IntervalWeeks int
	Frequencies   TaskFrequencies
	StartAt       time.Time
	EndAt         time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewTaskSchedule(
	id TaskScheduleID,
	taskID TaskID,
	title string,
	startAt time.Time,
	endAt time.Time,
) (TaskSchedule, error) {
	schedule := TaskSchedule{
		ID:            id,
		TaskID:        taskID,
		Title:         title,
		IntervalWeeks: 1,
		StartAt:       startAt,
		EndAt:         endAt,
	}
	return schedule, schedule.Validate()
}

func NewTaskScheduleWithDetails(
	id TaskScheduleID,
	taskID TaskID,
	title string,
	description string,
	location string,
	intervalWeeks int,
	frequencies TaskFrequencies,
	startAt time.Time,
	endAt time.Time,
) (TaskSchedule, error) {
	schedule := TaskSchedule{
		ID:            id,
		TaskID:        taskID,
		Title:         title,
		Description:   description,
		Location:      location,
		IntervalWeeks: intervalWeeks,
		Frequencies:   frequencies,
		StartAt:       startAt,
		EndAt:         endAt,
	}
	return schedule, schedule.Validate()
}

func NewExistingTaskSchedule(
	id TaskScheduleID,
	taskID TaskID,
	title string,
	description string,
	location string,
	intervalWeeks int,
	frequencies TaskFrequencies,
	startAt time.Time,
	endAt time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) (TaskSchedule, error) {
	schedule := TaskSchedule{
		ID:            id,
		TaskID:        taskID,
		Title:         title,
		Description:   description,
		Location:      location,
		IntervalWeeks: intervalWeeks,
		Frequencies:   frequencies,
		StartAt:       startAt,
		EndAt:         endAt,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
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
	if s.IntervalWeeks < 0 {
		return ErrTaskScheduleIntervalWeeksLess
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

func (s TaskSchedule) IsWeekly() bool {
	return s.IntervalWeeks == WeeklyIntervalWeeks
}

func (s TaskSchedule) IsEveryWeekday() bool {
	return s.Frequencies.IsWeekday() && s.IsWeekly()
}

func (s TaskSchedule) IsEveryWeekend() bool {
	return s.Frequencies.IsWeekend() && s.IsWeekly()
}

func (s TaskSchedule) IsBiWeekly() bool {
	return s.IntervalWeeks == BiWeeklyIntervalWeeks
}

func (s TaskSchedule) IsMonthly() bool {
	return s.IntervalWeeks == MonthlyIntervalWeeks
}

func (s TaskSchedule) IsQuarterly() bool {
	return s.IntervalWeeks == QuarterlyIntervalWeeks
}

func (s TaskSchedule) IsSemiAnnually() bool {
	return s.IntervalWeeks == SemiAnnualIntervalWeeks
}

func (s TaskSchedule) IsAnnually() bool {
	return s.IntervalWeeks == AnnualIntervalWeeks
}

func (s TaskSchedule) IsOnce() bool {
	return s.IntervalWeeks == OnceIntervalWeeks
}

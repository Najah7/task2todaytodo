package domain

import (
	"errors"
	"strings"
	"time"

	tasktime "github.com/Najah7/task2todaytodo/internal/application/task"
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

type TaskScheduleID string

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
	Completed     bool
	// SeriesID points to the original row. The original row is its own series root.
	SeriesID       TaskScheduleID
	OccurrenceDate time.Time
	Timezone       string
	IsException    bool
	Deleted        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
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
		IntervalWeeks: OnceIntervalWeeks,
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
	completed bool,
	createdAt time.Time,
	updatedAt time.Time,
	recurrence ...RecurrenceMetadata,
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
		Completed:     completed,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	if len(recurrence) > 0 {
		schedule.SeriesID = TaskScheduleID(recurrence[0].SeriesID)
		schedule.OccurrenceDate = recurrence[0].OccurrenceDate
		schedule.Timezone = recurrence[0].Timezone
		schedule.IsException = recurrence[0].IsException
		schedule.Deleted = recurrence[0].Deleted
	} else {
		schedule.SeriesID = id
		schedule.OccurrenceDate = startAt
		schedule.Timezone = "UTC"
	}
	if err := schedule.Validate(); err != nil {
		return NewZeroTaskSchedule(), err
	}
	if len(recurrence) > 0 {
		return NewTaskScheduleWithRecurrence(schedule)
	}

	return schedule, nil
}

func NewZeroTaskSchedule() TaskSchedule {
	return TaskSchedule{}
}

// NewTaskScheduleWithRecurrence restores a row with recurrence lineage metadata.
func NewTaskScheduleWithRecurrence(schedule TaskSchedule) (TaskSchedule, error) {
	if schedule.SeriesID == "" {
		schedule.SeriesID = schedule.ID
	}
	schedule.OccurrenceDate = tasktime.NormalizeCalendarDate(schedule.OccurrenceDate)
	if err := schedule.Validate(); err != nil {
		return NewZeroTaskSchedule(), err
	}
	if schedule.OccurrenceDate.IsZero() || strings.TrimSpace(schedule.Timezone) == "" {
		return NewZeroTaskSchedule(), ErrRecurrenceMetadataInvalid
	}
	if _, err := time.LoadLocation(schedule.Timezone); err != nil {
		return NewZeroTaskSchedule(), ErrRecurrenceTimezoneInvalid
	}
	return schedule, nil
}

func (s TaskSchedule) WithRecurrence(metadata RecurrenceMetadata) (TaskSchedule, error) {
	s.SeriesID = TaskScheduleID(metadata.SeriesID)
	s.OccurrenceDate = metadata.OccurrenceDate
	s.Timezone = metadata.Timezone
	s.IsException = metadata.IsException
	s.Deleted = metadata.Deleted
	return NewTaskScheduleWithRecurrence(s)
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

func (s TaskSchedule) IsEveryFourWeeks() bool {
	return s.IntervalWeeks == FourWeekIntervalWeeks
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

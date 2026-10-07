package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

var (
	ErrScheduleIDEmpty                 = errors.New("schedule ID cannot be empty")
	ErrScheduleUserIDEmpty             = errors.New("schedule user ID cannot be empty")
	ErrScheduleAssigneeIDEmpty         = errors.New("schedule assignee ID cannot be empty")
	ErrScheduleTitleEmpty              = errors.New("schedule title cannot be empty")
	ErrScheduleIntervalWeeksLess       = errors.New("schedule interval weeks must be greater than or equal to 0")
	ErrScheduleStartAtEmpty            = errors.New("schedule start time must be set")
	ErrScheduleEndAtEmpty              = errors.New("schedule end time must be set")
	ErrScheduleEndAtMustBeAfterStartAt = errors.New("schedule end time must be after start time")
	ErrScheduleNotFound                = errors.New("schedule not found")
)

type ScheduleID string
type UserID string
type ProjectID string
type Frequency = recurrence.Frequency
type Frequencies = recurrence.Frequencies
type RecurrenceMetadata = recurrence.Metadata

const (
	OnceIntervalWeeks       = recurrence.OnceIntervalWeeks
	WeeklyIntervalWeeks     = recurrence.WeeklyIntervalWeeks
	BiWeeklyIntervalWeeks   = recurrence.BiWeeklyIntervalWeeks
	FourWeekIntervalWeeks   = recurrence.FourWeekIntervalWeeks
	QuarterlyIntervalWeeks  = recurrence.QuarterlyIntervalWeeks
	SemiAnnualIntervalWeeks = recurrence.SemiAnnualIntervalWeeks
	AnnualIntervalWeeks     = recurrence.AnnualIntervalWeeks
)

type Schedule struct {
	ID             ScheduleID
	UserID         UserID
	ProjectID      ProjectID
	AssigneeID     UserID
	Title          string
	Description    string
	Location       string
	IntervalWeeks  int
	Frequencies    Frequencies
	StartAt        time.Time
	EndAt          time.Time
	Completed      bool
	SeriesID       ScheduleID
	OccurrenceDate time.Time
	Timezone       string
	IsException    bool
	Deleted        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewSchedule(id ScheduleID, userID UserID, projectID ProjectID, assigneeID UserID, title string, startAt, endAt time.Time) (Schedule, error) {
	schedule := Schedule{
		ID: id, UserID: userID, ProjectID: projectID, AssigneeID: assigneeID,
		Title: title, IntervalWeeks: OnceIntervalWeeks, StartAt: startAt, EndAt: endAt,
	}
	return schedule, schedule.Validate()
}

func NewScheduleWithDetails(id ScheduleID, userID UserID, projectID ProjectID, assigneeID UserID, title, description, location string, intervalWeeks int, frequencies Frequencies, startAt, endAt time.Time) (Schedule, error) {
	schedule := Schedule{
		ID: id, UserID: userID, ProjectID: projectID, AssigneeID: assigneeID,
		Title: title, Description: description, Location: location,
		IntervalWeeks: intervalWeeks, Frequencies: frequencies, StartAt: startAt, EndAt: endAt,
	}
	return schedule, schedule.Validate()
}

func NewExistingSchedule(id ScheduleID, userID UserID, projectID ProjectID, assigneeID UserID, title, description, location string, intervalWeeks int, frequencies Frequencies, startAt, endAt time.Time, completed bool, createdAt, updatedAt time.Time, metadata ...RecurrenceMetadata) (Schedule, error) {
	schedule := Schedule{
		ID: id, UserID: userID, ProjectID: projectID, AssigneeID: assigneeID,
		Title: title, Description: description, Location: location,
		IntervalWeeks: intervalWeeks, Frequencies: frequencies, StartAt: startAt, EndAt: endAt,
		Completed: completed, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	if len(metadata) > 0 {
		schedule.SeriesID = ScheduleID(metadata[0].SeriesID)
		schedule.OccurrenceDate = metadata[0].OccurrenceDate
		schedule.Timezone = metadata[0].Timezone
		schedule.IsException = metadata[0].IsException
		schedule.Deleted = metadata[0].Deleted
	} else {
		schedule.SeriesID = id
		schedule.OccurrenceDate = startAt
		schedule.Timezone = "UTC"
	}
	if err := schedule.Validate(); err != nil {
		return NewZeroSchedule(), err
	}
	if len(metadata) > 0 {
		return NewScheduleWithRecurrence(schedule)
	}
	return schedule, nil
}

func NewZeroSchedule() Schedule { return Schedule{} }

func NewScheduleWithRecurrence(schedule Schedule) (Schedule, error) {
	if schedule.SeriesID == "" {
		schedule.SeriesID = schedule.ID
	}
	schedule.OccurrenceDate = calendar.NormalizeCalendarDate(schedule.OccurrenceDate)
	if err := schedule.Validate(); err != nil {
		return NewZeroSchedule(), err
	}
	if schedule.OccurrenceDate.IsZero() || strings.TrimSpace(schedule.Timezone) == "" {
		return NewZeroSchedule(), recurrence.ErrRecurrenceMetadataInvalid
	}
	if _, err := time.LoadLocation(schedule.Timezone); err != nil {
		return NewZeroSchedule(), recurrence.ErrRecurrenceTimezoneInvalid
	}
	return schedule, nil
}

func (s Schedule) WithRecurrence(metadata RecurrenceMetadata) (Schedule, error) {
	s.SeriesID = ScheduleID(metadata.SeriesID)
	s.OccurrenceDate = metadata.OccurrenceDate
	s.Timezone = metadata.Timezone
	s.IsException = metadata.IsException
	s.Deleted = metadata.Deleted
	return NewScheduleWithRecurrence(s)
}

func (s Schedule) WithProject(projectID ProjectID) Schedule {
	s.ProjectID = projectID
	return s
}

func (s Schedule) WithAssignee(assigneeID UserID) (Schedule, error) {
	s.AssigneeID = assigneeID
	if err := s.Validate(); err != nil {
		return NewZeroSchedule(), err
	}
	return s, nil
}

func (s Schedule) IsZero() bool { return s.ID == "" }

func (s Schedule) Validate() error {
	if s.ID == "" {
		return ErrScheduleIDEmpty
	}
	if s.UserID == "" {
		return ErrScheduleUserIDEmpty
	}
	if s.AssigneeID == "" {
		return ErrScheduleAssigneeIDEmpty
	}
	if strings.TrimSpace(s.Title) == "" {
		return ErrScheduleTitleEmpty
	}
	if s.IntervalWeeks < 0 {
		return ErrScheduleIntervalWeeksLess
	}
	if s.StartAt.IsZero() {
		return ErrScheduleStartAtEmpty
	}
	if s.EndAt.IsZero() {
		return ErrScheduleEndAtEmpty
	}
	if !s.EndAt.After(s.StartAt) {
		return ErrScheduleEndAtMustBeAfterStartAt
	}
	return nil
}

func (s Schedule) IsWeekly() bool         { return s.IntervalWeeks == WeeklyIntervalWeeks }
func (s Schedule) IsEveryWeekday() bool   { return s.Frequencies.IsWeekday() && s.IsWeekly() }
func (s Schedule) IsEveryWeekend() bool   { return s.Frequencies.IsWeekend() && s.IsWeekly() }
func (s Schedule) IsBiWeekly() bool       { return s.IntervalWeeks == BiWeeklyIntervalWeeks }
func (s Schedule) IsEveryFourWeeks() bool { return s.IntervalWeeks == FourWeekIntervalWeeks }
func (s Schedule) IsQuarterly() bool      { return s.IntervalWeeks == QuarterlyIntervalWeeks }
func (s Schedule) IsSemiAnnually() bool   { return s.IntervalWeeks == SemiAnnualIntervalWeeks }
func (s Schedule) IsAnnually() bool       { return s.IntervalWeeks == AnnualIntervalWeeks }
func (s Schedule) IsOnce() bool           { return s.IntervalWeeks == OnceIntervalWeeks }

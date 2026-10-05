package domain

import (
	"errors"
	"time"

	tasktime "github.com/Najah7/task2todaytodo/internal/application/task"
)

var ErrProjectEndDateBeforeStartDate = errors.New("project end date cannot be before start date")

// ProjectSchedule stores calendar dates rather than instants. Dates are held
// as UTC-midnight values so their year/month/day remain stable across zones.
type ProjectSchedule struct {
	StartDate *time.Time
	EndDate   *time.Time
}

func NewProjectSchedule(startDate, endDate *time.Time) (ProjectSchedule, error) {
	schedule := ProjectSchedule{
		StartDate: normalizeProjectDate(startDate),
		EndDate:   normalizeProjectDate(endDate),
	}
	if err := schedule.Validate(); err != nil {
		return ProjectSchedule{}, err
	}
	return schedule, nil
}

func (s ProjectSchedule) Validate() error {
	if s.StartDate != nil && s.EndDate != nil && s.EndDate.Before(*s.StartDate) {
		return ErrProjectEndDateBeforeStartDate
	}
	return nil
}

func normalizeProjectDate(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	date := tasktime.NormalizeCalendarDate(*value)
	return &date
}

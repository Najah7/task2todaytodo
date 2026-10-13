package domain

import (
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

var ErrInvalidActiveTimeWindow = errors.New("invalid active_time window")

// ActiveWindow resolves the daily activeWindow window. ActiveEndTime at or before wake
// time belongs to the following local date. DST gaps move forward to the first
// valid local minute; folds use their earlier instant.
func ActiveWindow(date, wake, activeEndTime time.Time, timezone string) (Interval, error) {
	dayOffset := 0
	if activeEndTime.Hour() < wake.Hour() || (activeEndTime.Hour() == wake.Hour() && !activeEndTime.After(wake)) {
		dayOffset = 1
	}
	return ActiveWindowWithOffset(date, wake, activeEndTime, dayOffset, timezone)
}

// ActiveWindowWithOffset resolves an activeWindow window whose activeEndTime date is
// explicit, as it is for saved daily exceptions.
func ActiveWindowWithOffset(date, wake, activeEndTime time.Time, dayOffset int, timezone string) (Interval, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Interval{}, ErrInvalidActiveTimeWindow
	}
	date = calendar.DateOnly(date)
	wake, activeEndTime = wallClock(wake), wallClock(activeEndTime)
	if dayOffset < 0 || dayOffset > 1 {
		return Interval{}, ErrInvalidActiveTimeWindow
	}
	start, ok := calendar.ResolveForward(date, wake, 0, location)
	if !ok {
		return Interval{}, ErrInvalidActiveTimeWindow
	}
	end, ok := calendar.ResolveForward(date, activeEndTime, dayOffset, location)
	if !ok || !end.After(start) {
		return Interval{}, ErrInvalidActiveTimeWindow
	}
	return Interval{Start: start, End: end}, nil
}

// ActiveWindowFromLocalPair uses the explicit full local timestamps saved on
// a waking-date exception. The wake must be on date and activeEndTime may be on date
// or the following date.
func ActiveWindowFromLocalPair(date, wake, activeEndTime time.Time, timezone string) (Interval, error) {
	date = calendar.DateOnly(date)
	wakeDate := calendar.DateOnly(wake)
	bedDate := calendar.DateOnly(activeEndTime)
	if !wakeDate.Equal(date) || (!bedDate.Equal(date) && !bedDate.Equal(date.AddDate(0, 0, 1))) {
		return Interval{}, ErrInvalidActiveTimeWindow
	}
	offset := 0
	if bedDate.After(date) {
		offset = 1
	}
	return ActiveWindowWithOffset(date, wake, activeEndTime, offset, timezone)
}

func wallClock(value time.Time) time.Time {
	return time.Date(0, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

// ValidateActiveWindowSequence ensures adjacent waking-date windows do not overlap.
// ActiveWindow windows may touch at their endpoints.
func ValidateActiveWindowSequence(dates []time.Time, wakes, activeEndTimes []time.Time, timezones []string) error {
	if len(dates) != len(wakes) || len(dates) != len(activeEndTimes) || len(dates) != len(timezones) {
		return ErrInvalidActiveTimeWindow
	}
	var previous Interval
	for i := range dates {
		current, err := ActiveWindow(dates[i], wakes[i], activeEndTimes[i], timezones[i])
		if err != nil {
			return err
		}
		if i > 0 && current.Start.Before(previous.End) {
			return ErrInvalidActiveTimeWindow
		}
		previous = current
	}
	return nil
}

package domain

import (
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

var ErrInvalidSleepWindow = errors.New("invalid sleep window")

// AwakeInterval resolves the daily awake window. Bedtime at or before wake
// time belongs to the following local date. DST gaps move forward to the first
// valid local minute; folds use their earlier instant.
func AwakeInterval(date, wake, bedtime time.Time, timezone string) (Interval, error) {
	dayOffset := 0
	if bedtime.Hour() < wake.Hour() || (bedtime.Hour() == wake.Hour() && !bedtime.After(wake)) {
		dayOffset = 1
	}
	return AwakeIntervalWithOffset(date, wake, bedtime, dayOffset, timezone)
}

// AwakeIntervalWithOffset resolves an awake window whose bedtime date is
// explicit, as it is for saved daily exceptions.
func AwakeIntervalWithOffset(date, wake, bedtime time.Time, dayOffset int, timezone string) (Interval, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Interval{}, ErrInvalidSleepWindow
	}
	date = calendar.DateOnly(date)
	wake, bedtime = wallClock(wake), wallClock(bedtime)
	if dayOffset < 0 || dayOffset > 1 {
		return Interval{}, ErrInvalidSleepWindow
	}
	start, ok := calendar.ResolveForward(date, wake, 0, location)
	if !ok {
		return Interval{}, ErrInvalidSleepWindow
	}
	end, ok := calendar.ResolveForward(date, bedtime, dayOffset, location)
	if !ok || !end.After(start) {
		return Interval{}, ErrInvalidSleepWindow
	}
	return Interval{Start: start, End: end}, nil
}

// AwakeIntervalFromLocalPair uses the explicit full local timestamps saved on
// a waking-date exception. The wake must be on date and bedtime may be on date
// or the following date.
func AwakeIntervalFromLocalPair(date, wake, bedtime time.Time, timezone string) (Interval, error) {
	date = calendar.DateOnly(date)
	wakeDate := calendar.DateOnly(wake)
	bedDate := calendar.DateOnly(bedtime)
	if !wakeDate.Equal(date) || (!bedDate.Equal(date) && !bedDate.Equal(date.AddDate(0, 0, 1))) {
		return Interval{}, ErrInvalidSleepWindow
	}
	offset := 0
	if bedDate.After(date) {
		offset = 1
	}
	return AwakeIntervalWithOffset(date, wake, bedtime, offset, timezone)
}

func wallClock(value time.Time) time.Time {
	return time.Date(0, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

// ValidateAwakeSequence ensures adjacent waking-date windows do not overlap.
// Awake windows may touch at their endpoints.
func ValidateAwakeSequence(dates []time.Time, wakes, bedtimes []time.Time, timezones []string) error {
	if len(dates) != len(wakes) || len(dates) != len(bedtimes) || len(dates) != len(timezones) {
		return ErrInvalidSleepWindow
	}
	var previous Interval
	for i := range dates {
		current, err := AwakeInterval(dates[i], wakes[i], bedtimes[i], timezones[i])
		if err != nil {
			return err
		}
		if i > 0 && current.Start.Before(previous.End) {
			return ErrInvalidSleepWindow
		}
		previous = current
	}
	return nil
}

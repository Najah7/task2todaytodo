// Package calendar contains calendar-date and wall-time policies shared by
// application contexts.
package calendar

import "time"

// NormalizeCalendarDate treats the calendar components of value as a date and
// returns that date at UTC midnight. A zero value remains zero.
func NormalizeCalendarDate(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

// CalendarDate converts instant into location and returns its local calendar
// date at UTC midnight. location must be non-nil.
func CalendarDate(instant time.Time, location *time.Location) time.Time {
	local := instant.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

// CalendarDayOffset returns the number of calendar days from start to end,
// using their year, month, and day components rather than elapsed hours.
func CalendarDayOffset(start, end time.Time) int {
	startDate := NormalizeCalendarDate(start)
	endDate := NormalizeCalendarDate(end)
	return int(endDate.Sub(startDate).Hours() / 24)
}

// ResolveWallTime places wallTime's clock components on date plus dayOffset in
// location. It reports false when the requested local time does not exist,
// such as during a daylight-saving clock jump. location must be non-nil.
func ResolveWallTime(date, wallTime time.Time, dayOffset int, location *time.Location) (time.Time, bool) {
	if location == nil {
		return time.Time{}, false
	}
	localDate := date.AddDate(0, 0, dayOffset)
	resolved := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), wallTime.Hour(), wallTime.Minute(), wallTime.Second(), wallTime.Nanosecond(), location)
	local := resolved.In(location)
	valid := local.Year() == localDate.Year() && local.Month() == localDate.Month() && local.Day() == localDate.Day() &&
		local.Hour() == wallTime.Hour() && local.Minute() == wallTime.Minute() && local.Second() == wallTime.Second() && local.Nanosecond() == wallTime.Nanosecond()
	return resolved, valid
}

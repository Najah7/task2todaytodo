package domain

import (
	"errors"
	"time"

	tasktime "github.com/Najah7/task2todaytodo/internal/application/task"
)

var (
	ErrRecurrenceIntervalWeeksLess = errors.New("recurrence interval weeks must be greater than or equal to 0")
	ErrRecurrenceTimezoneInvalid   = errors.New("recurrence timezone must be a valid IANA timezone")
	ErrRecurrenceMetadataInvalid   = errors.New("recurrence occurrence date and timezone are required")
)

const RecurrenceHorizonDays = 30

// ErrRecurrenceLimitInvalid is returned when a caller asks for no occurrences.
var ErrRecurrenceLimitInvalid = errors.New("recurrence occurrence limit must be greater than zero")

// RecurrenceDate describes one local calendar date in a recurrence series.
// Date is represented at UTC midnight so its calendar components stay stable.
type RecurrenceDate struct {
	Date time.Time
	Root bool
}

// RecurrenceMetadata persists occurrence lineage and tombstone state.
type RecurrenceMetadata struct {
	SeriesID       string
	OccurrenceDate time.Time
	Timezone       string
	IsException    bool
	Deleted        bool
}

// GenerateRecurrenceDates returns occurrences from the local date containing now
// through the inclusive horizon. firstOccurrence carries the series-local date
// in its calendar fields. The first occurrence is always the root date;
// later occurrences follow weekday and interval-week settings. An interval of
// four means 28 days, not one calendar month.
func GenerateRecurrenceDates(
	firstOccurrence time.Time,
	now time.Time,
	intervalWeeks int,
	frequencies TaskFrequencies,
	timezone string,
	horizonDays int,
) ([]RecurrenceDate, error) {
	if intervalWeeks < 0 {
		return nil, ErrRecurrenceIntervalWeeksLess
	}
	if horizonDays < 0 {
		return nil, ErrRecurrenceHorizonInvalid
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, ErrRecurrenceTimezoneInvalid
	}
	rootDate := tasktime.NormalizeCalendarDate(firstOccurrence)
	fromDate := tasktime.CalendarDate(now, location)
	toDate := fromDate.AddDate(0, 0, horizonDays)
	if rootDate.After(toDate) {
		return nil, nil
	}

	var dates []RecurrenceDate
	for date := rootDate; !date.After(toDate); date = date.AddDate(0, 0, 1) {
		if date.Before(fromDate) {
			continue
		}
		if date.Equal(rootDate) {
			dates = append(dates, RecurrenceDate{Date: date, Root: true})
			continue
		}
		if intervalWeeks == OnceIntervalWeeks || !isScheduledRecurrenceDate(rootDate, date, intervalWeeks, frequencies) {
			continue
		}
		dates = append(dates, RecurrenceDate{Date: date})
	}
	return dates, nil
}

// GenerateRecurrenceDatesLimit returns the first limit dates on or after fromDate.
// Unlike GenerateRecurrenceDates, this does not impose a calendar horizon, so
// list callers can request enough rows to fill a page regardless of cadence.
func GenerateRecurrenceDatesLimit(
	firstOccurrence time.Time,
	fromDate time.Time,
	intervalWeeks int,
	frequencies TaskFrequencies,
	timezone string,
	limit int,
) ([]RecurrenceDate, error) {
	if intervalWeeks < 0 {
		return nil, ErrRecurrenceIntervalWeeksLess
	}
	if limit < 1 {
		return nil, ErrRecurrenceLimitInvalid
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, ErrRecurrenceTimezoneInvalid
	}
	rootDate := tasktime.NormalizeCalendarDate(firstOccurrence)
	// fromDate is a calendar date, carried at UTC midnight, not an instant.
	// Default list callers first convert their fixed reference instant into the
	// series timezone; explicit dates already have the requested calendar fields.
	startDate := tasktime.NormalizeCalendarDate(fromDate)
	if startDate.Before(rootDate) {
		startDate = rootDate
	}
	if intervalWeeks == OnceIntervalWeeks {
		if rootDate.Before(startDate) {
			return nil, nil
		}
		return []RecurrenceDate{{Date: rootDate, Root: true}}, nil
	}
	maxDate := time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
	if startDate.After(maxDate) {
		return nil, nil
	}
	weekSpan := startOfRecurrenceWeek(maxDate).Sub(startOfRecurrenceWeek(rootDate)).Hours() / (24 * 7)
	if intervalWeeks > int(weekSpan) {
		if rootDate.Before(startDate) || rootDate.After(maxDate) {
			return nil, nil
		}
		return []RecurrenceDate{{Date: rootDate, Root: true}}, nil
	}

	dates := make([]RecurrenceDate, 0, limit)
	for date := startDate; len(dates) < limit && !date.After(maxDate); date = date.AddDate(0, 0, 1) {
		if date.Equal(rootDate) {
			dates = append(dates, RecurrenceDate{Date: date, Root: true})
			continue
		}
		if isScheduledRecurrenceDate(rootDate, date, intervalWeeks, frequencies) {
			dates = append(dates, RecurrenceDate{Date: date})
		}
	}
	return dates, nil
}

// GenerateRecurrenceDatesFromAnchorLimit returns rule-matching dates on or
// after fromDate. Unlike GenerateRecurrenceDatesLimit, anchor is only cadence
// phase; it is not implicitly an occurrence.
func GenerateRecurrenceDatesFromAnchorLimit(
	anchor time.Time,
	fromDate time.Time,
	intervalWeeks int,
	frequencies TaskFrequencies,
	timezone string,
	limit int,
) ([]RecurrenceDate, error) {
	if intervalWeeks < 1 {
		return nil, ErrRecurrenceIntervalWeeksLess
	}
	if limit < 1 {
		return nil, ErrRecurrenceLimitInvalid
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, ErrRecurrenceTimezoneInvalid
	}
	anchor = tasktime.NormalizeCalendarDate(anchor)
	startDate := tasktime.NormalizeCalendarDate(fromDate)
	if startDate.Before(anchor) {
		startDate = anchor
	}
	maxDate := time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
	dates := make([]RecurrenceDate, 0, limit)
	for date := startDate; len(dates) < limit && !date.After(maxDate); date = date.AddDate(0, 0, 1) {
		if isScheduledRecurrenceDate(anchor, date, intervalWeeks, frequencies) {
			dates = append(dates, RecurrenceDate{Date: date, Root: date.Equal(anchor)})
		}
	}
	return dates, nil
}

// CountRecurrenceDatesFromAnchor returns count of rule dates from anchor
// through target, inclusive. It gives paged projections stable series positions.
func CountRecurrenceDatesFromAnchor(anchor, target time.Time, intervalWeeks int, frequencies TaskFrequencies, timezone string) (int, error) {
	if intervalWeeks < 1 {
		return 0, ErrRecurrenceIntervalWeeksLess
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return 0, ErrRecurrenceTimezoneInvalid
	}
	anchor, target = tasktime.NormalizeCalendarDate(anchor), tasktime.NormalizeCalendarDate(target)
	if target.Before(anchor) {
		return 0, nil
	}
	count := 0
	for date := anchor; !date.After(target); date = date.AddDate(0, 0, 1) {
		if isScheduledRecurrenceDate(anchor, date, intervalWeeks, frequencies) {
			count++
		}
	}
	return count, nil
}

var ErrRecurrenceHorizonInvalid = errors.New("recurrence horizon days must be greater than or equal to 0")

func isScheduledRecurrenceDate(root, date time.Time, intervalWeeks int, frequencies TaskFrequencies) bool {
	rootWeek := startOfRecurrenceWeek(root)
	dateWeek := startOfRecurrenceWeek(date)
	weekOffset := int(dateWeek.Sub(rootWeek).Hours() / (24 * 7))
	if weekOffset%intervalWeeks != 0 {
		return false
	}
	weekday := date.Weekday()
	if len(frequencies) == 0 {
		return weekday == root.Weekday()
	}
	for _, frequency := range frequencies {
		if weekdayFromCode(frequency.Value) == weekday {
			return true
		}
	}
	return false
}

func startOfRecurrenceWeek(date time.Time) time.Time {
	daysSinceMonday := (int(date.Weekday()) + 6) % 7
	return date.AddDate(0, 0, -daysSinceMonday)
}

func weekdayFromCode(value string) time.Weekday {
	switch value {
	case "sun":
		return time.Sunday
	case "mon":
		return time.Monday
	case "tue":
		return time.Tuesday
	case "wed":
		return time.Wednesday
	case "thu":
		return time.Thursday
	case "fri":
		return time.Friday
	case "sat":
		return time.Saturday
	default:
		return time.Weekday(8)
	}
}

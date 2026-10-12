package calendar

import (
	"errors"
	"fmt"
	"time"
)

// RecurrenceRule describes local wall-clock times in an IANA timezone. Start,
// AnchorDate, End, and EndDate carry calendar fields; their locations are ignored.
type RecurrenceRule struct {
	Start      time.Time
	AnchorDate time.Time
	End        time.Time
	Timezone   string
	Frequency  string
	Interval   int
	Weekdays   []time.Weekday
	EndDate    *time.Time
}

type Occurrence struct {
	Date    time.Time
	StartAt time.Time
	EndAt   time.Time
}

type OccurrenceKey struct {
	RuleID         string
	OccurrenceDate string
}

var (
	ErrRecurrenceTimezoneInvalid = errors.New("recurrence timezone must be a valid IANA timezone")
	ErrFrequencyInvalid          = errors.New("invalid recurrence frequency")
	ErrIntervalInvalid           = errors.New("recurrence interval must be positive")
	ErrRangeInvalid              = errors.New("recurrence range is invalid")
)

// GenerateOccurrences evaluates a recurrence over inclusive local dates.
// DST gaps are omitted; folds resolve to their earlier instant.
func GenerateOccurrences(rule RecurrenceRule, fromDate, throughDate time.Time) ([]Occurrence, error) {
	location, err := time.LoadLocation(rule.Timezone)
	if err != nil {
		return nil, ErrRecurrenceTimezoneInvalid
	}
	if rule.Frequency != "once" && rule.Interval < 1 {
		return nil, ErrIntervalInvalid
	}
	if rule.Frequency != "once" && rule.Frequency != "daily" && rule.Frequency != "weekly" && rule.Frequency != "monthly" && rule.Frequency != "yearly" {
		return nil, ErrFrequencyInvalid
	}
	startDate := DateOnly(rule.Start)
	anchorDate := DateOnly(rule.AnchorDate)
	if anchorDate.IsZero() {
		anchorDate = startDate
	}
	fromDate, throughDate = DateOnly(fromDate), DateOnly(throughDate)
	if throughDate.Before(fromDate) {
		return nil, ErrRangeInvalid
	}
	if fromDate.Before(startDate) {
		fromDate = startDate
	}
	if fromDate.After(throughDate) {
		return nil, nil
	}
	endDate := time.Time{}
	if rule.EndDate != nil {
		endDate = DateOnly(*rule.EndDate)
	}
	startOffset := dateDifference(startDate, DateOnly(rule.End))
	if startOffset < 0 {
		return nil, ErrRangeInvalid
	}
	capacity := dateDifference(fromDate, throughDate) + 1
	if capacity > 4096 {
		capacity = 4096
	}
	out := make([]Occurrence, 0, capacity)
	for date := fromDate; !date.After(throughDate); date = date.AddDate(0, 0, 1) {
		if !endDate.IsZero() && date.After(endDate) {
			break
		}
		if !dateMatches(rule, startDate, anchorDate, date) {
			continue
		}
		start, ok := ResolveEarlier(date, rule.Start, 0, location)
		if !ok {
			continue
		}
		end, ok := ResolveEarlier(date, rule.End, startOffset, location)
		if !ok || !end.After(start) {
			continue
		}
		out = append(out, Occurrence{Date: date, StartAt: start, EndAt: end})
	}
	return out, nil
}

func dateMatches(rule RecurrenceRule, phase, anchor, date time.Time) bool {
	if date.Equal(phase) {
		if rule.Frequency != "weekly" || len(rule.Weekdays) == 0 {
			return true
		}
		for _, weekday := range rule.Weekdays {
			if date.Weekday() == weekday {
				return true
			}
		}
		return false
	}
	switch rule.Frequency {
	case "once":
		return false
	case "daily":
		return dateDifference(phase, date)%rule.Interval == 0
	case "weekly":
		weeks := dateDifference(monday(phase), monday(date)) / 7
		if weeks%rule.Interval != 0 {
			return false
		}
		if len(rule.Weekdays) == 0 {
			return date.Weekday() == phase.Weekday()
		}
		for _, weekday := range rule.Weekdays {
			if weekday == date.Weekday() {
				return true
			}
		}
		return false
	case "monthly":
		months := (date.Year()-phase.Year())*12 + int(date.Month()-phase.Month())
		return months >= 0 && months%rule.Interval == 0 && date.Day() == min(anchor.Day(), daysInMonth(date.Year(), date.Month()))
	case "yearly":
		years := date.Year() - phase.Year()
		return years >= 0 && years%rule.Interval == 0 && date.Month() == anchor.Month() && date.Day() == min(anchor.Day(), daysInMonth(date.Year(), date.Month()))
	default:
		return false
	}
}

// DateOnly returns the calendar date at UTC midnight, independent of location.
func DateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

// ParseLocalTimestamp parses local wall time accepted by Schedule APIs and
// database timestamps. Returned location is UTC only as a neutral carrier for
// the parsed calendar fields; it does not represent an instant.
func ParseLocalTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05.999999999", "2006-01-02T15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid local timestamp")
}

func dateDifference(from, to time.Time) int {
	return int(toDateNumber(DateOnly(to)) - toDateNumber(DateOnly(from)))
}
func toDateNumber(date time.Time) int64 {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400
}
func monday(date time.Time) time.Time {
	days := (int(date.Weekday()) + 6) % 7
	return date.AddDate(0, 0, -days)
}
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// ResolveEarlier resolves local wall time; nonexistent times return false and
// ambiguous times select earlier instant.
func ResolveEarlier(date, wall time.Time, dayOffset int, location *time.Location) (time.Time, bool) {
	localDate := date.AddDate(0, 0, dayOffset)
	wanted := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), wall.Hour(), wall.Minute(), wall.Second(), wall.Nanosecond(), time.UTC)
	base := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), wall.Hour(), wall.Minute(), wall.Second(), wall.Nanosecond(), location)
	candidates := make([]time.Time, 0, 2)
	seenOffsets := make(map[int]struct{})
	for delta := -48 * time.Hour; delta <= 48*time.Hour; delta += time.Hour {
		_, offset := base.Add(delta).In(location).Zone()
		if _, exists := seenOffsets[offset]; exists {
			continue
		}
		seenOffsets[offset] = struct{}{}
		candidate := wanted.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(location)
		if local.Year() == wanted.Year() && local.Month() == wanted.Month() && local.Day() == wanted.Day() && local.Hour() == wanted.Hour() && local.Minute() == wanted.Minute() && local.Second() == wanted.Second() && local.Nanosecond() == wanted.Nanosecond() {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return time.Time{}, false
	}
	earliest := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Before(earliest) {
			earliest = candidate
		}
	}
	return earliest, true
}

// ResolveForward resolves a local wall time, choosing the earlier instant in
// folds and moving a nonexistent time forward to the first valid wall minute.
// The seconds and nanoseconds are retained while scanning. The search is
// bounded to 48 hours to cover timezone transitions that skip a civil date.
func ResolveForward(date, wall time.Time, dayOffset int, location *time.Location) (time.Time, bool) {
	localDate := date.AddDate(0, 0, dayOffset)
	wallTime := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), wall.Hour(), wall.Minute(), wall.Second(), wall.Nanosecond(), time.UTC)
	for shift := time.Duration(0); shift <= 48*time.Hour; shift += time.Minute {
		candidateWall := wallTime.Add(shift)
		if resolved, ok := ResolveEarlier(candidateWall, candidateWall, 0, location); ok {
			return resolved, true
		}
	}
	return time.Time{}, false
}

// Overlaps uses half-open intervals [start,end).
func Overlaps(start, end, from, through time.Time) bool {
	return start.Before(through) && end.After(from)
}

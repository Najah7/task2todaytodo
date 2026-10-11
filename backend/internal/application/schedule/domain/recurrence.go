package domain

import (
	"errors"
	"time"
)

// BlueprintRule describes recurrence in the blueprint's local calendar.
// Start and End carry local date and clock fields; their locations are ignored.
type BlueprintRule struct {
	Start      time.Time
	AnchorDate time.Time
	End        time.Time
	Timezone   string
	Frequency  string
	Interval   int
	Weekdays   []time.Weekday
	EndDate    *time.Time
}

type BlueprintOccurrence struct {
	Date    time.Time
	StartAt time.Time
	EndAt   time.Time
}

var (
	ErrRecurrenceTimezoneInvalid = errors.New("recurrence timezone must be a valid IANA timezone")
	ErrBlueprintFrequencyInvalid = errors.New("invalid blueprint recurrence frequency")
	ErrBlueprintIntervalInvalid  = errors.New("blueprint recurrence interval must be positive")
	ErrBlueprintRangeInvalid     = errors.New("blueprint recurrence range is invalid")
)

// GenerateBlueprintOccurrences evaluates a rule over inclusive local dates.
// DST gaps are omitted; folds resolve to their earlier instant.
func GenerateBlueprintOccurrences(rule BlueprintRule, fromDate, throughDate time.Time) ([]BlueprintOccurrence, error) {
	location, err := time.LoadLocation(rule.Timezone)
	if err != nil {
		return nil, ErrRecurrenceTimezoneInvalid
	}
	if rule.Frequency != "once" && rule.Interval < 1 {
		return nil, ErrBlueprintIntervalInvalid
	}
	if rule.Frequency != "once" && rule.Frequency != "daily" && rule.Frequency != "weekly" && rule.Frequency != "monthly" && rule.Frequency != "yearly" {
		return nil, ErrBlueprintFrequencyInvalid
	}
	startDate := dateOnly(rule.Start)
	anchorDate := dateOnly(rule.AnchorDate)
	if anchorDate.IsZero() {
		anchorDate = startDate
	}
	fromDate, throughDate = dateOnly(fromDate), dateOnly(throughDate)
	if throughDate.Before(fromDate) {
		return nil, ErrBlueprintRangeInvalid
	}
	if fromDate.Before(startDate) {
		fromDate = startDate
	}
	if fromDate.After(throughDate) {
		return nil, nil
	}
	endDate := time.Time{}
	if rule.EndDate != nil {
		endDate = dateOnly(*rule.EndDate)
	}
	startOffset := dateDifference(startDate, dateOnly(rule.End))
	if startOffset < 0 {
		return nil, ErrBlueprintRangeInvalid
	}
	capacity := dateDifference(fromDate, throughDate) + 1
	if capacity > 4096 {
		capacity = 4096
	}
	out := make([]BlueprintOccurrence, 0, capacity)
	for date := fromDate; !date.After(throughDate); date = date.AddDate(0, 0, 1) {
		if !endDate.IsZero() && date.After(endDate) {
			break
		}
		if !blueprintDateMatches(rule, startDate, anchorDate, date) {
			continue
		}
		start, ok := resolveEarlier(date, rule.Start, 0, location)
		if !ok {
			continue
		}
		end, ok := resolveEarlier(date, rule.End, startOffset, location)
		if !ok || !end.After(start) {
			continue
		}
		out = append(out, BlueprintOccurrence{Date: date, StartAt: start, EndAt: end})
	}
	return out, nil
}

func blueprintDateMatches(rule BlueprintRule, phase, anchor, date time.Time) bool {
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
		if months < 0 || months%rule.Interval != 0 {
			return false
		}
		return date.Day() == min(anchor.Day(), daysInMonth(date.Year(), date.Month()))
	case "yearly":
		years := date.Year() - phase.Year()
		if years < 0 || years%rule.Interval != 0 || date.Month() != anchor.Month() {
			return false
		}
		return date.Day() == min(anchor.Day(), daysInMonth(date.Year(), date.Month()))
	default:
		return false
	}
}

func resolveEarlier(date, wall time.Time, dayOffset int, location *time.Location) (time.Time, bool) {
	localDate := date.AddDate(0, 0, dayOffset)
	wanted := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), wall.Hour(), wall.Minute(), wall.Second(), wall.Nanosecond(), time.UTC)
	// Sample possible offsets around the transition, then retain exact wall-time matches.
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
		if local.Year() == wanted.Year() && local.Month() == wanted.Month() && local.Day() == wanted.Day() &&
			local.Hour() == wanted.Hour() && local.Minute() == wanted.Minute() && local.Second() == wanted.Second() && local.Nanosecond() == wanted.Nanosecond() {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return time.Time{}, false
	}
	var earliest time.Time
	for _, candidate := range candidates {
		if earliest.IsZero() || candidate.Before(earliest) {
			earliest = candidate
		}
	}
	return earliest, true
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func dateDifference(from, to time.Time) int {
	return int(toDateNumber(dateOnly(to)) - toDateNumber(dateOnly(from)))
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

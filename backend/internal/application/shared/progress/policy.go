// Package progress contains progress and virtual-occurrence rules shared by
// Task and Project read models.
package progress

import (
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

// RecurrenceRule contains the stored root metadata needed to evaluate a
// recurring virtual occurrence for a local calendar date.
type RecurrenceRule struct {
	OccurrenceDate       string
	Timezone             string
	IntervalWeeks        int
	FrequencyAnchorDate  string
	Frequencies          []string
	OccurrenceSavedToday bool
	StartAt              string
	EndAt                string
}

// VirtualOccurrenceOccursToday reports whether an unsaved recurrence root has
// an occurrence today. StartAt and EndAt are required together when the work
// has fixed wall times; nonexistent daylight-saving wall times are ineligible.
func VirtualOccurrenceOccursToday(rule RecurrenceRule, asOf time.Time) (bool, error) {
	if rule.OccurrenceSavedToday || rule.IntervalWeeks < 1 {
		return false, nil
	}
	location, err := time.LoadLocation(rule.Timezone)
	if err != nil {
		return false, recurrence.ErrRecurrenceTimezoneInvalid
	}
	first, err := parseCalendarDate(rule.OccurrenceDate)
	if err != nil {
		return false, fmt.Errorf("parse recurrence occurrence date: %w", err)
	}
	today := calendar.CalendarDate(asOf, location)
	if today.Before(first) {
		return false, nil
	}
	anchor := first
	if rule.FrequencyAnchorDate != "" {
		anchor, err = parseCalendarDate(rule.FrequencyAnchorDate)
		if err != nil {
			return false, fmt.Errorf("parse recurrence frequency anchor: %w", err)
		}
	}
	frequencies := make(recurrence.Frequencies, 0, len(rule.Frequencies))
	for _, value := range rule.Frequencies {
		frequency, frequencyErr := recurrence.NewFrequency(value)
		if frequencyErr != nil {
			return false, frequencyErr
		}
		frequencies = append(frequencies, frequency)
	}
	dates, err := recurrence.GenerateRecurrenceDatesFromAnchorLimit(anchor, today, rule.IntervalWeeks, frequencies, rule.Timezone, 1)
	if err != nil {
		return false, err
	}
	if len(dates) == 0 || !dates[0].Date.Equal(today) {
		return false, nil
	}
	if rule.StartAt == "" && rule.EndAt == "" {
		return true, nil
	}
	startAt, err := time.Parse(time.RFC3339Nano, rule.StartAt)
	if err != nil {
		return false, fmt.Errorf("parse recurrence start time: %w", err)
	}
	endAt, err := time.Parse(time.RFC3339Nano, rule.EndAt)
	if err != nil {
		return false, fmt.Errorf("parse recurrence end time: %w", err)
	}
	startLocal, endLocal := startAt.In(location), endAt.In(location)
	if _, ok := calendar.ResolveWallTime(today, startLocal, 0, location); !ok {
		return false, nil
	}
	if _, ok := calendar.ResolveWallTime(today, endLocal, calendar.CalendarDayOffset(startLocal, endLocal), location); !ok {
		return false, nil
	}
	return true, nil
}

// TaskPercent returns the exposed whole percentage for a Task. The exact
// completed/total comparison remains separately available to Task mutations.
func TaskPercent(done bool, completed, total int) int {
	if done {
		return 100
	}
	if total < 1 {
		return 0
	}
	return 100 * completed / total
}

func parseCalendarDate(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Format("2006-01-02") != value {
		return time.Time{}, fmt.Errorf("invalid calendar date %q", value)
	}
	return date, nil
}

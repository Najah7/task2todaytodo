package domain

import (
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

// Recurrence types and errors remain aliases for the Task context's TodoItem
// API. Recurrence policy and generation live in application/shared/recurrence.
type RecurrenceDate = recurrence.RecurrenceDate
type RecurrenceMetadata = recurrence.Metadata

var (
	ErrRecurrenceIntervalWeeksLess = recurrence.ErrRecurrenceIntervalWeeksLess
	ErrRecurrenceTimezoneInvalid   = recurrence.ErrRecurrenceTimezoneInvalid
	ErrRecurrenceMetadataInvalid   = recurrence.ErrRecurrenceMetadataInvalid
	ErrRecurrenceLimitInvalid      = recurrence.ErrRecurrenceLimitInvalid
	ErrRecurrenceHorizonInvalid    = recurrence.ErrRecurrenceHorizonInvalid
)

const RecurrenceHorizonDays = recurrence.RecurrenceHorizonDays

func GenerateRecurrenceDates(firstOccurrence, now time.Time, intervalWeeks int, frequencies TaskFrequencies, timezone string, horizonDays int) ([]RecurrenceDate, error) {
	return recurrence.GenerateRecurrenceDates(firstOccurrence, now, intervalWeeks, frequencies, timezone, horizonDays)
}

func GenerateRecurrenceDatesLimit(firstOccurrence, fromDate time.Time, intervalWeeks int, frequencies TaskFrequencies, timezone string, limit int) ([]RecurrenceDate, error) {
	return recurrence.GenerateRecurrenceDatesLimit(firstOccurrence, fromDate, intervalWeeks, frequencies, timezone, limit)
}

func GenerateRecurrenceDatesFromAnchorLimit(anchor, fromDate time.Time, intervalWeeks int, frequencies TaskFrequencies, timezone string, limit int) ([]RecurrenceDate, error) {
	return recurrence.GenerateRecurrenceDatesFromAnchorLimit(anchor, fromDate, intervalWeeks, frequencies, timezone, limit)
}

func CountRecurrenceDatesFromAnchor(anchor, target time.Time, intervalWeeks int, frequencies TaskFrequencies, timezone string) (int, error) {
	return recurrence.CountRecurrenceDatesFromAnchor(anchor, target, intervalWeeks, frequencies, timezone)
}

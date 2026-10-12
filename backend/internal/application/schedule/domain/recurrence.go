package domain

import (
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

type RecurrenceRule = calendar.RecurrenceRule
type Occurrence = calendar.Occurrence

var (
	ErrRecurrenceTimezoneInvalid = calendar.ErrRecurrenceTimezoneInvalid
	ErrRuleFrequencyInvalid      = calendar.ErrFrequencyInvalid
	ErrRuleIntervalInvalid       = calendar.ErrIntervalInvalid
	ErrRuleRangeInvalid          = calendar.ErrRangeInvalid
)

// GenerateOccurrences delegates recurrence and timezone policy to shared calendar.
func GenerateOccurrences(rule RecurrenceRule, fromDate, throughDate time.Time) ([]Occurrence, error) {
	return calendar.GenerateOccurrences(rule, fromDate, throughDate)
}

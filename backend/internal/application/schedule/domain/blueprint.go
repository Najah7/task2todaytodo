package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	FrequencyOnce    = "once"
	FrequencyDaily   = "daily"
	FrequencyWeekly  = "weekly"
	FrequencyMonthly = "monthly"
	FrequencyYearly  = "yearly"
)

var (
	ErrBlueprintIDEmpty       = errors.New("schedule blueprint ID cannot be empty")
	ErrBlueprintTitleEmpty    = errors.New("schedule blueprint title cannot be empty")
	ErrBlueprintTimezoneEmpty = errors.New("schedule blueprint timezone cannot be empty")
	ErrBlueprintFrequency     = errors.New("schedule blueprint frequency is invalid")
	ErrBlueprintInterval      = errors.New("schedule blueprint interval is invalid")
	ErrBlueprintWeekdays      = errors.New("weekly schedule blueprint needs valid weekdays")
	ErrBlueprintDateRange     = errors.New("schedule blueprint date range is invalid")
	ErrBlueprintTimeRange     = errors.New("schedule blueprint end must be after start")
)

type BlueprintID string
type ScheduleOccurrenceID string

// BlueprintDefinition contains local wall times; no UTC conversion belongs in
// this value. Its timezone determines the instant for each generated date.
type BlueprintDefinition struct {
	Title       string
	Description string
	Location    string
	ProjectID   string
	AssigneeID  string
	LocalStart  time.Time
	LocalEnd    time.Time
	Timezone    string
	Frequency   string
	Interval    int
	Weekdays    []time.Weekday
	EndDate     *time.Time
}

func NewBlueprintDefinition(definition BlueprintDefinition) (BlueprintDefinition, error) {
	definition.Weekdays = append([]time.Weekday(nil), definition.Weekdays...)
	if strings.TrimSpace(definition.Title) == "" {
		return BlueprintDefinition{}, ErrBlueprintTitleEmpty
	}
	if strings.TrimSpace(definition.Timezone) == "" {
		return BlueprintDefinition{}, ErrBlueprintTimezoneEmpty
	}
	if definition.LocalStart.IsZero() || definition.LocalEnd.IsZero() || !definition.LocalEnd.After(definition.LocalStart) {
		return BlueprintDefinition{}, ErrBlueprintTimeRange
	}
	switch definition.Frequency {
	case FrequencyOnce:
		if definition.Interval != 0 || len(definition.Weekdays) != 0 || definition.EndDate != nil {
			return BlueprintDefinition{}, ErrBlueprintInterval
		}
	case FrequencyDaily, FrequencyWeekly, FrequencyMonthly, FrequencyYearly:
		if definition.Interval < 1 {
			return BlueprintDefinition{}, ErrBlueprintInterval
		}
	default:
		return BlueprintDefinition{}, ErrBlueprintFrequency
	}
	if definition.Frequency == FrequencyWeekly {
		if len(definition.Weekdays) == 0 {
			return BlueprintDefinition{}, ErrBlueprintWeekdays
		}
		seen := make(map[time.Weekday]struct{}, len(definition.Weekdays))
		for _, weekday := range definition.Weekdays {
			if weekday < time.Sunday || weekday > time.Saturday {
				return BlueprintDefinition{}, ErrBlueprintWeekdays
			}
			if _, ok := seen[weekday]; ok {
				return BlueprintDefinition{}, ErrBlueprintWeekdays
			}
			seen[weekday] = struct{}{}
		}
	} else if len(definition.Weekdays) != 0 {
		return BlueprintDefinition{}, ErrBlueprintWeekdays
	}
	startDate := time.Date(definition.LocalStart.Year(), definition.LocalStart.Month(), definition.LocalStart.Day(), 0, 0, 0, 0, time.UTC)
	if definition.EndDate != nil {
		endDate := time.Date(definition.EndDate.Year(), definition.EndDate.Month(), definition.EndDate.Day(), 0, 0, 0, 0, time.UTC)
		if endDate.Before(startDate) {
			return BlueprintDefinition{}, ErrBlueprintDateRange
		}
		definition.EndDate = &endDate
	}
	return definition, nil
}

func NewBlueprintID(value string) (BlueprintID, error) {
	if strings.TrimSpace(value) == "" {
		return "", ErrBlueprintIDEmpty
	}
	return BlueprintID(value), nil
}

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
	ErrRuleIDEmpty       = errors.New("schedule rule ID cannot be empty")
	ErrRuleTitleEmpty    = errors.New("schedule rule title cannot be empty")
	ErrRuleTimezoneEmpty = errors.New("schedule rule timezone cannot be empty")
	ErrRuleFrequency     = errors.New("schedule rule frequency is invalid")
	ErrRuleInterval      = errors.New("schedule rule interval is invalid")
	ErrRuleWeekdays      = errors.New("weekly schedule rule needs valid weekdays")
	ErrRuleDateRange     = errors.New("schedule rule date range is invalid")
	ErrRuleTimeRange     = errors.New("schedule rule end must be after start")
	ErrRuleAllDayTime    = errors.New("all-day schedule boundaries must be local midnight")
)

type RuleID string
type ScheduleOccurrenceID string

// RuleDefinition contains local wall times; no UTC conversion belongs in
// this value. Its timezone determines the instant for each generated date.
type RuleDefinition struct {
	Title       string
	Description string
	Location    string
	ProjectID   string
	AssigneeID  string
	LocalStart  time.Time
	LocalEnd    time.Time
	AllDay      bool
	Timezone    string
	Frequency   string
	Interval    int
	Weekdays    []time.Weekday
	EndDate     *time.Time
}

func NewRuleDefinition(definition RuleDefinition) (RuleDefinition, error) {
	definition.Weekdays = append([]time.Weekday(nil), definition.Weekdays...)
	if strings.TrimSpace(definition.Title) == "" {
		return RuleDefinition{}, ErrRuleTitleEmpty
	}
	if strings.TrimSpace(definition.Timezone) == "" {
		return RuleDefinition{}, ErrRuleTimezoneEmpty
	}
	if definition.LocalStart.IsZero() || definition.LocalEnd.IsZero() || !definition.LocalEnd.After(definition.LocalStart) {
		return RuleDefinition{}, ErrRuleTimeRange
	}
	if definition.AllDay && (!isLocalMidnight(definition.LocalStart) || !isLocalMidnight(definition.LocalEnd)) {
		return RuleDefinition{}, ErrRuleAllDayTime
	}
	switch definition.Frequency {
	case FrequencyOnce:
		if definition.Interval != 0 || len(definition.Weekdays) != 0 || definition.EndDate != nil {
			return RuleDefinition{}, ErrRuleInterval
		}
	case FrequencyDaily, FrequencyWeekly, FrequencyMonthly, FrequencyYearly:
		if definition.Interval < 1 {
			return RuleDefinition{}, ErrRuleInterval
		}
	default:
		return RuleDefinition{}, ErrRuleFrequency
	}
	if definition.Frequency == FrequencyWeekly {
		if len(definition.Weekdays) == 0 {
			return RuleDefinition{}, ErrRuleWeekdays
		}
		seen := make(map[time.Weekday]struct{}, len(definition.Weekdays))
		for _, weekday := range definition.Weekdays {
			if weekday < time.Sunday || weekday > time.Saturday {
				return RuleDefinition{}, ErrRuleWeekdays
			}
			if _, ok := seen[weekday]; ok {
				return RuleDefinition{}, ErrRuleWeekdays
			}
			seen[weekday] = struct{}{}
		}
	} else if len(definition.Weekdays) != 0 {
		return RuleDefinition{}, ErrRuleWeekdays
	}
	startDate := time.Date(definition.LocalStart.Year(), definition.LocalStart.Month(), definition.LocalStart.Day(), 0, 0, 0, 0, time.UTC)
	if definition.EndDate != nil {
		endDate := time.Date(definition.EndDate.Year(), definition.EndDate.Month(), definition.EndDate.Day(), 0, 0, 0, 0, time.UTC)
		if endDate.Before(startDate) {
			return RuleDefinition{}, ErrRuleDateRange
		}
		definition.EndDate = &endDate
	}
	return definition, nil
}

func isLocalMidnight(value time.Time) bool {
	return value.Hour() == 0 && value.Minute() == 0 && value.Second() == 0 && value.Nanosecond() == 0
}

func NewRuleID(value string) (RuleID, error) {
	if strings.TrimSpace(value) == "" {
		return "", ErrRuleIDEmpty
	}
	return RuleID(value), nil
}

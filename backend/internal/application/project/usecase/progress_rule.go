package usecase

import (
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

func projectProgressRule(root dao.ProjectProgressRecurrence) (domain.ScheduleProgressRule, error) {
	start, err := time.Parse("2006-01-02 15:04:05", root.LocalStart)
	if err != nil {
		return domain.ScheduleProgressRule{}, err
	}
	end, err := time.Parse("2006-01-02 15:04:05", root.LocalEnd)
	if err != nil {
		return domain.ScheduleProgressRule{}, err
	}
	anchor, err := time.Parse("2006-01-02", root.RecurrenceAnchorDate)
	if err != nil {
		return domain.ScheduleProgressRule{}, err
	}
	rule := domain.ScheduleProgressRule{
		Rule: recurrence.BlueprintRule{
			Start: start, End: end, AnchorDate: anchor, Timezone: root.Timezone,
			Frequency: root.Frequency, Interval: root.Interval, Weekdays: recurrenceWeekdays(root.Weekdays),
		},
		OccurrenceSavedToday: root.OccurrenceSavedToday,
	}
	if root.EndDate != nil {
		endDate, err := time.Parse("2006-01-02", *root.EndDate)
		if err != nil {
			return domain.ScheduleProgressRule{}, err
		}
		rule.Rule.EndDate = &endDate
	}
	if root.CutoffAfter != nil {
		cutoff, err := time.Parse("2006-01-02", *root.CutoffAfter)
		if err != nil {
			return domain.ScheduleProgressRule{}, err
		}
		if rule.Rule.EndDate == nil || cutoff.Before(*rule.Rule.EndDate) {
			rule.Rule.EndDate = &cutoff
		}
	}
	for _, pause := range root.PauseIntervals {
		pausedAt, err := time.Parse(time.RFC3339Nano, pause.PausedAt)
		if err != nil {
			return domain.ScheduleProgressRule{}, err
		}
		interval := domain.ScheduleProgressPause{PausedAt: pausedAt}
		if pause.ResumedAt != nil {
			resumedAt, err := time.Parse(time.RFC3339Nano, *pause.ResumedAt)
			if err != nil {
				return domain.ScheduleProgressRule{}, err
			}
			interval.ResumedAt = &resumedAt
		}
		rule.PauseIntervals = append(rule.PauseIntervals, interval)
	}
	return rule, nil
}

func recurrenceWeekdays(values []string) []time.Weekday {
	weekdays := make([]time.Weekday, 0, len(values))
	for _, value := range values {
		switch value {
		case "sun":
			weekdays = append(weekdays, time.Sunday)
		case "mon":
			weekdays = append(weekdays, time.Monday)
		case "tue":
			weekdays = append(weekdays, time.Tuesday)
		case "wed":
			weekdays = append(weekdays, time.Wednesday)
		case "thu":
			weekdays = append(weekdays, time.Thursday)
		case "fri":
			weekdays = append(weekdays, time.Friday)
		case "sat":
			weekdays = append(weekdays, time.Saturday)
		}
	}
	return weekdays
}

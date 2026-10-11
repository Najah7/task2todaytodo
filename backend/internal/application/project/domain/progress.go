package domain

import (
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

type TaskProgressFacts struct {
	Progress int
}

type ScheduleProgressFacts struct {
	Total     int
	Completed int
	Roots     []ScheduleProgressRule
}

type ScheduleProgressPause struct {
	PausedAt  time.Time
	ResumedAt *time.Time
}

type ScheduleProgressRule struct {
	Rule                 recurrence.BlueprintRule
	OccurrenceSavedToday bool
	PauseIntervals       []ScheduleProgressPause
}

// CalculateProjectProgress returns the truncated mean of each Task's exposed
// percentage and each eligible Schedule occurrence as its own 0/100 value.
func CalculateProjectProgress(tasks []TaskProgressFacts, schedules []ScheduleProgressFacts, asOf time.Time) (int, error) {
	completed, total := 0, 0
	for _, task := range tasks {
		completed += task.Progress
		total++
	}
	for _, schedule := range schedules {
		scheduleTotal := schedule.Total
		for _, root := range schedule.Roots {
			eligible, err := VirtualScheduleOccurrenceOccursToday(root, asOf)
			if err != nil {
				return 0, err
			}
			if eligible {
				scheduleTotal++
			}
		}
		completed += schedule.Completed * 100
		total += scheduleTotal
	}
	if total == 0 {
		return 0, nil
	}
	return completed / total, nil
}

func VirtualScheduleOccurrenceOccursToday(root ScheduleProgressRule, asOf time.Time) (bool, error) {
	if root.OccurrenceSavedToday {
		return false, nil
	}
	location, err := time.LoadLocation(root.Rule.Timezone)
	if err != nil {
		return false, recurrence.ErrRecurrenceTimezoneInvalid
	}
	localNow := asOf.In(location)
	date := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	occurrences, err := recurrence.GenerateBlueprintOccurrences(root.Rule, date, date)
	if err != nil || len(occurrences) == 0 {
		return false, err
	}
	startAt := occurrences[0].StartAt
	for _, interval := range root.PauseIntervals {
		if startAt.Before(interval.PausedAt) {
			continue
		}
		if interval.ResumedAt == nil || startAt.Before(*interval.ResumedAt) {
			return false, nil
		}
	}
	return true, nil
}

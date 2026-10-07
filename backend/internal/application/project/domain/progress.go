package domain

import (
	"time"

	sharedprogress "github.com/Najah7/task2todaytodo/internal/application/shared/progress"
)

type TaskProgressFacts struct {
	Done      bool
	Total     int
	Completed int
	Roots     []sharedprogress.RecurrenceRule
}

type ScheduleProgressFacts struct {
	Total     int
	Completed int
	Roots     []sharedprogress.RecurrenceRule
}

// CalculateProjectProgress returns the truncated mean of each Task's exposed
// percentage and each eligible Schedule occurrence as its own 0/100 value.
func CalculateProjectProgress(tasks []TaskProgressFacts, schedules []ScheduleProgressFacts, asOf time.Time) (int, error) {
	completed, total := 0, 0
	for _, task := range tasks {
		taskTotal := task.Total
		if !task.Done {
			for _, root := range task.Roots {
				eligible, err := sharedprogress.VirtualOccurrenceOccursToday(root, asOf)
				if err == nil && eligible {
					taskTotal++
				}
			}
		}
		completed += sharedprogress.TaskPercent(task.Done, task.Completed, taskTotal)
		total++
	}
	for _, schedule := range schedules {
		scheduleTotal := schedule.Total
		for _, root := range schedule.Roots {
			eligible, err := sharedprogress.VirtualOccurrenceOccursToday(root, asOf)
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

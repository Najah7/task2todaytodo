package domain

type TaskProgressFacts struct {
	Progress int
}

type ScheduleProgressFacts struct {
	Total     int
	Completed int
}

// CalculateProjectProgress returns the truncated mean of each Task's exposed
// percentage and each persisted Schedule occurrence as its own 0/100 value.
func CalculateProjectProgress(tasks []TaskProgressFacts, schedules []ScheduleProgressFacts) int {
	completed, total := 0, 0
	for _, task := range tasks {
		completed += task.Progress
		total++
	}
	for _, schedule := range schedules {
		completed += schedule.Completed * 100
		total += schedule.Total
	}
	if total == 0 {
		return 0
	}
	return completed / total
}

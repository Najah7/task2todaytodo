package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	sharedprogress "github.com/Najah7/task2todaytodo/internal/application/shared/progress"
)

type ProjectProgressReader interface {
	ReadProjectProgressSources(context.Context, []string, time.Time) (dao.ProjectProgressSources, error)
}

func applyProjectProgress(ctx context.Context, reader ProjectProgressReader, projects []dao.Project, asOf time.Time) ([]dao.Project, error) {
	if len(projects) == 0 {
		return projects, nil
	}
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	sources, err := reader.ReadProjectProgressSources(ctx, ids, asOf)
	if err != nil {
		return nil, err
	}
	tasksByProject := make(map[string][]domain.TaskProgressFacts, len(projects))
	for _, task := range sources.Tasks {
		facts := domain.TaskProgressFacts{Done: task.Done, Total: task.Total, Completed: task.Completed}
		for _, root := range task.Roots {
			facts.Roots = append(facts.Roots, progressRule(root))
		}
		tasksByProject[task.ProjectID] = append(tasksByProject[task.ProjectID], facts)
	}
	schedulesByProject := make(map[string][]domain.ScheduleProgressFacts, len(projects))
	for _, schedule := range sources.Schedules {
		facts := domain.ScheduleProgressFacts{Total: schedule.Total, Completed: schedule.Completed}
		for _, root := range schedule.Roots {
			facts.Roots = append(facts.Roots, progressRule(root))
		}
		schedulesByProject[schedule.ProjectID] = append(schedulesByProject[schedule.ProjectID], facts)
	}
	for index := range projects {
		projects[index].Progress, err = domain.CalculateProjectProgress(tasksByProject[projects[index].ID], schedulesByProject[projects[index].ID], asOf)
		if err != nil {
			return nil, err
		}
	}
	return projects, nil
}

func progressRule(root dao.ProjectProgressRecurrence) sharedprogress.RecurrenceRule {
	return sharedprogress.RecurrenceRule{
		OccurrenceDate: root.OccurrenceDate, Timezone: root.Timezone, IntervalWeeks: root.IntervalWeeks,
		FrequencyAnchorDate: root.FrequencyAnchorDate, Frequencies: root.Frequencies,
		OccurrenceSavedToday: root.OccurrenceSavedToday, StartAt: root.StartAt, EndAt: root.EndAt,
	}
}

package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *ProjectRepository) ReadProjectProgressSources(ctx context.Context, projectIDs []string, asOf time.Time) (dao.ProjectProgressSources, error) {
	sources := dao.ProjectProgressSources{}
	if len(projectIDs) == 0 {
		return sources, nil
	}

	taskRows, err := r.queries.ReadTaskProgressSources(ctx, sqlc.ReadTaskProgressSourcesParams{ProjectIds: projectIDs})
	if err != nil {
		return dao.ProjectProgressSources{}, err
	}
	for _, row := range taskRows {
		task := dao.ProjectTaskProgress{
			ProjectID: row.ProjectID.String, TaskID: row.TaskID, UserID: row.UserID, Done: row.Status == "done", ProjectStatus: row.ProjectStatus,
			Total: int(row.ActionItemTotal), Completed: int(row.ActionItemCompleted),
		}
		sources.Tasks = append(sources.Tasks, task)
	}

	scheduleIndexes := make(map[string]int)
	scheduleCounts, err := r.queries.ReadProjectScheduleProgressCounts(ctx, projectIDs)
	if err != nil {
		return dao.ProjectProgressSources{}, err
	}
	for _, row := range scheduleCounts {
		sources.Schedules = append(sources.Schedules, dao.ProjectScheduleProgress{
			ProjectID: row.ProjectID.String, Total: int(row.Total), Completed: int(row.Completed),
		})
		scheduleIndexes[row.ProjectID.String] = len(sources.Schedules) - 1
	}
	scheduleRoots, err := r.queries.ReadProjectScheduleProgressRoots(ctx, sqlc.ReadProjectScheduleProgressRootsParams{
		AsOf: projectProgressTime(asOf), ProjectIds: projectIDs,
	})
	if err != nil {
		return dao.ProjectProgressSources{}, err
	}
	for _, row := range scheduleRoots {
		index, ok := scheduleIndexes[row.ProjectID.String]
		if !ok {
			sources.Schedules = append(sources.Schedules, dao.ProjectScheduleProgress{ProjectID: row.ProjectID.String})
			index = len(sources.Schedules) - 1
			scheduleIndexes[row.ProjectID.String] = index
		}
		sources.Schedules[index].Roots = append(sources.Schedules[index].Roots, dao.ProjectProgressRecurrence{
			OccurrenceDate: row.OccurrenceDate, Timezone: row.Timezone, IntervalWeeks: int(row.IntervalWeeks),
			FrequencyAnchorDate: row.FrequencyAnchorDate.Time.Format("2006-01-02"), Frequencies: row.Frequencies,
			OccurrenceSavedToday: row.OccurrenceSavedToday.Bool,
			StartAt:              row.StartAt.Time.UTC().Format(time.RFC3339Nano), EndAt: row.EndAt.Time.UTC().Format(time.RFC3339Nano),
		})
	}
	return sources, nil
}

func projectProgressTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

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
		rootKey := row.ProjectID.String + "/" + row.BlueprintID + "/" + row.OccurrenceDate
		index, ok := scheduleIndexes[rootKey]
		if !ok {
			index, ok = scheduleIndexes[row.ProjectID.String]
			if !ok {
				sources.Schedules = append(sources.Schedules, dao.ProjectScheduleProgress{ProjectID: row.ProjectID.String})
				index = len(sources.Schedules) - 1
				scheduleIndexes[row.ProjectID.String] = index
			}
			recurrence := dao.ProjectProgressRecurrence{
				BlueprintID: row.BlueprintID, RecurrenceAnchorDate: row.RecurrenceAnchorDate,
				LocalStart: row.LocalStart.Time.Format("2006-01-02 15:04:05"), LocalEnd: row.LocalEnd.Time.Format("2006-01-02 15:04:05"),
				Timezone: row.Timezone, Frequency: row.Frequency, Interval: int(row.IntervalCount),
				Weekdays: row.Weekdays, OccurrenceDate: row.OccurrenceDate,
				OccurrenceSavedToday: row.OccurrenceSavedToday,
			}
			if row.CutoffAfter != "" {
				recurrence.CutoffAfter = &row.CutoffAfter
			}
			if row.EndDate != "" {
				recurrence.EndDate = &row.EndDate
			}
			sources.Schedules[index].Roots = append(sources.Schedules[index].Roots, recurrence)
			scheduleIndexes[rootKey] = len(sources.Schedules[index].Roots) - 1
		}
		rootIndex := scheduleIndexes[rootKey]
		if row.PauseStartedAt.Valid {
			pause := dao.ProjectSchedulePauseInterval{PausedAt: row.PauseStartedAt.Time.UTC().Format(time.RFC3339Nano)}
			if row.PauseEndedAt.Valid {
				resumed := row.PauseEndedAt.Time.UTC().Format(time.RFC3339Nano)
				pause.ResumedAt = &resumed
			}
			sources.Schedules[index].Roots[rootIndex].PauseIntervals = append(sources.Schedules[index].Roots[rootIndex].PauseIntervals, pause)
		}
	}
	return sources, nil
}

func projectProgressTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

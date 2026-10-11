package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
)

func (r *ProjectRepository) ReadProjectProgressSources(ctx context.Context, projectIDs []string, _ time.Time) (dao.ProjectProgressSources, error) {
	sources := dao.ProjectProgressSources{}
	if len(projectIDs) == 0 {
		return sources, nil
	}

	taskRows, err := r.queries.ReadTaskProgressSources(ctx, sqlc.ReadTaskProgressSourcesParams{ProjectIds: projectIDs})
	if err != nil {
		return dao.ProjectProgressSources{}, err
	}
	for _, row := range taskRows {
		sources.Tasks = append(sources.Tasks, dao.ProjectTaskProgress{
			ProjectID: row.ProjectID.String, TaskDone: row.Status == "done",
			TotalActionItems: int(row.ActionItemTotal), CompletedActionItems: int(row.ActionItemCompleted),
		})
	}

	scheduleRows, err := r.queries.ReadProjectScheduleProgressCounts(ctx, projectIDs)
	if err != nil {
		return dao.ProjectProgressSources{}, err
	}
	for _, row := range scheduleRows {
		sources.Schedules = append(sources.Schedules, dao.ProjectScheduleProgress{
			ProjectID: row.ProjectID.String, Total: int(row.Total), Completed: int(row.Completed),
		})
	}
	return sources, nil
}

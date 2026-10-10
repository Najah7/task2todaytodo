package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

// TaskListProjectionReader loads saved ActionItem rows and skipped recurrence dates for Tasks in one read.
type TaskListProjectionReader interface {
	ReadTaskListProjection(ctx context.Context, userID domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error)
}

// EnrichTaskList calculates Task progress and effective estimates from the same finite ActionItem list projection.
func EnrichTaskList(ctx context.Context, reader TaskListProjectionReader, userID domain.UserID, tasks []dao.Task, asOf time.Time) ([]dao.Task, error) {
	if len(tasks) == 0 {
		return tasks, nil
	}
	if reader == nil {
		return nil, ErrTaskProjectionUnavailable
	}
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	sources, err := reader.ReadTaskListProjection(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	return ApplyTaskListProjection(tasks, sources, asOf)
}

// EnrichTasksInRepositories loads project state and ActionItem projections
// from the current unit of work before applying Task's shared read model.
func EnrichTasksInRepositories(ctx context.Context, repos Repositories, userID domain.UserID, tasks []dao.Task, asOf time.Time) ([]dao.Task, error) {
	for i := range tasks {
		if tasks[i].ProjectID == "" || repos.ProjectLifecycle() == nil {
			continue
		}
		status, err := repos.ProjectLifecycle().ReadStatus(ctx, tasks[i].ProjectID)
		if err != nil {
			return nil, err
		}
		tasks[i].ProjectStatus = status
	}
	return EnrichTaskList(ctx, repos.ActionItems(), userID, tasks, asOf)
}

// ApplyTaskListProjection is the Task-owned SSOT for list counts, progress, and effective estimates.
func ApplyTaskListProjection(tasks []dao.Task, sources dao.TaskListProjectionSources, asOf time.Time) ([]dao.Task, error) {
	for i := range tasks {
		task := &tasks[i]
		rows := sources.ActionItemsByTask[task.ID]
		skipped := sources.SkippedByTask[task.ID]
		projected, err := ExpandActionItemListRows(rows, asOf, task.Status.Value == "done", task.ProjectStatus == "done", skipped)
		if err != nil {
			return nil, err
		}
		task.ActionItemCount = len(projected)
		task.ActionItemCompletedCount = 0
		var total int
		hasEstimate := false
		for _, item := range projected {
			if item.Completed {
				task.ActionItemCompletedCount++
			}
			if item.EstimatedMinutes != nil {
				total += *item.EstimatedMinutes
				hasEstimate = true
			}
		}
		definitions := 0
		for _, row := range rows {
			if row.ID == row.SeriesID && !row.Deleted {
				definitions++
			}
		}
		if definitions == 0 && len(projected) == 0 {
			task.EstimatedMinutes = cloneMinutes(task.ManualEstimatedMinutes)
			task.EstimateSource = "manual"
		} else {
			task.EstimateSource = "action_items"
			task.EstimatedMinutes = nil
			if hasEstimate {
				task.EstimatedMinutes = &total
			}
		}
		task.Progress = taskProgressPercent(task.Status.Value, dao.TaskProgressCounts{Total: task.ActionItemCount, Completed: task.ActionItemCompletedCount})
	}
	return tasks, nil
}

func cloneMinutes(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

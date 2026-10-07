package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	sharedprogress "github.com/Najah7/task2todaytodo/internal/application/shared/progress"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

func loadTaskProgress(ctx context.Context, source taskProgressSource, tasks []dao.Task, asOf time.Time) ([]dao.Task, map[string]dao.TaskProgressCounts, error) {
	if len(tasks) == 0 {
		return tasks, nil, nil
	}
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	sources, err := source.ReadTaskProgressSources(ctx, ids, asOf)
	if err != nil {
		return nil, nil, err
	}
	return applyTaskProgressSources(tasks, sources, asOf)
}

func applyTaskProgressSources(tasks []dao.Task, sources dao.TaskProgressSources, asOf time.Time) ([]dao.Task, map[string]dao.TaskProgressCounts, error) {
	counts := make(map[string]dao.TaskProgressCounts, len(tasks))
	for index, task := range tasks {
		if status, ok := sources.Statuses[task.ID]; ok {
			task.Status = status
		}
		value := sources.Counts[task.ID]
		if task.Status.Value != "done" {
			for _, root := range sources.TodoItemRoots {
				if root.TaskID == task.ID && progressTodoOccursToday(root, asOf) {
					value.Total++
				}
			}
		}
		counts[task.ID] = value
		task.Progress = taskProgressPercent(task.Status.Value, value)
		tasks[index] = task
	}
	return tasks, counts, nil
}

func taskProgressCounts(ctx context.Context, tasks TaskRepository, task dao.Task, asOf time.Time) (dao.Task, dao.TaskProgressCounts, error) {
	inputs, err := tasks.ReadTaskProgressSources(ctx, []string{task.ID}, asOf)
	if err != nil {
		return dao.Task{}, dao.TaskProgressCounts{}, err
	}
	if status, ok := inputs.Statuses[task.ID]; ok {
		task.Status = status
	}
	counts := inputs.Counts[task.ID]
	// For mutation comparisons, project a done task's current virtual set as if
	// it were reopened. This detects a restored skipped occurrence that becomes
	// visible only after done-task recurrence suppression ends.
	for _, root := range inputs.TodoItemRoots {
		if progressTodoOccursToday(root, asOf) {
			counts.Total++
		}
	}
	return task, counts, nil
}

func taskMutationCounts(
	ctx context.Context,
	repos Repositories,
	userID domain.UserID,
	taskID domain.TaskID,
	asOf time.Time,
	permission shared.Capability,
) (dao.Task, dao.TaskProgressCounts, error) {
	tasks := repos.Tasks()
	task, err := tasks.LockByUserIDWithPermission(ctx, userID, taskID, permission)
	if err != nil {
		return dao.Task{}, dao.TaskProgressCounts{}, err
	}
	return taskProgressCounts(ctx, tasks, task, asOf)
}

func finishTaskProgressMutation(
	ctx context.Context,
	repos Repositories,
	userID domain.UserID,
	taskID domain.TaskID,
	asOf time.Time,
	beforeTask dao.Task,
	before dao.TaskProgressCounts,
	permission shared.Capability,
) error {
	afterTask, after, err := taskMutationCounts(ctx, repos, userID, taskID, asOf, permission)
	if err != nil {
		return err
	}
	locking := repos.Tasks()
	if beforeTask.Status.Value == "done" || afterTask.Status.Value == "done" {
		if after.Total-after.Completed <= before.Total-before.Completed {
			return nil
		}
		open, err := domain.NewTaskStatus("open")
		if err != nil {
			return err
		}
		if err := setTaskStatusForPermission(ctx, locking, userID, taskID, open, afterTask.Revision, permission); err != nil {
			return err
		}
		return nil
	}
	if !progressCountsChanged(before, after) || after.Total == 0 || after.Completed != after.Total {
		return nil
	}
	done, err := domain.NewTaskStatus("done")
	if err != nil {
		return err
	}
	return setTaskStatusForPermission(ctx, locking, userID, taskID, done, afterTask.Revision, permission)
}

func withTaskProgressMutation(
	ctx context.Context,
	repos Repositories,
	userID domain.UserID,
	taskID domain.TaskID,
	asOf time.Time,
	mutate func() error,
) error {
	return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TaskUpdate(), mutate)
}

func withTaskProgressMutationForPermission(
	ctx context.Context,
	repos Repositories,
	userID domain.UserID,
	taskID domain.TaskID,
	asOf time.Time,
	permission shared.Capability,
	mutate func() error,
) error {
	task, before, err := taskMutationCounts(ctx, repos, userID, taskID, asOf, permission)
	if err != nil {
		return err
	}
	if err := mutate(); err != nil {
		return err
	}
	return finishTaskProgressMutation(ctx, repos, userID, taskID, asOf, task, before, permission)
}

func setTaskStatusForPermission(ctx context.Context, tasks TaskRepository, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, expectedRevision int32, permission shared.Capability) error {
	return tasks.SetStatusByUserIDWithPermission(ctx, userID, taskID, status, expectedRevision, permission)
}

func taskProgressPercent(status string, counts dao.TaskProgressCounts) int {
	return sharedprogress.TaskPercent(status == "done", counts.Completed, counts.Total)
}

func applyTaskProgress(ctx context.Context, source taskProgressSource, tasks []dao.Task, asOf time.Time) ([]dao.Task, error) {
	result, _, err := loadTaskProgress(ctx, source, tasks, asOf)
	return result, err
}

func progressTodoOccursToday(root dao.ProgressRecurrence, asOf time.Time) bool {
	anchorDate := ""
	if root.FrequencyAnchorDate != 0 {
		anchorDate = time.Unix(root.FrequencyAnchorDate, 0).UTC().Format("2006-01-02")
	}
	frequencies := make([]string, 0, len(root.Frequencies))
	for _, frequency := range root.Frequencies {
		frequencies = append(frequencies, frequency.Value)
	}
	eligible, err := sharedprogress.VirtualOccurrenceOccursToday(sharedprogress.RecurrenceRule{
		OccurrenceDate: root.OccurrenceDate, Timezone: root.Timezone, IntervalWeeks: root.IntervalWeeks,
		FrequencyAnchorDate: anchorDate, Frequencies: frequencies, OccurrenceSavedToday: root.OccurrenceSavedToday,
	}, asOf)
	return err == nil && eligible
}

func progressCountsChanged(before, after dao.TaskProgressCounts) bool {
	return before.Total != after.Total || before.Completed != after.Completed
}

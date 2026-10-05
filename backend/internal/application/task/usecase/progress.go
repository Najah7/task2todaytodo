package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	tasktime "github.com/Najah7/task2todaytodo/internal/application/task"
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
	sources, err := source.ReadTaskProgressSources(ctx, ids, nil, asOf)
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
			for _, root := range sources.TaskScheduleRoots {
				if root.TaskID == task.ID && progressScheduleOccursToday(root, asOf) {
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
	inputs, err := tasks.ReadTaskProgressSources(ctx, []string{task.ID}, nil, asOf)
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
	for _, root := range inputs.TaskScheduleRoots {
		if progressScheduleOccursToday(root, asOf) {
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
	if status == "done" {
		return 100
	}
	if counts.Total == 0 {
		return 0
	}
	return 100 * counts.Completed / counts.Total
}

func projectProgressPercent(tasks []dao.Task) int {
	if len(tasks) == 0 {
		return 0
	}
	total := 0
	for _, task := range tasks {
		total += task.Progress
	}
	return total / len(tasks)
}

func applyTaskProgress(ctx context.Context, source taskProgressSource, tasks []dao.Task, asOf time.Time) ([]dao.Task, error) {
	result, _, err := loadTaskProgress(ctx, source, tasks, asOf)
	return result, err
}

func applyProjectProgress(ctx context.Context, source taskProgressSource, projects []dao.Project, asOf time.Time) ([]dao.Project, error) {
	if len(projects) == 0 {
		return projects, nil
	}
	projectIDs := make([]string, 0, len(projects))
	for _, project := range projects {
		projectIDs = append(projectIDs, project.ID)
	}
	sources, err := source.ReadTaskProgressSources(ctx, nil, projectIDs, asOf)
	if err != nil {
		return nil, err
	}
	tasks := make([]dao.Task, 0, len(sources.ProjectTasks))
	for _, row := range sources.ProjectTasks {
		tasks = append(tasks, dao.Task{ID: row.ID, ProjectID: row.ProjectID, Status: row.Status})
	}
	tasks, _, err = applyTaskProgressSources(tasks, sources, asOf)
	if err != nil {
		return nil, err
	}
	progressByProject := make(map[string][]dao.Task, len(projects))
	for _, task := range tasks {
		progressByProject[task.ProjectID] = append(progressByProject[task.ProjectID], task)
	}
	for i := range projects {
		projects[i].Progress = projectProgressPercent(progressByProject[projects[i].ID])
	}
	return projects, nil
}

func applyProjectTaskProgress(ctx context.Context, source taskProgressSource, projectID string, tasks []dao.Task, asOf time.Time) ([]dao.Task, int, error) {
	tasks, err := applyTaskProgress(ctx, source, tasks, asOf)
	if err != nil {
		return nil, 0, err
	}
	for i := range tasks {
		tasks[i].ProjectID = projectID
	}
	return tasks, projectProgressPercent(tasks), nil
}

func progressTodoOccursToday(root dao.ProgressRecurrence, asOf time.Time) bool {
	if root.OccurrenceSavedToday || root.IntervalWeeks < 1 {
		return false
	}
	location, err := time.LoadLocation(root.Timezone)
	if err != nil {
		return false
	}
	first, err := parseOccurrenceDate(root.OccurrenceDate)
	if err != nil {
		return false
	}
	today := localToday(asOf, location)
	if today.Before(first) {
		return false
	}
	anchor := first
	if root.FrequencyAnchorDate != 0 {
		anchor = time.Unix(root.FrequencyAnchorDate, 0).UTC()
	}
	frequencies, err := taskFrequenciesFromDAO(root.Frequencies)
	if err != nil {
		return false
	}
	dates, err := domain.GenerateRecurrenceDatesFromAnchorLimit(anchor, today, root.IntervalWeeks, frequencies, root.Timezone, 1)
	return err == nil && len(dates) > 0 && dates[0].Date.Equal(today)
}

func progressScheduleOccursToday(root dao.ProgressRecurrence, asOf time.Time) bool {
	if root.OccurrenceSavedToday || root.IntervalWeeks < 1 {
		return false
	}
	location, err := time.LoadLocation(root.Timezone)
	if err != nil {
		return false
	}
	first, err := parseOccurrenceDate(root.OccurrenceDate)
	if err != nil {
		return false
	}
	today := localToday(asOf, location)
	if today.Before(first) {
		return false
	}
	anchor := first
	if root.FrequencyAnchorDate != 0 {
		anchor = time.Unix(root.FrequencyAnchorDate, 0).UTC()
	}
	frequencies, err := taskFrequenciesFromDAO(root.Frequencies)
	if err != nil {
		return false
	}
	dates, err := domain.GenerateRecurrenceDatesFromAnchorLimit(anchor, today, root.IntervalWeeks, frequencies, root.Timezone, 1)
	if err != nil || len(dates) == 0 || !dates[0].Date.Equal(today) {
		return false
	}
	startLocal, endLocal := time.Unix(root.StartAt, 0).In(location), time.Unix(root.EndAt, 0).In(location)
	if _, ok := tasktime.ResolveWallTime(today, startLocal, 0, location); !ok {
		return false
	}
	_, ok := tasktime.ResolveWallTime(today, endLocal, tasktime.CalendarDayOffset(startLocal, endLocal), location)
	return ok
}

func progressCountsChanged(before, after dao.TaskProgressCounts) bool {
	return before.Total != after.Total || before.Completed != after.Completed
}

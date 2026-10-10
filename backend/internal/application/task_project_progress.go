package application

import (
	"context"
	"time"

	projectdao "github.com/Najah7/task2todaytodo/internal/application/project/dao"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	taskdao "github.com/Najah7/task2todaytodo/internal/application/task/dao"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
)

// taskProjectProgressReader joins Project-owned schedule facts with Task-owned
// progress projection. It lives at the application boundary so neither
// context imports the other's implementation.
type taskProjectProgressReader struct {
	projects    projectusecase.ProjectProgressReader
	actionItems taskusecase.TaskListProjectionReader
}

func newTaskProjectProgressReader(projects projectusecase.ProjectProgressReader, actionItems taskusecase.TaskListProjectionReader) projectusecase.ProjectProgressReader {
	if actionItems == nil {
		return projects
	}
	return taskProjectProgressReader{projects: projects, actionItems: actionItems}
}

func (reader taskProjectProgressReader) ReadProjectProgressSources(ctx context.Context, projectIDs []string, asOf time.Time) (projectdao.ProjectProgressSources, error) {
	sources, err := reader.projects.ReadProjectProgressSources(ctx, projectIDs, asOf)
	if err != nil || len(sources.Tasks) == 0 {
		return sources, err
	}
	indexesByUser := make(map[string][]int)
	tasksByUser := make(map[string][]taskdao.Task)
	for index, row := range sources.Tasks {
		userID := row.UserID
		indexesByUser[userID] = append(indexesByUser[userID], index)
		tasksByUser[userID] = append(tasksByUser[userID], taskdao.Task{
			ID: row.TaskID, UserID: userID, ProjectID: row.ProjectID,
			Status: taskdao.TaskStatus{Value: progressStatus(row.Done)}, ProjectStatus: row.ProjectStatus,
		})
	}
	for userID, tasks := range tasksByUser {
		projection, err := reader.actionItems.ReadTaskListProjection(ctx, taskdomain.UserID(userID), taskIDs(tasks))
		if err != nil {
			return projectdao.ProjectProgressSources{}, err
		}
		enriched, err := taskusecase.ApplyTaskListProjection(tasks, projection, asOf)
		if err != nil {
			return projectdao.ProjectProgressSources{}, err
		}
		for index, task := range enriched {
			sources.Tasks[indexesByUser[userID][index]].Progress = task.Progress
		}
	}
	return sources, nil
}

func taskIDs(tasks []taskdao.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func progressStatus(done bool) string {
	if done {
		return "done"
	}
	return "open"
}

var _ projectusecase.ProjectProgressReader = taskProjectProgressReader{}

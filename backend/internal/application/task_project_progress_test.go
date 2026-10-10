package application

import (
	"context"
	"testing"
	"time"

	projectdao "github.com/Najah7/task2todaytodo/internal/application/project/dao"
	taskdao "github.com/Najah7/task2todaytodo/internal/application/task/dao"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type projectProgressReaderFake struct {
	sources projectdao.ProjectProgressSources
}

func (fake projectProgressReaderFake) ReadProjectProgressSources(context.Context, []string, time.Time) (projectdao.ProjectProgressSources, error) {
	return fake.sources, nil
}

type taskProjectionReaderFake struct {
	sources taskdao.TaskListProjectionSources
	userID  taskdomain.UserID
	taskIDs []string
}

func (fake *taskProjectionReaderFake) ReadTaskListProjection(_ context.Context, userID taskdomain.UserID, taskIDs []string) (taskdao.TaskListProjectionSources, error) {
	fake.userID, fake.taskIDs = userID, append([]string(nil), taskIDs...)
	return fake.sources, nil
}

func TestTaskProjectProgressReaderUsesTaskListProjectionAndUserScope(t *testing.T) {
	asOf := time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)
	reader := &taskProjectionReaderFake{sources: taskdao.TaskListProjectionSources{
		ActionItemsByTask: map[string][]taskdao.ActionItem{
			"task-1": {
				{ID: "item-done", TaskID: "task-1", SeriesID: "item-done", OccurrenceDate: "2026-10-10", Timezone: "Asia/Tokyo", RepeatState: "one_off", Completed: true},
				{ID: "item-open", TaskID: "task-1", SeriesID: "item-open", OccurrenceDate: "2026-10-10", Timezone: "Asia/Tokyo", RepeatState: "one_off"},
			},
		},
		SkippedByTask: map[string]map[string]map[string]bool{},
	}}
	base := projectProgressReaderFake{sources: projectdao.ProjectProgressSources{Tasks: []projectdao.ProjectTaskProgress{{
		ProjectID: "project-1", TaskID: "task-1", UserID: "owner-1", Done: false,
	}}}}

	sources, err := newTaskProjectProgressReader(base, reader).ReadProjectProgressSources(context.Background(), []string{"project-1"}, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if sources.Tasks[0].Progress != 50 {
		t.Fatalf("Task progress = %d, want 50 from one completed of two list rows", sources.Tasks[0].Progress)
	}
	if reader.userID != "owner-1" || len(reader.taskIDs) != 1 || reader.taskIDs[0] != "task-1" {
		t.Fatalf("projection scope = user %q tasks %v, want owner-1/task-1", reader.userID, reader.taskIDs)
	}
}

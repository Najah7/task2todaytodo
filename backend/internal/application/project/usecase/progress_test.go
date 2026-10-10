package usecase

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
)

func TestApplyProjectProgressUsesProjectFactsForWholeTaskAndScheduleOccurrenceWeights(t *testing.T) {
	projects := []dao.Project{{ID: "mixed"}, {ID: "schedule-only"}, {ID: "empty"}}
	reader := &projectProgressSourceFake{sources: dao.ProjectProgressSources{
		Tasks: []dao.ProjectTaskProgress{
			{ProjectID: "mixed", TaskID: "task-partial", Progress: 33},
			{ProjectID: "mixed", TaskID: "task-done", Done: true, Progress: 100},
		},
		Schedules: []dao.ProjectScheduleProgress{
			{ProjectID: "mixed", Total: 2, Completed: 1},
			{ProjectID: "schedule-only", Total: 4, Completed: 2},
		},
	}}
	got, err := applyProjectProgress(context.Background(), reader, projects, time.Date(2026, time.October, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{58, 50, 0}; !reflect.DeepEqual([]int{got[0].Progress, got[1].Progress, got[2].Progress}, want) {
		t.Fatalf("project progresses = %d, %d, %d; want %v", got[0].Progress, got[1].Progress, got[2].Progress, want)
	}
	if reader.calls != 1 || !reflect.DeepEqual(reader.ids, []string{"mixed", "schedule-only", "empty"}) {
		t.Fatalf("progress reads = %d for projects %v; want one read of all projects", reader.calls, reader.ids)
	}
}

func TestApplyProjectProgressDoesNotReadSourcesForEmptyBatch(t *testing.T) {
	reader := &projectProgressSourceFake{}
	got, err := applyProjectProgress(context.Background(), reader, nil, time.Now())
	if err != nil || len(got) != 0 || reader.calls != 0 {
		t.Fatalf("empty project progress = %#v, error %v, reads %d; want empty, nil, 0", got, err, reader.calls)
	}
}

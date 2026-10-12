package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type taskListCapabilitiesReader struct{ sources dao.TaskListProjectionSources }

func (reader taskListCapabilitiesReader) ReadTaskListProjection(context.Context, domain.UserID, []string) (dao.TaskListProjectionSources, error) {
	return reader.sources, nil
}

func TestEnrichTaskListUsesUnfilteredChildAndCreateCapabilityProjections(t *testing.T) {
	tasks := []dao.Task{
		{ID: "childless", UserID: "actor", AssigneeID: "actor", CanUpdate: true, Status: dao.TaskStatus{Value: "open"}},
		{ID: "hidden-child", UserID: "actor", AssigneeID: "actor", CanUpdate: true, Status: dao.TaskStatus{Value: "open"}},
		{ID: "no-create", UserID: "actor", AssigneeID: "actor", CanUpdate: true, Status: dao.TaskStatus{Value: "open"}},
	}
	reader := taskListCapabilitiesReader{sources: dao.TaskListProjectionSources{
		ActionItemsByTask:    map[string][]dao.ActionItem{"childless": nil, "hidden-child": nil, "no-create": nil},
		TasksWithChildren:    map[string]bool{"hidden-child": true},
		CanCreateActionItems: map[string]bool{"childless": true, "hidden-child": true},
	}}
	got, err := EnrichTaskList(context.Background(), reader, "actor", tasks, time.Now())
	if err != nil {
		t.Fatalf("EnrichTaskList() error = %v", err)
	}
	if !got[0].CanAddToToday || !got[0].CanAddActionItem {
		t.Errorf("childless capabilities = add_today:%t add_action:%t; want both", got[0].CanAddToToday, got[0].CanAddActionItem)
	}
	if got[1].CanAddToToday || !got[1].CanAddActionItem {
		t.Errorf("hidden-child capabilities = add_today:%t add_action:%t; want no direct Task row and allowed child add", got[1].CanAddToToday, got[1].CanAddActionItem)
	}
	if got[2].CanAddActionItem {
		t.Error("CanAddActionItem = true without ActionItem-create capability")
	}
}

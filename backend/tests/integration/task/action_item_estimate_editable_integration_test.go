//go:build integration

package task_test

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	taskdao "github.com/Najah7/task2todaytodo/internal/application/task/dao"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/oklog/ulid/v2"
)

func TestActionItemEstimateEditableAcrossReadAndWriteProjections(t *testing.T) {
	pool := integrationPool(t)
	userID, taskID := seedTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	repo := taskrepo.NewActionItemRepository(pool)

	create := func(title string) (taskdomain.ActionItemID, int, bool) {
		t.Helper()
		id := taskdomain.ActionItemID(ulid.Make().String())
		estimate := 30
		priority, err := taskdomain.NewTaskPriority("low")
		if err != nil {
			t.Fatal(err)
		}
		item, err := taskdomain.NewActionItemWithDetails(id, taskID, title, "", time.Time{}, false, 0,
			taskdomain.ActionItemPlanning{EstimatedMinutes: &estimate, Priority: priority})
		if err != nil {
			t.Fatal(err)
		}
		created, err := repo.CreateForOwnedTask(ctx, userID, item, true)
		if err != nil {
			t.Fatalf("create ActionItem: %v", err)
		}
		if created.EstimatedMinutes == nil {
			t.Fatal("created ActionItem has no estimate")
		}
		return id, *created.EstimatedMinutes, created.EstimateEditable
	}

	assertProjections := func(id taskdomain.ActionItemID, want bool) {
		t.Helper()
		get, err := repo.GetForOwnedTask(ctx, userID, taskID, id)
		if err != nil {
			t.Fatalf("get ActionItem %s: %v", id, err)
		}
		assertEditable(t, "GetForOwnedTask", get.EstimateEditable, want)
		command, err := repo.GetForCommand(ctx, userID, taskID, id, shared.ActionItemUpdate())
		if err != nil {
			t.Fatalf("get ActionItem for command %s: %v", id, err)
		}
		assertEditable(t, "GetForCommand", command.EstimateEditable, want)

		list, err := repo.ListByTask(ctx, userID, taskID)
		if err != nil {
			t.Fatalf("list ActionItems: %v", err)
		}
		assertEditable(t, "ListByTask", findEstimateEditable(t, list, id), want)

		projection, err := repo.ReadTaskListProjection(ctx, userID, []string{string(taskID)})
		if err != nil {
			t.Fatalf("read task list projection: %v", err)
		}
		assertEditable(t, "ReadTaskListProjection", findEstimateEditable(t, projection.ActionItemsByTask[string(taskID)], id), want)

		cursor, err := repo.ListByTaskCursor(ctx, userID, taskID, 20, nil)
		if err != nil {
			t.Fatalf("list ActionItems by cursor: %v", err)
		}
		assertEditable(t, "ListByTaskCursor", findEstimateEditable(t, cursor, id), want)
	}

	completedSourceID, oldEstimate, createdEditable := create("Source completed")
	assertEditable(t, "CreateForOwnedTask", createdEditable, true)
	completedFrameID, _, frameCreatedEditable := create("Frame completed")
	assertEditable(t, "CreateForOwnedTask before frame completion", frameCreatedEditable, true)
	newEstimate := oldEstimate + 5
	priority, _ := taskdomain.NewTaskPriority("low")
	updatedItem, err := taskdomain.NewActionItemWithDetails(completedSourceID, taskID, "Source completed", "", time.Time{}, false, 0,
		taskdomain.ActionItemPlanning{EstimatedMinutes: &newEstimate, Priority: priority})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := repo.UpdateForOwnedTask(ctx, userID, updatedItem)
	if err != nil {
		t.Fatalf("update unstarted ActionItem: %v", err)
	}
	assertEditable(t, "UpdateForOwnedTask", updated.EstimateEditable, true)
	assertProjections(completedSourceID, true)
	if err := taskusecase.NewCompleteActionItemUseCase(actionItemTestUOW{pool: pool}, nil).Execute(ctx, userID, taskID, completedSourceID); err != nil {
		t.Fatalf("complete ActionItem source: %v", err)
	}
	assertProjections(completedSourceID, false)
	assertProjections(completedFrameID, true)
	todoID := ulid.Make().String()
	frameID := ulid.Make().String()
	if _, err := pool.Exec(ctx, `INSERT INTO todos(id,user_id,todo_date,timezone) VALUES($1,$2,(now() AT TIME ZONE 'Asia/Tokyo')::date,'Asia/Tokyo')`, todoID, userID); err != nil {
		t.Fatalf("insert Todo: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO todo_items(id,todo_id,kind,action_item_id,planned_minutes,completed,position) VALUES($1,$2,'action_item',$3,30,true,0)`, frameID, todoID, completedFrameID); err != nil {
		t.Fatalf("insert completed Todo frame: %v", err)
	}
	assertProjections(completedFrameID, false)
}

func findEstimateEditable(t *testing.T, items []taskdao.ActionItem, id taskdomain.ActionItemID) bool {
	t.Helper()
	for _, item := range items {
		if item.ID == string(id) {
			return item.EstimateEditable
		}
	}
	t.Fatalf("ActionItem %s is absent from projection", id)
	return false
}

func assertEditable(t *testing.T, projection string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s estimate_editable = %v, want %v", projection, got, want)
	}
}

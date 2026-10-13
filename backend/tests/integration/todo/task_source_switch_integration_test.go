//go:build integration

package todo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	restapi "github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/go-chi/chi/v5"
	"github.com/oklog/ulid/v2"
)

func mountTaskSourceMutationRoutes(t *testing.T, fixture *todoFixture) {
	t.Helper()
	ids := adapter.NewIDIssuer()
	store := application.NewStore(fixture.pool)
	uow := application.NewUOW(fixture.pool, store.Auth, store.Project, store.Task, store.Schedule, store.Allocation)
	usecases, closeUseCases := application.NewUseCase(store, uow, ids, telemetry.Nop())
	testdb.RegisterShutdown(t, closeUseCases)
	tasks := restapi.NewTaskHandler(usecases.Task, ids.NewULID)
	actionItems := restapi.NewActionItemHandler(usecases.ActionItem, ids.NewULID)
	router, ok := fixture.router.(*chi.Mux)
	if !ok {
		t.Fatalf("Todo fixture router has type %T; want *chi.Mux", fixture.router)
	}
	router.Patch("/tasks/{id}", tasks.Update)
	router.Post("/tasks/{taskId}/action-items", actionItems.Create)
	router.Delete("/tasks/{taskId}/action-items/{id}", actionItems.Delete)
}

func TestTaskManualEstimateLocksAfterDone(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
	mountTaskSourceMutationRoutes(t, fixture)
	doneTaskID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'Done manual estimate',30,'high','done',$2)`, doneTaskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, doneTaskID)
	})
	revision := taskRevision(t, fixture, doneTaskID)
	response := fixture.request(t, http.MethodPatch, "/tasks/"+doneTaskID, `{"manual_estimated_minutes":45}`, revision)
	if response.Code != http.StatusConflict {
		t.Fatalf("PATCH manual estimate for done Task status/body=%d/%s; want 409", response.Code, response.Body.String())
	}
	var estimate int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT manual_estimated_minutes FROM tasks WHERE id=$1`, doneTaskID).Scan(&estimate); err != nil {
		t.Fatal(err)
	}
	if estimate != 30 {
		t.Fatalf("done Task estimate changed to %d; want 30", estimate)
	}
}
func taskRevision(t *testing.T, fixture *todoFixture, taskID string) int32 {
	t.Helper()
	var revision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT revision FROM tasks WHERE id=$1`, taskID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	return revision
}

func TestFirstActionItemCreationDoesNotRebuildExistingTodo(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
	mountTaskSourceMutationRoutes(t, fixture)
	taskID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'First child source',60,'high','open',$2)`, taskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, taskID)
	})
	initialResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("GET Todo status/body=%d/%s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decodeToday(t, initialResponse)
	var taskFrameID string
	for _, item := range initial.Items {
		if item.Kind == "task" && item.TaskID == taskID {
			taskFrameID = item.ID
		}
	}
	if taskFrameID == "" {
		t.Fatalf("childless Task absent from initial Todo: %+v", initial.Items)
	}
	created := fixture.request(t, http.MethodPost, "/tasks/"+taskID+"/action-items", `{"title":"First child","estimated_minutes":30}`, 0)
	if created.Code != http.StatusCreated {
		t.Fatalf("first-child create status/body=%d/%s", created.Code, created.Body.String())
	}
	var todoRevision int32
	var persistedTaskFrames int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT revision FROM todos WHERE id=$1`, initial.ID).Scan(&todoRevision); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM todo_items WHERE todo_id=$1 AND kind='task' AND task_id=$2`, initial.ID, taskID).Scan(&persistedTaskFrames); err != nil {
		t.Fatal(err)
	}
	if todoRevision != initial.Revision || persistedTaskFrames != 1 {
		t.Fatalf("source change repaired existing Todo: revision=%d/%d saved Task rows=%d", todoRevision, initial.Revision, persistedTaskFrames)
	}
	reloaded := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if reloaded.Code != http.StatusOK {
		t.Fatalf("GET after first ActionItem creation status/body=%d/%s", reloaded.Code, reloaded.Body.String())
	}
	items := decodeToday(t, reloaded).Items
	foundTask, foundNewAction := false, false
	for _, item := range items {
		if item.ID == taskFrameID && item.Kind == "task" && item.TaskID == taskID {
			foundTask = true
		}
		if item.Kind == "action_item" && item.TaskID == taskID {
			foundNewAction = true
		}
	}
	if !foundTask || foundNewAction {
		t.Fatalf("first ActionItem creation replaced saved direct Task or exposed the new child: %+v", items)
	}
}
func TestLastActionItemDeleteDoesNotRebuildExistingTodo(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
	mountTaskSourceMutationRoutes(t, fixture)
	taskID, actionItemID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,priority,status,changed_by) VALUES($1,$2,$2,'Last child source','high','open',$2)`, taskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO action_items(id,task_id,title,position,estimated_minutes,priority) VALUES($1,$2,'Only child',0,30,'high')`, actionItemID, taskID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, taskID)
	})
	initialResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("GET Todo status/body=%d/%s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decodeToday(t, initialResponse)
	var frame restapi.TodoItemResponse
	for _, item := range initial.Items {
		if item.Kind == "action_item" && item.ActionItemID == actionItemID {
			frame = item
		}
	}
	if frame.ID == "" {
		t.Fatalf("ActionItem absent from initial Todo: %+v", initial.Items)
	}
	var originalStart, originalEnd sql.NullTime
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, frame.ID).Scan(&originalStart, &originalEnd); err != nil {
		t.Fatal(err)
	}
	if !originalStart.Valid || !originalEnd.Valid {
		t.Fatalf("generated ActionItem lacks internal schedule: %+v/%+v", originalStart, originalEnd)
	}
	taskRevision := taskRevision(t, fixture, taskID)
	deleted := fixture.request(t, http.MethodDelete, "/tasks/"+taskID+"/action-items/"+actionItemID, `{"manual_estimated_minutes":45}`, taskRevision)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete last ActionItem status/body=%d/%s", deleted.Code, deleted.Body.String())
	}
	var sourceDeleted bool
	var manualEstimate sql.NullInt32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT deleted_at IS NOT NULL FROM action_items WHERE id=$1`, actionItemID).Scan(&sourceDeleted); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT manual_estimated_minutes FROM tasks WHERE id=$1`, taskID).Scan(&manualEstimate); err != nil {
		t.Fatal(err)
	}
	var todoRevision int32
	var savedActionFrames int
	var savedStart, savedEnd time.Time
	if err := fixture.pool.QueryRow(t.Context(), `SELECT l.revision FROM todos l WHERE l.id=$1`, initial.ID).Scan(&todoRevision); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM todo_items WHERE id=$1 AND action_item_id=$2`, frame.ID, actionItemID).Scan(&savedActionFrames); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, frame.ID).Scan(&savedStart, &savedEnd); err != nil {
		t.Fatal(err)
	}
	if !sourceDeleted || !manualEstimate.Valid || manualEstimate.Int32 != 45 || todoRevision != initial.Revision || savedActionFrames != 1 || !savedStart.Equal(originalStart.Time) || !savedEnd.Equal(originalEnd.Time) {
		t.Fatalf("source delete rebuilt Todo: deleted=%t estimate=%+v revision=%d/%d frames=%d times=%s-%s original=%s-%s", sourceDeleted, manualEstimate, todoRevision, initial.Revision, savedActionFrames, savedStart, savedEnd, originalStart.Time, originalEnd.Time)
	}
}

func TestTodoHidesUnavailableWorkAndCanReorderRemainingItems(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 30)
	enableInitialActionItemGeneration(t, fixture)
	mountTaskSourceMutationRoutes(t, fixture)
	directTaskID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'Visible direct Task',15,'urgent','open',$2)`, directTaskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, directTaskID)
	})

	initialResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial GET Todo status/body=%d/%s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decodeToday(t, initialResponse)
	var directRow, actionRow restapi.TodoItemResponse
	for _, item := range initial.Items {
		if item.Kind == "task" && item.TaskID == directTaskID {
			directRow = item
		}
		if item.Kind == "action_item" && item.ActionItemID == fixture.itemID {
			actionRow = item
		}
	}
	if directRow.ID == "" || actionRow.ID == "" {
		t.Fatalf("initial Todo lacks both work sources: %+v", initial.Items)
	}
	var originalStart, originalEnd sql.NullTime
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, directRow.ID).Scan(&originalStart, &originalEnd); err != nil {
		t.Fatal(err)
	}
	if !originalStart.Valid || !originalEnd.Valid {
		t.Fatalf("direct work did not retain internal times: %+v/%+v", originalStart, originalEnd)
	}

	deleted := fixture.request(t, http.MethodDelete, "/tasks/"+fixture.taskID+"/action-items/"+fixture.itemID, `{"manual_estimated_minutes":30}`, taskRevision(t, fixture, fixture.taskID))
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete saved ActionItem source status/body=%d/%s", deleted.Code, deleted.Body.String())
	}
	reloaded := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if reloaded.Code != http.StatusOK {
		t.Fatalf("GET Todo after source becomes unavailable status/body=%d/%s", reloaded.Code, reloaded.Body.String())
	}
	afterDelete := decodeToday(t, reloaded)
	if len(afterDelete.Items) != 1 || afterDelete.Items[0].ID != directRow.ID || afterDelete.Items[0].ScheduledStart != nil || afterDelete.Items[0].ScheduledEnd != nil {
		t.Fatalf("Todo should hide deleted ActionItem and retain visible Task: %+v", afterDelete.Items)
	}
	body, err := json.Marshal(struct {
		ItemIDs []string `json:"item_ids"`
	}{ItemIDs: []string{directRow.ID}})
	if err != nil {
		t.Fatal(err)
	}
	reordered := fixture.request(t, http.MethodPost, "/today-todo:reorder", string(body), afterDelete.Revision)
	if reordered.Code != http.StatusOK {
		t.Fatalf("reorder remaining visible work status/body=%d/%s", reordered.Code, reordered.Body.String())
	}
	afterReorder := decodeToday(t, reordered)
	if len(afterReorder.Items) != 1 || afterReorder.Items[0].ID != directRow.ID {
		t.Fatalf("reorder regenerated or exposed unavailable work: %+v", afterReorder.Items)
	}
	var currentStart, currentEnd sql.NullTime
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, directRow.ID).Scan(&currentStart, &currentEnd); err != nil {
		t.Fatal(err)
	}
	if !currentStart.Valid || !currentEnd.Valid || !currentStart.Time.Equal(originalStart.Time) || !currentEnd.Time.Equal(originalEnd.Time) {
		t.Fatalf("reorder changed remaining work internal times: before=%+v/%+v after=%+v/%+v", originalStart, originalEnd, currentStart, currentEnd)
	}
}

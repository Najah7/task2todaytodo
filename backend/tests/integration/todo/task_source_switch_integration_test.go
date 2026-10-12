//go:build integration

package todo_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
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

func TestTaskManualEstimateLocksAfterDoneOrCompletedTaskHistory(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
	mountTaskSourceMutationRoutes(t, fixture)
	doneTaskID, startedTaskID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'Done manual estimate',30,'high','done',$2),($3,$2,$2,'Started manual estimate',30,'high','open',$2)`, doneTaskID, fixture.userID, startedTaskID); err != nil {
		t.Fatal(err)
	}
	previousDate := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	todoID, frameID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todos(id,user_id,todo_date,timezone) VALUES($1,$2,$3,'UTC')`, todoID, fixture.userID, previousDate); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todo_items(id,todo_id,kind,task_id,planned_minutes,completed,position) VALUES($1,$2,'task',$3,10,true,0)`, frameID, todoID, startedTaskID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=ANY($1::text[])`, []string{doneTaskID, startedTaskID})
	})
	for _, taskID := range []string{doneTaskID, startedTaskID} {
		revision := taskRevision(t, fixture, taskID)
		response := fixture.request(t, http.MethodPatch, "/tasks/"+taskID, `{"manual_estimated_minutes":45}`, revision)
		if response.Code != http.StatusConflict {
			t.Errorf("PATCH manual estimate for done/started Task %s status/body=%d/%s; want 409", taskID, response.Code, response.Body.String())
		}
		var estimate int
		if err := fixture.pool.QueryRow(t.Context(), `SELECT manual_estimated_minutes FROM tasks WHERE id=$1`, taskID).Scan(&estimate); err != nil {
			t.Fatal(err)
		}
		if estimate != 30 {
			t.Errorf("locked Task %s estimate changed to %d; want 30", taskID, estimate)
		}
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

func TestFirstActionItemRequiresConfirmationBeforeReplacingTodayTask(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
	mountTaskSourceMutationRoutes(t, fixture)
	taskID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'First child confirmation',60,'high','open',$2)`, taskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, taskID)
	})

	initialResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("GET Today status/body=%d/%s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decodeToday(t, initialResponse)
	var directFrameID string
	for _, item := range initial.Items {
		if item.Kind == "task" && item.TaskID == taskID {
			directFrameID = item.ID
		}
	}
	if directFrameID == "" {
		t.Fatalf("childless Task was not in initial Today plan: %+v", initial.Items)
	}
	createPath := fmt.Sprintf("/tasks/%s/action-items", taskID)
	createBody := `{"title":"First child","estimated_minutes":30}`
	unconfirmed := fixture.request(t, http.MethodPost, createPath, createBody, 0)
	if unconfirmed.Code != http.StatusConflict || !strings.Contains(unconfirmed.Body.String(), "todo_regeneration_required") {
		t.Fatalf("unconfirmed first-child create status/body=%d/%s; want regeneration confirmation 409", unconfirmed.Code, unconfirmed.Body.String())
	}
	var children int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM action_items WHERE task_id=$1 AND deleted_at IS NULL`, taskID).Scan(&children); err != nil {
		t.Fatal(err)
	}
	var revision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT revision FROM todos WHERE id=$1`, initial.ID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if children != 0 || revision != initial.Revision {
		t.Fatalf("cancelled first-child create mutated source/todo: children=%d listRevision=%d initial=%d", children, revision, initial.Revision)
	}
	unchanged := decodeToday(t, fixture.request(t, http.MethodGet, "/today-todo", "", 0))
	if itemByID(unchanged.Items, directFrameID).Kind != "task" {
		t.Fatalf("cancelled first-child create replaced direct Task frame: %+v", unchanged.Items)
	}

	confirmed := fixture.request(t, http.MethodPost, createPath, `{"title":"First child","estimated_minutes":30,"confirm_todo_regeneration":true}`, 0)
	if confirmed.Code != http.StatusCreated {
		t.Fatalf("confirmed first-child create status/body=%d/%s", confirmed.Code, confirmed.Body.String())
	}
	var newRevision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT revision FROM todos WHERE id=$1`, initial.ID).Scan(&newRevision); err != nil {
		t.Fatal(err)
	}
	if newRevision <= initial.Revision {
		t.Fatalf("confirmed first-child create did not regenerate the todo: revision %d -> %d", initial.Revision, newRevision)
	}
	updated := decodeToday(t, fixture.request(t, http.MethodGet, "/today-todo", "", 0))
	var actionFrameFound bool
	for _, item := range updated.Items {
		if item.Kind == "task" && item.TaskID == taskID {
			t.Fatalf("confirmed first-child create left a direct Task row: %+v", updated.Items)
		}
		if item.Kind == "action_item" && item.TaskID == taskID && item.PlannedMinutes != nil && *item.PlannedMinutes == 30 {
			actionFrameFound = true
		}
	}
	if !actionFrameFound {
		t.Fatalf("confirmed first-child create did not replace the Task row with its 30m ActionItem frame: %+v", updated.Items)
	}
}

func TestLastActionItemDeleteRequiresRevisionAndAtomicallyRestoresTaskSource(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
	mountTaskSourceMutationRoutes(t, fixture)
	taskID, actionItemID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,priority,status,changed_by) VALUES($1,$2,$2,'Last child fallback','high','open',$2)`, taskID, fixture.userID); err != nil {
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
		t.Fatalf("GET Today status/body=%d/%s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decodeToday(t, initialResponse)
	var sourceFrameFound bool
	for _, item := range initial.Items {
		if item.Kind == "action_item" && item.ActionItemID == actionItemID {
			sourceFrameFound = true
		}
	}
	if !sourceFrameFound {
		t.Fatalf("only child was not in initial Today plan: %+v", initial.Items)
	}
	deletePath := fmt.Sprintf("/tasks/%s/action-items/%s", taskID, actionItemID)
	body := `{"manual_estimated_minutes":45}`
	missingRevision := fixture.request(t, http.MethodDelete, deletePath, body, 0)
	if missingRevision.Code != http.StatusPreconditionRequired {
		t.Fatalf("last-child delete without Task If-Match status/body=%d/%s; want 428", missingRevision.Code, missingRevision.Body.String())
	}
	currentTaskRevision := taskRevision(t, fixture, taskID)
	staleRevision := fixture.request(t, http.MethodDelete, deletePath, body, currentTaskRevision+1)
	if staleRevision.Code != http.StatusPreconditionFailed {
		t.Fatalf("last-child delete with stale Task If-Match status/body=%d/%s; want 412", staleRevision.Code, staleRevision.Body.String())
	}
	currentTaskRevision = taskRevision(t, fixture, taskID)
	unconfirmed := fixture.request(t, http.MethodDelete, deletePath, body, currentTaskRevision)
	if unconfirmed.Code != http.StatusConflict || !strings.Contains(unconfirmed.Body.String(), "todo_regeneration_required") {
		t.Fatalf("unconfirmed last-child delete status/body=%d/%s; want regeneration confirmation 409", unconfirmed.Code, unconfirmed.Body.String())
	}
	var deleted bool
	var manual sql.NullInt32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT deleted_at IS NOT NULL FROM action_items WHERE id=$1`, actionItemID).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT manual_estimated_minutes FROM tasks WHERE id=$1`, taskID).Scan(&manual); err != nil {
		t.Fatal(err)
	}
	var listRevision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT revision FROM todos WHERE id=$1`, initial.ID).Scan(&listRevision); err != nil {
		t.Fatal(err)
	}
	if deleted || manual.Valid || listRevision != initial.Revision {
		t.Fatalf("cancelled last-child delete mutated source/todo: deleted=%t manual=%+v listRevision=%d initial=%d", deleted, manual, listRevision, initial.Revision)
	}

	confirmed := fixture.request(t, http.MethodDelete, deletePath, `{"manual_estimated_minutes":45,"confirm_todo_regeneration":true}`, currentTaskRevision)
	if confirmed.Code != http.StatusNoContent {
		t.Fatalf("confirmed last-child delete status/body=%d/%s", confirmed.Code, confirmed.Body.String())
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT deleted_at IS NOT NULL FROM action_items WHERE id=$1`, actionItemID).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT manual_estimated_minutes FROM tasks WHERE id=$1`, taskID).Scan(&manual); err != nil {
		t.Fatal(err)
	}
	if !deleted || !manual.Valid || manual.Int32 != 45 {
		t.Fatalf("confirmed last-child delete did not atomically persist source fallback: deleted=%t manual=%+v", deleted, manual)
	}
	var newListRevision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT revision FROM todos WHERE id=$1`, initial.ID).Scan(&newListRevision); err != nil {
		t.Fatal(err)
	}
	if newListRevision <= initial.Revision {
		t.Fatalf("confirmed last-child delete did not regenerate todo: revision %d -> %d", initial.Revision, newListRevision)
	}
	updated := decodeToday(t, fixture.request(t, http.MethodGet, "/today-todo", "", 0))
	var taskFrameFound bool
	for _, item := range updated.Items {
		if item.Kind == "task" && item.TaskID == taskID {
			if item.PlannedMinutes == nil || *item.PlannedMinutes != 45 {
				t.Fatalf("restored Task frame planned minutes=%v; want 45", item.PlannedMinutes)
			}
			taskFrameFound = true
		}
		if item.Kind == "action_item" && item.ActionItemID == actionItemID {
			t.Fatalf("confirmed last-child delete left a stale ActionItem row: %+v", updated.Items)
		}
	}
	if !taskFrameFound {
		t.Fatalf("confirmed last-child delete did not restore direct Task plan row: %+v", updated.Items)
	}
}

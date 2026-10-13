//go:build integration

package todo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	restapi "github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type todoFixture struct {
	pool   *pgxpool.Pool
	userID string
	taskID string
	itemID string
	router http.Handler
}

func newTodoFixture(t *testing.T, timezone string, estimate int) *todoFixture {
	t.Helper()
	pool := testdb.Open(t)
	userID, taskID, itemID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	ctx := t.Context()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone) VALUES($1,'Todo','Flow',$2,'unused',now(),$3)`, userID, userID+"@todo-flow.test", timezone); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,assignee_id,title,priority,status,changed_by) VALUES($1,$2,$2,'Todo integration','low','pending',$2)`, taskID, userID); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO action_items(id,task_id,title,position,estimated_minutes,priority) VALUES($1,$2,'Manual action',0,$3,'high')`, itemID, taskID, estimate); err != nil {
		t.Fatalf("insert ActionItem: %v", err)
	}
	store := application.NewStore(pool)
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule, store.Allocation)
	usecases, closeUseCases := application.NewUseCase(store, uow, adapter.NewIDIssuer(), telemetry.Nop())
	testdb.RegisterShutdown(t, closeUseCases)
	handlers, err := restapi.NewHandler(usecases, adapter.NewIDIssuer().NewULID, []byte(strings.Repeat("t", 32)))
	if err != nil {
		t.Fatalf("rest.NewHandler(): %v", err)
	}
	router := chi.NewRouter()
	router.Get("/today-todo", handlers.Todos.GetToday)
	router.Post("/today-todo:regenerate", handlers.Todos.RegenerateToday)
	router.Post("/today-todo/action-items/{actionItemId}", handlers.Todos.AddActionItem)
	router.Post("/today-todo/tasks/{taskId}", handlers.Todos.AddTask)
	router.Post("/today-todo:reorder", handlers.Todos.Reorder)
	router.Post("/today-todo/items/{itemId}:remove", handlers.Todos.RemoveActionItem)
	router.Post("/today-todo/items/{itemId}:complete", handlers.Todos.CompleteFrame)
	router.Post("/today-todo/items/{itemId}:reopen", handlers.Todos.ReopenFrame)
	router.Post("/today-todo/schedules/{itemId}:hide", handlers.Todos.HideSchedule)
	router.Post("/today-todo/schedules/{itemId}:restore", handlers.Todos.RestoreSchedule)
	router.Post("/schedule-rules", handlers.Schedule.CreateRule)
	router.Get("/schedules/{rule_id}/{occurrence_date}", handlers.Schedule.GetOccurrence)
	router.Patch("/schedules/{rule_id}/{occurrence_date}", handlers.Schedule.UpdateOccurrence)
	router.Post("/schedules/{rule_id}/{occurrence_date}:skip", handlers.Schedule.SkipOccurrence)
	router.Post("/schedules/{rule_id}/{occurrence_date}:restore", handlers.Schedule.RestoreOccurrence)
	router.Delete("/schedules/{rule_id}/{occurrence_date}", handlers.Schedule.DeleteOccurrence)
	router.Patch("/tasks/{taskId}/action-items/{id}", handlers.ActionItem.Update)
	router.Post("/tasks/{taskId}/action-items/{id}:complete", handlers.ActionItem.Complete)
	router.Post("/tasks/{taskId}/action-items/{id}:reopen", handlers.ActionItem.Reopen)
	router.Post("/tasks/{id}:reopen", handlers.Task.Reopen)
	router.Post("/tasks", handlers.Task.Create)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	return &todoFixture{pool: pool, userID: userID, taskID: taskID, itemID: itemID, router: router}
}

func enableInitialActionItemGeneration(t *testing.T, fixture *todoFixture) {
	t.Helper()
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE tasks SET status='open' WHERE id=$1`, fixture.taskID); err != nil {
		t.Fatalf("make fixture ActionItem eligible for initial generation: %v", err)
	}
}

func TestTodoDirectTaskAddCompleteReopenAndChildExclusion(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE users SET active_start_time=TIME '00:00',active_end_time=TIME '00:15' WHERE id=$1`, fixture.userID); err != nil {
		t.Fatal(err)
	}
	directTaskID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'Manual pending Task',60,'urgent','pending',$2)`, directTaskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	initial := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initial.Code != http.StatusOK {
		t.Fatalf("initial GET=%d %s", initial.Code, initial.Body.String())
	}
	today := decodeToday(t, initial)
	added := fixture.request(t, http.MethodPost, "/today-todo/tasks/"+directTaskID, "", today.Revision)
	if added.Code != http.StatusOK {
		t.Fatalf("AddTask status/body=%d/%s", added.Code, added.Body.String())
	}
	today = decodeToday(t, added)
	var taskFrame restapi.TodoItemResponse
	found := false
	for _, item := range today.Items {
		if item.Kind == "task" && item.TaskID == directTaskID {
			taskFrame = item
			found = true
		}
	}
	if !found || taskFrame.PlannedMinutes == nil || *taskFrame.PlannedMinutes != 60 || taskFrame.ScheduledStart != nil || taskFrame.ScheduledEnd != nil {
		t.Fatalf("Task frame=%+v items=%+v, want 60m direct Task", taskFrame, today.Items)
	}
	var taskStart, taskEnd sql.NullTime
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, taskFrame.ID).Scan(&taskStart, &taskEnd); err != nil {
		t.Fatal(err)
	}
	if taskStart.Valid || taskEnd.Valid {
		t.Fatalf("manual Task frame stored internal times: start=%+v end=%+v", taskStart, taskEnd)
	}
	completed := fixture.request(t, http.MethodPost, "/today-todo/items/"+taskFrame.ID+":complete", `{"actual_minutes":38}`, today.Revision)
	if completed.Code != http.StatusOK {
		t.Fatalf("complete Task status/body=%d/%s", completed.Code, completed.Body.String())
	}
	var status string
	var actual sql.NullInt32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT status,actual_minutes FROM tasks WHERE id=$1`, directTaskID).Scan(&status, &actual); err != nil {
		t.Fatal(err)
	}
	if status != "done" || !actual.Valid || actual.Int32 != 38 {
		t.Fatalf("Task source=%s/%+v, want done/38", status, actual)
	}
	today = decodeToday(t, completed)
	completedRow := itemByID(today.Items, taskFrame.ID)
	if !completedRow.Completed || !completedRow.SourceCompleted {
		t.Fatalf("completed Task row=%+v", completedRow)
	}
	completedDuplicate := fixture.request(t, http.MethodPost, "/today-todo/tasks/"+directTaskID, "", today.Revision)
	if completedDuplicate.Code != http.StatusConflict {
		t.Fatalf("AddTask for completed source status/body=%d/%s, want 409", completedDuplicate.Code, completedDuplicate.Body.String())
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT status,actual_minutes FROM tasks WHERE id=$1`, directTaskID).Scan(&status, &actual); err != nil {
		t.Fatal(err)
	}
	if status != "done" || !actual.Valid || actual.Int32 != 38 {
		t.Fatalf("rejected completed duplicate mutated Task source=%s/%+v", status, actual)
	}
	reopened := fixture.request(t, http.MethodPost, "/today-todo/items/"+taskFrame.ID+":reopen", "", today.Revision)
	if reopened.Code != http.StatusOK {
		t.Fatalf("reopen Task status/body=%d/%s", reopened.Code, reopened.Body.String())
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT status,actual_minutes FROM tasks WHERE id=$1`, directTaskID).Scan(&status, &actual); err != nil {
		t.Fatal(err)
	}
	if status == "done" || actual.Valid {
		t.Fatalf("reopened Task source=%s/%+v, want incomplete/NULL", status, actual)
	}
	today = decodeToday(t, reopened)
	if itemByID(today.Items, taskFrame.ID).Completed {
		t.Fatalf("reopened frame remains complete: %+v", itemByID(today.Items, taskFrame.ID))
	}
	childID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO action_items(id,task_id,title,position,estimated_minutes,priority) VALUES($1,$2,'Child action',0,15,'high')`, childID, directTaskID); err != nil {
		t.Fatal(err)
	}
	parentAdd := fixture.request(t, http.MethodPost, "/today-todo/tasks/"+directTaskID, "", today.Revision)
	if parentAdd.Code == http.StatusOK {
		t.Fatalf("AddTask accepted Task after a child was added: %s", parentAdd.Body.String())
	}
	var storedEstimate int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT manual_estimated_minutes FROM tasks WHERE id=$1`, directTaskID).Scan(&storedEstimate); err != nil {
		t.Fatal(err)
	}
	if storedEstimate != 60 {
		t.Fatalf("Task estimate mutated during rejected parent add: %d", storedEstimate)
	}
	_, _ = fixture.pool.Exec(t.Context(), `DELETE FROM tasks WHERE id=$1`, directTaskID)
}

func (f *todoFixture) request(t *testing.T, method, path, body string, revision int32) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), restapi.UserIDContextKey, f.userID))
	if revision > 0 {
		req.Header.Set("If-Match", `"`+strconv.Itoa(int(revision))+`"`)
	}
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, req)
	return response
}

func decodeToday(t *testing.T, response *httptest.ResponseRecorder) restapi.TodoTodayResponse {
	t.Helper()
	var value restapi.TodoTodayResponse
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode Todo response: %v; body=%s", err, response.Body.String())
	}
	return value
}

func TestTodoGeneratesOnceAndManualListEditsDoNotReflowWork(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 30)
	enableInitialActionItemGeneration(t, fixture)
	initialResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial GET today status/body=%d/%s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decodeToday(t, initialResponse)
	if len(initial.Items) != 1 || initial.Items[0].Kind != "action_item" || initial.Items[0].ActionItemID != fixture.itemID {
		t.Fatalf("initial generated Todo=%+v; want one selected ActionItem", initial.Items)
	}
	actionFrame := initial.Items[0]
	if actionFrame.ScheduledStart != nil || actionFrame.ScheduledEnd != nil {
		t.Fatalf("generated work leaked internal times in API response: %+v", actionFrame)
	}
	var originalStart, originalEnd time.Time
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, actionFrame.ID).Scan(&originalStart, &originalEnd); err != nil {
		t.Fatal(err)
	}
	if !originalEnd.After(originalStart) {
		t.Fatalf("generated work internal interval=%s-%s; want positive persisted interval", originalStart, originalEnd)
	}

	manualTaskID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'Added after generation',15,'urgent','open',$2)`, manualTaskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	unchangedResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if unchangedResponse.Code != http.StatusOK {
		t.Fatalf("repeat GET today status/body=%d/%s", unchangedResponse.Code, unchangedResponse.Body.String())
	}
	unchanged := decodeToday(t, unchangedResponse)
	if unchanged.Revision != initial.Revision || len(unchanged.Items) != 1 {
		t.Fatalf("source creation repaired existing Todo: initial=%+v after=%+v", initial, unchanged)
	}
	added := fixture.request(t, http.MethodPost, "/today-todo/tasks/"+manualTaskID, "", unchanged.Revision)
	if added.Code != http.StatusOK {
		t.Fatalf("manual AddTask status/body=%d/%s", added.Code, added.Body.String())
	}
	afterAdd := decodeToday(t, added)
	var manualFrame restapi.TodoItemResponse
	for _, item := range afterAdd.Items {
		if item.Kind == "task" && item.TaskID == manualTaskID {
			manualFrame = item
		}
	}
	if manualFrame.ID == "" || manualFrame.ScheduledStart != nil || manualFrame.ScheduledEnd != nil {
		t.Fatalf("manual work row=%+v; want row with hidden, unset work times", manualFrame)
	}
	var manualStart, manualEnd sql.NullTime
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, manualFrame.ID).Scan(&manualStart, &manualEnd); err != nil {
		t.Fatal(err)
	}
	if manualStart.Valid || manualEnd.Valid {
		t.Fatalf("manual work unexpectedly stored internal times: start=%+v end=%+v", manualStart, manualEnd)
	}

	reorderBody, err := json.Marshal(struct {
		ItemIDs []string `json:"item_ids"`
	}{ItemIDs: []string{manualFrame.ID, actionFrame.ID}})
	if err != nil {
		t.Fatal(err)
	}
	reordered := fixture.request(t, http.MethodPost, "/today-todo:reorder", string(reorderBody), afterAdd.Revision)
	if reordered.Code != http.StatusOK {
		t.Fatalf("manual reorder status/body=%d/%s", reordered.Code, reordered.Body.String())
	}
	afterReorder := decodeToday(t, reordered)
	if len(afterReorder.Items) != 2 || afterReorder.Items[0].ID != manualFrame.ID || afterReorder.Items[1].ID != actionFrame.ID {
		t.Fatalf("reordered Todo=%+v; want manual Task before original ActionItem", afterReorder.Items)
	}
	var position int32
	var reorderedStart, reorderedEnd time.Time
	if err := fixture.pool.QueryRow(t.Context(), `SELECT position,scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, actionFrame.ID).Scan(&position, &reorderedStart, &reorderedEnd); err != nil {
		t.Fatal(err)
	}
	if position != 1 || !reorderedStart.Equal(originalStart) || !reorderedEnd.Equal(originalEnd) {
		t.Fatalf("reordered ActionItem position/times=%d/%s-%s; want position 1 and unchanged %s-%s", position, reorderedStart, reorderedEnd, originalStart, originalEnd)
	}

	removed := fixture.request(t, http.MethodPost, "/today-todo/items/"+manualFrame.ID+":remove", "", afterReorder.Revision)
	if removed.Code != http.StatusOK {
		t.Fatalf("manual remove status/body=%d/%s", removed.Code, removed.Body.String())
	}
	afterRemove := decodeToday(t, removed)
	if len(afterRemove.Items) != 1 || afterRemove.Items[0].ID != actionFrame.ID || afterRemove.Items[0].ScheduledStart != nil || afterRemove.Items[0].ScheduledEnd != nil {
		t.Fatalf("after remove Todo=%+v; want only original ActionItem with hidden internal time", afterRemove.Items)
	}
	var finalStart, finalEnd time.Time
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, actionFrame.ID).Scan(&finalStart, &finalEnd); err != nil {
		t.Fatal(err)
	}
	if !finalStart.Equal(originalStart) || !finalEnd.Equal(originalEnd) {
		t.Fatalf("manual list edits changed internal work interval from %s-%s to %s-%s", originalStart, originalEnd, finalStart, finalEnd)
	}
}

func TestTodoFirstManualAddCreatesInitialSelectionBeforeAddingActionItem(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 30)
	enableInitialActionItemGeneration(t, fixture)
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE users SET active_start_time=TIME '00:00',active_end_time=TIME '00:45' WHERE id=$1`, fixture.userID); err != nil {
		t.Fatal(err)
	}
	manualTaskID, manualActionID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,priority,status,changed_by) VALUES($1,$2,$2,'Manual-only source parent','low','open',$2)`, manualTaskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO action_items(id,task_id,title,position,estimated_minutes,priority) VALUES($1,$2,'Manual-only ActionItem',0,60,'low')`, manualActionID, manualTaskID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, manualTaskID)
	})

	// The 30-minute high-priority source fits the 45-minute active window;
	// the 60-minute low-priority source does not fit and is manually added.
	added := fixture.request(t, http.MethodPost, "/today-todo/action-items/"+manualActionID, "", 1)
	if added.Code != http.StatusOK {
		t.Fatalf("first API request AddActionItem status/body=%d/%s", added.Code, added.Body.String())
	}
	firstSnapshot := decodeToday(t, added)
	if firstSnapshot.Revision != 2 || len(firstSnapshot.Items) != 2 {
		t.Fatalf("first AddActionItem response=%+v; want generated selection plus manual item at revision 2", firstSnapshot)
	}
	var generated, manual restapi.TodoItemResponse
	for _, item := range firstSnapshot.Items {
		if item.Kind != "action_item" {
			t.Fatalf("unexpected Todo item kind after first AddActionItem: %+v", item)
		}
		if item.ActionItemID == fixture.itemID {
			generated = item
		}
		if item.ActionItemID == manualActionID {
			manual = item
		}
		if item.ScheduledStart != nil || item.ScheduledEnd != nil {
			t.Fatalf("work internal times visible after first AddActionItem: %+v", item)
		}
	}
	if generated.ID == "" || manual.ID == "" || generated.ID == manual.ID {
		t.Fatalf("first AddActionItem did not preserve initial selection and add the requested source: %+v", firstSnapshot.Items)
	}
	var generatedStart, generatedEnd time.Time
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, generated.ID).Scan(&generatedStart, &generatedEnd); err != nil {
		t.Fatal(err)
	}
	if !generatedEnd.After(generatedStart) {
		t.Fatalf("initially selected ActionItem has no positive internal interval: %s-%s", generatedStart, generatedEnd)
	}
	var manualStart, manualEnd sql.NullTime
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, manual.ID).Scan(&manualStart, &manualEnd); err != nil {
		t.Fatal(err)
	}
	if manualStart.Valid || manualEnd.Valid {
		t.Fatalf("manually added ActionItem unexpectedly has internal times: start=%+v end=%+v", manualStart, manualEnd)
	}

	get := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if get.Code != http.StatusOK {
		t.Fatalf("GET after first AddActionItem status/body=%d/%s", get.Code, get.Body.String())
	}
	secondSnapshot := decodeToday(t, get)
	if secondSnapshot.ID != firstSnapshot.ID || secondSnapshot.Revision != firstSnapshot.Revision || len(secondSnapshot.Items) != 2 || itemByID(secondSnapshot.Items, generated.ID).ID == "" || itemByID(secondSnapshot.Items, manual.ID).ID == "" {
		t.Fatalf("GET after first AddActionItem lost or regenerated the initial selection: before=%+v after=%+v", firstSnapshot, secondSnapshot)
	}
}

func TestTodoManualAddCompleteActualReopenAndRevisionFlow(t *testing.T) {
	fixture := newTodoFixture(t, "America/Los_Angeles", 60)
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE users SET active_start_time=TIME '00:00',active_end_time=TIME '00:15' WHERE id=$1`, fixture.userID); err != nil {
		t.Fatal(err)
	}
	get := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if get.Code != http.StatusOK {
		t.Fatalf("GET today status/body=%d/%s", get.Code, get.Body.String())
	}
	today := decodeToday(t, get)
	if today.Revision != 1 || len(today.Items) != 0 {
		t.Fatalf("initial snapshot=%+v, want revision 1 and no source that fits the 15-minute active-time window", today)
	}
	added := fixture.request(t, http.MethodPost, "/today-todo/action-items/"+fixture.itemID, "", today.Revision)
	if added.Code != http.StatusOK {
		t.Fatalf("manual add status/body=%d/%s", added.Code, added.Body.String())
	}
	today = decodeToday(t, added)
	if len(today.Items) != 1 {
		t.Fatalf("after manual add snapshot=%+v, want one whole ActionItem", today.Items)
	}
	currentFrame := today.Items[0]
	if currentFrame.ActionItemID != fixture.itemID || currentFrame.PlannedMinutes == nil || *currentFrame.PlannedMinutes != 60 || currentFrame.ScheduledStart != nil || currentFrame.ScheduledEnd != nil {
		t.Fatalf("after manual add items=%+v, want whole 60-minute ActionItem without public work timestamps", today.Items)
	}
	frameID := currentFrame.ID
	completed := fixture.request(t, http.MethodPost, "/today-todo/items/"+frameID+":complete", `{"actual_minutes":50}`, today.Revision)
	if completed.Code != http.StatusOK {
		t.Fatalf("complete status/body=%d/%s", completed.Code, completed.Body.String())
	}
	today = decodeToday(t, completed)
	if len(today.Items) != 1 || !itemByID(today.Items, frameID).Completed || !itemByID(today.Items, frameID).SourceCompleted || itemByID(today.Items, frameID).ActualMinutes == nil || *itemByID(today.Items, frameID).ActualMinutes != 50 {
		t.Fatalf("completed snapshot=%+v, want whole source completed and actual minutes 50", today)
	}
	var taskStatus string
	var actualMinutes int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT t.status,ai.actual_minutes FROM tasks t JOIN action_items ai ON ai.task_id=t.id WHERE t.id=$1`, fixture.taskID).Scan(&taskStatus, &actualMinutes); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "done" || actualMinutes != 50 {
		t.Fatalf("owner source state=%s/%d, want done/50", taskStatus, actualMinutes)
	}
	reopened := fixture.request(t, http.MethodPost, "/today-todo/items/"+frameID+":reopen", "", today.Revision)
	if reopened.Code != http.StatusOK {
		t.Fatalf("reopen status/body=%d/%s", reopened.Code, reopened.Body.String())
	}
	today = decodeToday(t, reopened)
	if len(today.Items) != 1 || itemByID(today.Items, frameID).Completed {
		t.Fatalf("reopened snapshot=%+v, want unfinished source and frame", today)
	}
	var reopenedSource bool
	var actualStillSet bool
	if err := fixture.pool.QueryRow(t.Context(), `SELECT completed,actual_minutes IS NOT NULL FROM action_items WHERE id=$1`, fixture.itemID).Scan(&reopenedSource, &actualStillSet); err != nil {
		t.Fatal(err)
	}
	if reopenedSource || actualStillSet {
		t.Fatalf("reopened ActionItem state completed=%t actual_set=%t, want false/false", reopenedSource, actualStillSet)
	}
	todoDate, err := time.Parse("2006-01-02", today.Date)
	if err != nil {
		t.Fatal(err)
	}
	newTimezone := offsetDifferentTimezoneOnDate(t, "America/Los_Angeles", todoDate)
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE users SET timezone=$2 WHERE id=$1`, fixture.userID, newTimezone); err != nil {
		t.Fatalf("change user timezone: %v", err)
	}
	savedTimezone := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if savedTimezone.Code != http.StatusOK {
		t.Fatalf("reload saved todo after timezone change status/body=%d/%s", savedTimezone.Code, savedTimezone.Body.String())
	}
	savedSnapshot := decodeToday(t, savedTimezone)
	if savedSnapshot.Timezone != "America/Los_Angeles" || savedSnapshot.Revision != today.Revision {
		t.Fatalf("saved todo after profile timezone change=%s/%d, want original timezone and revision %d", savedSnapshot.Timezone, savedSnapshot.Revision, today.Revision)
	}
	stale := fixture.request(t, http.MethodPost, "/today-todo/action-items/"+fixture.itemID, "", 1)
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale revision status/body=%d/%s, want 412", stale.Code, stale.Body.String())
	}
}

func TestTodoSavedScheduleUsesPersistedTimezoneAfterUserTimezoneChange(t *testing.T) {
	fixture := newTodoFixture(t, "America/Los_Angeles", 60)
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	laToday, tokyoToday := now.In(la), now.In(tokyo)
	date := time.Date(laToday.Year(), laToday.Month(), laToday.Day(), 23, 30, 0, 0, la)
	created := fixture.request(t, http.MethodPost, "/schedule-rules", `{"title":"Late boundary Schedule","timezone":"America/Los_Angeles","local_start":"`+date.Format("2006-01-02T15:04:05")+`","local_end":"`+date.Add(15*time.Minute).Format("2006-01-02T15:04:05")+`","all_day":false,"frequency":"once","interval":0}`, 0)
	if created.Code != http.StatusCreated {
		t.Fatalf("create boundary Schedule status/body=%d/%s", created.Code, created.Body.String())
	}
	initial := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initial.Code != http.StatusOK {
		t.Fatalf("initial GET status/body=%d/%s", initial.Code, initial.Body.String())
	}
	saved := decodeToday(t, initial)
	if len(saved.Items) != 1 || saved.Items[0].Kind != "schedule" || saved.Items[0].ScheduledStart == nil {
		t.Fatalf("initial boundary plan=%+v, want one fixed Schedule", saved.Items)
	}
	savedStart, scheduleID := *saved.Items[0].ScheduledStart, saved.Items[0].ID
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE users SET timezone='Asia/Tokyo' WHERE id=$1`, fixture.userID); err != nil {
		t.Fatalf("change user timezone: %v", err)
	}
	tx, err := fixture.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	sources, err := application.NewTodoSourceReaderFromTx(tx).ReadSavedScheduleSources(t.Context(), fixture.userID, date, "America/Los_Angeles")
	if err != nil {
		t.Fatalf("read saved schedule sources in persisted timezone: %v", err)
	}
	found := false
	for _, source := range sources {
		if source.RuleID == itemByID(saved.Items, scheduleID).ScheduleRuleID && source.StartAt.Equal(time.Date(date.Year(), date.Month(), date.Day(), 23, 30, 0, 0, la).UTC()) {
			found = true
		}
	}
	if !found {
		t.Fatalf("saved timezone sources=%+v, want original boundary occurrence", sources)
	}
	if laToday.Year() == tokyoToday.Year() && laToday.YearDay() == tokyoToday.YearDay() {
		reloaded := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
		if reloaded.Code != http.StatusOK {
			t.Fatalf("reload saved plan status/body=%d/%s", reloaded.Code, reloaded.Body.String())
		}
		current := decodeToday(t, reloaded)
		item := itemByID(current.Items, scheduleID)
		if current.Timezone != "America/Los_Angeles" || item.ID == "" || item.ScheduledStart == nil || *item.ScheduledStart != savedStart {
			t.Fatalf("reloaded snapshot=%+v, want persisted LA plan and exact fixed Schedule time", current)
		}
		hidden := fixture.request(t, http.MethodPost, "/today-todo/schedules/"+scheduleID+":hide", "", current.Revision)
		if hidden.Code != http.StatusOK {
			t.Fatalf("hide persisted boundary Schedule after timezone change status/body=%d/%s", hidden.Code, hidden.Body.String())
		}
		hiddenSnapshot := decodeToday(t, hidden)
		if len(hiddenSnapshot.HiddenSchedules) != 1 || hiddenSnapshot.HiddenSchedules[0].ScheduledStart == nil || *hiddenSnapshot.HiddenSchedules[0].ScheduledStart != savedStart {
			t.Fatalf("hidden schedule after reflow=%+v, want source found and persisted time fixed", hiddenSnapshot.HiddenSchedules)
		}
	}
}

func offsetDifferentTimezoneOnDate(t *testing.T, sourceTimezone string, targetDate time.Time) string {
	t.Helper()
	now := time.Now()
	sourceLocation, err := time.LoadLocation(sourceTimezone)
	if err != nil {
		t.Fatal(err)
	}
	_, sourceOffset := now.In(sourceLocation).Zone()
	for _, candidate := range []string{"America/Denver", "America/Anchorage", "America/Phoenix", "America/Vancouver", "America/Chicago", "America/New_York"} {
		location, err := time.LoadLocation(candidate)
		if err != nil {
			continue
		}
		local := now.In(location)
		_, offset := local.Zone()
		if offset != sourceOffset && local.Year() == targetDate.Year() && local.YearDay() == targetDate.YearDay() {
			return candidate
		}
	}
	t.Skip("no available offset-different timezone shares the saved todo date")
	return ""
}

func itemByID(items []restapi.TodoItemResponse, id string) restapi.TodoItemResponse {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return restapi.TodoItemResponse{}
}

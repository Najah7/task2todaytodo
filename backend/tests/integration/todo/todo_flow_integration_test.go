//go:build integration

package todo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
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
	router.Post("/today-todo:regenerate", handlers.Todos.Regenerate)
	router.Post("/today-todo/action-items/{actionItemId}", handlers.Todos.AddActionItem)
	router.Post("/today-todo/tasks/{taskId}", handlers.Todos.AddTask)
	router.Delete("/today-todo/items/{itemId}", handlers.Todos.RemoveActionItem)
	router.Post("/today-todo/items/{itemId}:complete", handlers.Todos.CompleteFrame)
	router.Post("/today-todo/items/{itemId}:reopen", handlers.Todos.ReopenFrame)
	router.Post("/today-todo/schedules/{itemId}:hide", handlers.Todos.HideSchedule)
	router.Post("/today-todo/schedules/{itemId}:restore", handlers.Todos.RestoreSchedule)
	router.Post("/schedule-rules", handlers.Schedule.CreateRule)
	router.Patch("/tasks/{taskId}/action-items/{id}", handlers.ActionItem.Update)
	router.Post("/tasks/{taskId}/action-items/{id}:complete", handlers.ActionItem.Complete)
	router.Post("/tasks/{taskId}/action-items/{id}:reopen", handlers.ActionItem.Reopen)
	router.Post("/tasks/{id}:reopen", handlers.Task.Reopen)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	return &todoFixture{pool: pool, userID: userID, taskID: taskID, itemID: itemID, router: router}
}

func TestTodoDirectTaskAddCompleteReopenAndChildExclusion(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
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
	if !found || taskFrame.PlannedMinutes == nil || *taskFrame.PlannedMinutes != 60 {
		t.Fatalf("Task frame=%+v items=%+v, want 60m direct Task", taskFrame, today.Items)
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
	if !completedRow.Completed || completedRow.CompletesSource {
		t.Fatalf("completed Task row=%+v", completedRow)
	}
	startedDuplicate := fixture.request(t, http.MethodPost, "/today-todo/tasks/"+directTaskID, "", today.Revision)
	if startedDuplicate.Code != http.StatusConflict {
		t.Fatalf("AddTask for started source status/body=%d/%s, want 409", startedDuplicate.Code, startedDuplicate.Body.String())
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT status,actual_minutes FROM tasks WHERE id=$1`, directTaskID).Scan(&status, &actual); err != nil {
		t.Fatal(err)
	}
	if status != "done" || !actual.Valid || actual.Int32 != 38 {
		t.Fatalf("rejected started duplicate mutated Task source=%s/%+v", status, actual)
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

func TestTodoManualAddCompleteActualReopenAndRevisionFlow(t *testing.T) {
	fixture := newTodoFixture(t, "America/Los_Angeles", 60)
	get := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if get.Code != http.StatusOK {
		t.Fatalf("GET today status/body=%d/%s", get.Code, get.Body.String())
	}
	today := decodeToday(t, get)
	if today.Revision != 1 || len(today.Items) != 0 {
		t.Fatalf("initial snapshot=%+v, want revision 1 and no auto candidate without Project allocation", today)
	}
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	localToday := time.Now().In(location)
	previousDate := time.Date(localToday.Year(), localToday.Month(), localToday.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
	historyTodoID, historyItemID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todos(id,user_id,todo_date,timezone) VALUES($1,$2,$3,'America/Los_Angeles')`, historyTodoID, fixture.userID, previousDate.Format("2006-01-02")); err != nil {
		t.Fatalf("insert history todo: %v", err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todo_items(id,todo_id,kind,action_item_id,planned_minutes,completed,position) VALUES($1,$2,'action_item',$3,15,true,0)`, historyItemID, historyTodoID, fixture.itemID); err != nil {
		t.Fatalf("insert completed history frame: %v", err)
	}
	added := fixture.request(t, http.MethodPost, "/today-todo/action-items/"+fixture.itemID, "", today.Revision)
	if added.Code != http.StatusOK {
		t.Fatalf("manual add status/body=%d/%s", added.Code, added.Body.String())
	}
	today = decodeToday(t, added)
	if len(today.Items) != 2 {
		t.Fatalf("after add snapshot=%+v, want current row plus one prior completed history row", today)
	}
	var currentFrame restapi.TodoItemResponse
	foundHistory := false
	for _, item := range today.Items {
		if item.History {
			foundHistory = item.ID == historyItemID && item.Completed && item.PlannedMinutes != nil && *item.PlannedMinutes == 15
		} else {
			currentFrame = item
		}
	}
	if currentFrame.ID == "" || currentFrame.ActionItemID != fixture.itemID || currentFrame.PlannedMinutes == nil || *currentFrame.PlannedMinutes != 45 || !foundHistory {
		t.Fatalf("after add items=%+v, want 45-minute remainder and 15-minute history", today.Items)
	}
	frameID := currentFrame.ID
	completed := fixture.request(t, http.MethodPost, "/today-todo/items/"+frameID+":complete", `{"actual_minutes":50}`, today.Revision)
	if completed.Code != http.StatusOK {
		t.Fatalf("complete status/body=%d/%s", completed.Code, completed.Body.String())
	}
	today = decodeToday(t, completed)
	if len(today.Items) != 2 || !itemByID(today.Items, frameID).Completed || itemByID(today.Items, frameID).CompletesSource || itemByID(today.Items, frameID).ActualMinutes == nil || *itemByID(today.Items, frameID).ActualMinutes != 50 {
		t.Fatalf("completed snapshot=%+v, want completed frame capability cleared and actual minutes 50", today)
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
	if len(today.Items) != 2 || itemByID(today.Items, frameID).Completed {
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
	oldRevision := today.Revision
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
		t.Fatalf("saved todo timezone/revision=%s/%d, want old timezone and unchanged revision", savedSnapshot.Timezone, savedSnapshot.Revision)
	}
	regenerateStarted := time.Now()
	regenerated := fixture.request(t, http.MethodPost, "/today-todo:regenerate", "", savedSnapshot.Revision)
	t.Logf("DB-backed full regeneration HTTP latency: %s", time.Since(regenerateStarted))
	if regenerated.Code != http.StatusOK {
		t.Fatalf("regenerate after timezone change status/body=%d/%s", regenerated.Code, regenerated.Body.String())
	}
	today = decodeToday(t, regenerated)
	if today.Timezone != newTimezone || today.Revision != oldRevision+1 {
		t.Fatalf("regenerated todo timezone/revision=%s/%d, want %s/%d", today.Timezone, today.Revision, newTimezone, oldRevision+1)
	}
	stale := fixture.request(t, http.MethodPost, "/today-todo/action-items/"+fixture.itemID, "", 1)
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale revision status/body=%d/%s, want 412", stale.Code, stale.Body.String())
	}
}

func TestTodoPendingActionItemWithHistoryStillAutoGeneratesRemainder(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
	todayDate := time.Now().UTC().Format("2006-01-02")
	todoDate, err := time.Parse("2006-01-02", todayDate)
	if err != nil {
		t.Fatal(err)
	}
	historyTodoID, historyFrameID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todos(id,user_id,todo_date,timezone) VALUES($1,$2,$3,'UTC')`, historyTodoID, fixture.userID, todoDate.AddDate(0, 0, -1)); err != nil {
		t.Fatalf("insert ActionItem history todo: %v", err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todo_items(id,todo_id,kind,action_item_id,planned_minutes,completed,position) VALUES($1,$2,'action_item',$3,15,true,0)`, historyFrameID, historyTodoID, fixture.itemID); err != nil {
		t.Fatalf("insert prior completed ActionItem frame: %v", err)
	}

	response := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if response.Code != http.StatusOK {
		t.Fatalf("GET pending ActionItem with history status/body=%d/%s", response.Code, response.Body.String())
	}
	today := decodeToday(t, response)
	var generated *restapi.TodoItemResponse
	for i := range today.Items {
		item := &today.Items[i]
		if !item.History && item.Kind == "action_item" && item.ActionItemID == fixture.itemID {
			generated = item
		}
	}
	if generated == nil || generated.PlannedMinutes == nil || *generated.PlannedMinutes != 45 {
		t.Fatalf("pending ActionItem remaining work=%+v, want automatically generated 45-minute row", today.Items)
	}
	history := itemByID(today.Items, historyFrameID)
	if !history.History || !history.Completed || history.PlannedMinutes == nil || *history.PlannedMinutes != 15 {
		t.Fatalf("pending ActionItem history=%+v, want prior completed 15-minute frame", history)
	}
}

func TestTodoSplitFramesCompleteSourceOnlyOnLastFrameAndReopenPreservesOther(t *testing.T) {
	fixture := newTodoFixture(t, "America/Los_Angeles", 60)
	initial := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initial.Code != http.StatusOK {
		t.Fatalf("GET today status/body=%d/%s", initial.Code, initial.Body.String())
	}
	today := decodeToday(t, initial)
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	localDate := time.Now().In(location)
	dayStart := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 9, 0, 0, 0, location)
	firstID, secondID := ulid.Make().String(), ulid.Make().String()
	_, err = fixture.pool.Exec(t.Context(), `INSERT INTO todo_items(id,todo_id,kind,action_item_id,planned_minutes,scheduled_start,scheduled_end,position)
		VALUES ($1,$3,'action_item',$4,30,$5::timestamptz,$5::timestamptz + interval '30 minutes',0),
		       ($2,$3,'action_item',$4,30,$6::timestamptz,$6::timestamptz + interval '30 minutes',1)`,
		firstID, secondID, today.ID, fixture.itemID, dayStart, dayStart.Add(35*time.Minute))
	if err != nil {
		t.Fatalf("seed split Todo frames: %v", err)
	}
	previousDate := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -1)
	historyTodoID, historyID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todos(id,user_id,todo_date,timezone) VALUES($1,$2,$3,'America/Los_Angeles')`, historyTodoID, fixture.userID, previousDate.Format("2006-01-02")); err != nil {
		t.Fatalf("seed prior history todo: %v", err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todo_items(id,todo_id,kind,action_item_id,planned_minutes,completed,position) VALUES($1,$2,'action_item',$3,15,true,0)`, historyID, historyTodoID, fixture.itemID); err != nil {
		t.Fatalf("seed prior completed history: %v", err)
	}
	read := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if read.Code != http.StatusOK {
		t.Fatalf("GET split today status/body=%d/%s", read.Code, read.Body.String())
	}
	today = decodeToday(t, read)
	if len(today.Items) != 3 || itemByID(today.Items, firstID).CompletesSource || itemByID(today.Items, secondID).CompletesSource || !itemByID(today.Items, historyID).History {
		t.Fatalf("split snapshot=%+v, want two unfinished 30-minute rows plus prior history", today.Items)
	}

	partial := fixture.request(t, http.MethodPost, "/today-todo/items/"+firstID+":complete", `{"actual_minutes":20}`, today.Revision)
	if partial.Code != http.StatusBadRequest {
		t.Fatalf("partial complete with actual status/body=%d/%s, want 400", partial.Code, partial.Body.String())
	}
	var partialCompleted bool
	var currentRevision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT i.completed,l.revision FROM todo_items i JOIN todos l ON l.id=i.todo_id WHERE i.id=$1`, firstID).Scan(&partialCompleted, &currentRevision); err != nil {
		t.Fatal(err)
	}
	if partialCompleted || currentRevision != today.Revision {
		t.Fatalf("after rejected partial actual completed=%t revision=%d, want false/%d", partialCompleted, currentRevision, today.Revision)
	}

	firstDone := fixture.request(t, http.MethodPost, "/today-todo/items/"+firstID+":complete", "", today.Revision)
	if firstDone.Code != http.StatusOK {
		t.Fatalf("complete first split row status/body=%d/%s", firstDone.Code, firstDone.Body.String())
	}
	today = decodeToday(t, firstDone)
	if !itemByID(today.Items, firstID).Completed || !itemByID(today.Items, secondID).CompletesSource {
		t.Fatalf("after first completion=%+v, want remaining row to complete source", today.Items)
	}
	removeStarted := fixture.request(t, http.MethodDelete, "/today-todo/items/"+secondID, "", today.Revision)
	if removeStarted.Code != http.StatusConflict {
		t.Fatalf("remove sibling after work started status/body=%d/%s, want 409", removeStarted.Code, removeStarted.Body.String())
	}
	changedEstimate := fixture.request(t, http.MethodPatch, "/tasks/"+fixture.taskID+"/action-items/"+fixture.itemID, `{"estimated_minutes":90}`, 0)
	if changedEstimate.Code != http.StatusConflict {
		t.Fatalf("change started estimate without confirmation status/body=%d/%s, want 409", changedEstimate.Code, changedEstimate.Body.String())
	}
	var storedEstimate int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT estimated_minutes FROM action_items WHERE id=$1`, fixture.itemID).Scan(&storedEstimate); err != nil {
		t.Fatal(err)
	}
	if storedEstimate != 60 {
		t.Fatalf("estimate after rejected started edit=%d, want 60", storedEstimate)
	}

	lastDone := fixture.request(t, http.MethodPost, "/today-todo/items/"+secondID+":complete", `{"actual_minutes":55}`, today.Revision)
	if lastDone.Code != http.StatusOK {
		t.Fatalf("complete final split row status/body=%d/%s", lastDone.Code, lastDone.Body.String())
	}
	today = decodeToday(t, lastDone)
	if !itemByID(today.Items, firstID).Completed || !itemByID(today.Items, secondID).Completed || itemByID(today.Items, secondID).CompletesSource {
		t.Fatalf("after source completion=%+v, want both rows complete and capability false", today.Items)
	}
	var taskStatus string
	var actualMinutes sql.NullInt32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT t.status,ai.actual_minutes FROM tasks t JOIN action_items ai ON ai.task_id=t.id WHERE t.id=$1`, fixture.taskID).Scan(&taskStatus, &actualMinutes); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "done" || !actualMinutes.Valid || actualMinutes.Int32 != 55 {
		t.Fatalf("owner source state=%s/%+v, want done/55", taskStatus, actualMinutes)
	}

	reopened := fixture.request(t, http.MethodPost, "/today-todo/items/"+firstID+":reopen", "", today.Revision)
	if reopened.Code != http.StatusOK {
		t.Fatalf("reopen first completed row status/body=%d/%s", reopened.Code, reopened.Body.String())
	}
	today = decodeToday(t, reopened)
	if itemByID(today.Items, firstID).Completed || !itemByID(today.Items, secondID).Completed {
		t.Fatalf("after one frame reopen=%+v, want only selected frame reopened", today.Items)
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT t.status,ai.actual_minutes FROM tasks t JOIN action_items ai ON ai.task_id=t.id WHERE t.id=$1`, fixture.taskID).Scan(&taskStatus, &actualMinutes); err != nil {
		t.Fatal(err)
	}
	if taskStatus == "done" || actualMinutes.Valid {
		t.Fatalf("owner source after reopen=%s/%+v, want not done with actual cleared", taskStatus, actualMinutes)
	}

	taskCompleted := fixture.request(t, http.MethodPost, "/tasks/"+fixture.taskID+"/action-items/"+fixture.itemID+":complete", "", 0)
	if taskCompleted.Code != http.StatusOK {
		t.Fatalf("Task-owned source complete status/body=%d/%s", taskCompleted.Code, taskCompleted.Body.String())
	}
	taskReopened := fixture.request(t, http.MethodPost, "/tasks/"+fixture.taskID+"/action-items/"+fixture.itemID+":reopen", "", 0)
	if taskReopened.Code != http.StatusOK {
		t.Fatalf("Task-owned source reopen status/body=%d/%s", taskReopened.Code, taskReopened.Body.String())
	}
	for _, frameID := range []string{firstID, secondID, historyID} {
		var completed bool
		if err := fixture.pool.QueryRow(t.Context(), `SELECT completed FROM todo_items WHERE id=$1`, frameID).Scan(&completed); err != nil {
			t.Fatal(err)
		}
		if completed {
			t.Fatalf("Task-owned reopen left frame %s completed", frameID)
		}
	}
	var actualIsNull bool
	if err := fixture.pool.QueryRow(t.Context(), `SELECT t.status,ai.actual_minutes IS NULL FROM tasks t JOIN action_items ai ON ai.task_id=t.id WHERE ai.id=$1`, fixture.itemID).Scan(&taskStatus, &actualIsNull); err != nil {
		t.Fatal(err)
	}
	if taskStatus == "done" || !actualIsNull {
		t.Fatalf("Task-owned source after reopen status=%s actual_is_null=%t, want reopened with cleared actual", taskStatus, actualIsNull)
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

func TestTodoAllDaySuppressesActionItemsUntilScheduleHidden(t *testing.T) {
	fixture := newTodoFixture(t, "Asia/Tokyo", 45)
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(location)
	localDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)
	start := localDate.Format("2006-01-02T15:04:05")
	end := localDate.AddDate(0, 0, 1).Format("2006-01-02T15:04:05")
	body := `{"title":"All day blocker","timezone":"Asia/Tokyo","local_start":"` + start + `","local_end":"` + end + `","all_day":true,"frequency":"once","interval":0}`
	created := fixture.request(t, http.MethodPost, "/schedule-rules", body, 0)
	if created.Code != http.StatusCreated {
		t.Fatalf("create all-day Schedule status/body=%d/%s", created.Code, created.Body.String())
	}
	timedStart := localDate.Add(13 * time.Hour)
	timedBody := `{"title":"Other hidden Schedule","timezone":"Asia/Tokyo","local_start":"` + timedStart.Format("2006-01-02T15:04:05") + `","local_end":"` + timedStart.Add(time.Hour).Format("2006-01-02T15:04:05") + `","all_day":false,"frequency":"once","interval":0}`
	createdTimed := fixture.request(t, http.MethodPost, "/schedule-rules", timedBody, 0)
	if createdTimed.Code != http.StatusCreated {
		t.Fatalf("create timed Schedule status/body=%d/%s", createdTimed.Code, createdTimed.Body.String())
	}
	initial := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initial.Code != http.StatusOK {
		t.Fatalf("GET today status/body=%d/%s", initial.Code, initial.Body.String())
	}
	todayResponse := decodeToday(t, initial)
	if len(todayResponse.Items) != 2 {
		t.Fatalf("all-day snapshot=%+v, want both all-day and timed Schedule rows", todayResponse.Items)
	}
	allDayID, timedID := "", ""
	for _, item := range todayResponse.Items {
		if item.Kind != "schedule" || item.PlannedMinutes != nil {
			t.Fatalf("schedule row=%+v, want zero-work Schedule", item)
		}
		if item.DurationMinutes == 1440 {
			allDayID = item.ID
		} else {
			timedID = item.ID
		}
	}
	if allDayID == "" || timedID == "" {
		t.Fatalf("schedule rows=%+v, want all-day and timed IDs", todayResponse.Items)
	}
	var schedulePlannedMinutesNull bool
	if err := fixture.pool.QueryRow(t.Context(), `SELECT planned_minutes IS NULL FROM todo_items WHERE id=$1`, allDayID).Scan(&schedulePlannedMinutesNull); err != nil {
		t.Fatalf("read persisted all-day Schedule planned_minutes: %v", err)
	}
	if !schedulePlannedMinutesNull {
		t.Fatal("persisted Schedule planned_minutes should be SQL NULL")
	}
	blocked := fixture.request(t, http.MethodPost, "/today-todo/action-items/"+fixture.itemID, "", todayResponse.Revision)
	if blocked.Code != http.StatusConflict {
		t.Fatalf("add while all-day visible status/body=%d/%s, want 409", blocked.Code, blocked.Body.String())
	}
	hidden := fixture.request(t, http.MethodPost, "/today-todo/schedules/"+timedID+":hide", "", todayResponse.Revision)
	if hidden.Code != http.StatusOK {
		t.Fatalf("hide timed Schedule status/body=%d/%s", hidden.Code, hidden.Body.String())
	}
	todayResponse = decodeToday(t, hidden)
	if len(todayResponse.HiddenSchedules) != 1 || !todayResponse.HiddenSchedules[0].Hidden {
		t.Fatalf("hidden snapshot=%+v, want timed Schedule hidden separately", todayResponse)
	}
	hiddenAllDay := fixture.request(t, http.MethodPost, "/today-todo/schedules/"+allDayID+":hide", "", todayResponse.Revision)
	if hiddenAllDay.Code != http.StatusOK {
		t.Fatalf("hide all-day Schedule status/body=%d/%s", hiddenAllDay.Code, hiddenAllDay.Body.String())
	}
	todayResponse = decodeToday(t, hiddenAllDay)
	if len(todayResponse.HiddenSchedules) != 2 || len(todayResponse.Items) != 0 {
		t.Fatalf("hide both schedules snapshot=%+v, want both hidden", todayResponse)
	}
	added := fixture.request(t, http.MethodPost, "/today-todo/action-items/"+fixture.itemID, "", todayResponse.Revision)
	if added.Code != http.StatusOK {
		t.Fatalf("add after hiding Schedule status/body=%d/%s", added.Code, added.Body.String())
	}
	todayResponse = decodeToday(t, added)
	if len(todayResponse.Items) != 1 || todayResponse.Items[0].Kind != "action_item" || len(todayResponse.HiddenSchedules) != 2 {
		t.Fatalf("post-hide add snapshot=%+v, want ActionItem visible and both hidden Schedules retained", todayResponse)
	}
	restored := fixture.request(t, http.MethodPost, "/today-todo/schedules/"+allDayID+":restore", "", todayResponse.Revision)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore all-day Schedule status/body=%d/%s", restored.Code, restored.Body.String())
	}
	todayResponse = decodeToday(t, restored)
	if len(todayResponse.HiddenSchedules) != 0 || len(todayResponse.Items) != 2 || todayResponse.Items[0].Kind != "schedule" || todayResponse.Items[1].Kind != "schedule" {
		t.Fatalf("after restore snapshot=%+v, want both Schedules visible and reset ActionItem plan", todayResponse)
	}
}

func TestTodoEstimateChangeRequiresConfirmationThenRegeneratesAtomically(t *testing.T) {
	fixture := newTodoFixture(t, "America/Los_Angeles", 60)
	initial := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initial.Code != http.StatusOK {
		t.Fatalf("GET today status/body=%d/%s", initial.Code, initial.Body.String())
	}
	today := decodeToday(t, initial)
	added := fixture.request(t, http.MethodPost, "/today-todo/action-items/"+fixture.itemID, "", today.Revision)
	if added.Code != http.StatusOK {
		t.Fatalf("manual add status/body=%d/%s", added.Code, added.Body.String())
	}
	today = decodeToday(t, added)
	if len(today.Items) != 1 || today.Items[0].ActionItemID != fixture.itemID {
		t.Fatalf("before estimate change=%+v, want manual ActionItem frame", today.Items)
	}
	updatePath := "/tasks/" + fixture.taskID + "/action-items/" + fixture.itemID
	unconfirmed := fixture.request(t, http.MethodPatch, updatePath, `{"estimated_minutes":90}`, 0)
	if unconfirmed.Code != http.StatusConflict {
		t.Fatalf("unconfirmed estimate change status/body=%d/%s, want 409", unconfirmed.Code, unconfirmed.Body.String())
	}
	var estimate int
	var revision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT ai.estimated_minutes,l.revision FROM action_items ai JOIN todos l ON l.user_id=$2 AND l.todo_date=(now() AT TIME ZONE l.timezone)::date WHERE ai.id=$1`, fixture.itemID, fixture.userID).Scan(&estimate, &revision); err != nil {
		t.Fatal(err)
	}
	if estimate != 60 || revision != today.Revision {
		t.Fatalf("after unconfirmed edit estimate/revision=%d/%d, want 60/%d", estimate, revision, today.Revision)
	}
	confirmed := fixture.request(t, http.MethodPatch, updatePath, `{"estimated_minutes":90,"confirm_todo_regeneration":true}`, 0)
	if confirmed.Code != http.StatusOK {
		t.Fatalf("confirmed estimate change status/body=%d/%s", confirmed.Code, confirmed.Body.String())
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT ai.estimated_minutes,l.revision FROM action_items ai JOIN todos l ON l.user_id=$2 AND l.todo_date=(now() AT TIME ZONE l.timezone)::date WHERE ai.id=$1`, fixture.itemID, fixture.userID).Scan(&estimate, &revision); err != nil {
		t.Fatal(err)
	}
	if estimate != 90 || revision != today.Revision+1 {
		t.Fatalf("after confirmed edit estimate/revision=%d/%d, want 90/%d", estimate, revision, today.Revision+1)
	}
	var plannedFrames int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM todo_items i JOIN todos l ON l.id=i.todo_id WHERE l.user_id=$1 AND l.todo_date=(now() AT TIME ZONE l.timezone)::date AND i.action_item_id=$2`, fixture.userID, fixture.itemID).Scan(&plannedFrames); err != nil {
		t.Fatal(err)
	}
	if plannedFrames != 0 {
		t.Fatalf("frames after confirmed regeneration=%d, want reset to zero without allocation", plannedFrames)
	}
}

func TestTodoGenerationDatabaseLatencyWith100And1000Candidates(t *testing.T) {
	fixture := newTodoFixture(t, "America/Los_Angeles", 15)
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(location)
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	projectID, allocationID, revisionID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO projects(id,user_id,title,changed_by) VALUES($1,$2,'Generation benchmark',$2)`, projectID, fixture.userID); err != nil {
		t.Fatalf("insert benchmark Project: %v", err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE tasks SET project_id=$2 WHERE id=$1`, fixture.taskID, projectID); err != nil {
		t.Fatalf("assign benchmark Task to Project: %v", err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO project_time_allocation_rules(id,user_id,project_id) VALUES($1,$2,$3)`, allocationID, fixture.userID, projectID); err != nil {
		t.Fatalf("insert benchmark allocation rule: %v", err)
	}
	weekday := int32(date.Weekday())
	previousWeek := date.AddDate(0, 0, -7)
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO project_time_allocation_rule_revisions(
		id,rule_id,user_id,project_id,revision,effective_from,recurrence_start_date,weekday,local_start,local_end,end_day_offset,timezone,frequency,changed_by)
		VALUES($1,$2,$3,$4,1,$5,$5,$6,TIME '09:00',TIME '17:00',0,'America/Los_Angeles','weekly',$3)`,
		revisionID, allocationID, fixture.userID, projectID, previousWeek.Format("2006-01-02"), weekday); err != nil {
		t.Fatalf("insert benchmark allocation revision: %v", err)
	}
	seedActionItems := func(count int) {
		t.Helper()
		ids := make([]string, count)
		positions := make([]int32, count)
		for index := range ids {
			ids[index] = ulid.Make().String()
			positions[index] = int32(index + 1)
		}
		if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO action_items(id,task_id,title,position,estimated_minutes,priority)
			SELECT item_id,$1,'Benchmark candidate',position,15,'low'
			FROM unnest($2::text[],$3::integer[]) AS rows(item_id,position)`, fixture.taskID, ids, positions); err != nil {
			t.Fatalf("insert %d benchmark ActionItems: %v", count, err)
		}
	}
	seedActionItems(99)
	started := time.Now()
	initial := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	initialLatency := time.Since(started)
	if initial.Code != http.StatusOK {
		t.Fatalf("100-candidate initial generation status/body=%d/%s", initial.Code, initial.Body.String())
	}
	today := decodeToday(t, initial)
	t.Logf("DB-backed initial generation with 100 candidates: %s", initialLatency)
	seedActionItems(900)
	started = time.Now()
	regenerated := fixture.request(t, http.MethodPost, "/today-todo:regenerate", "", today.Revision)
	regenerationLatency := time.Since(started)
	if regenerated.Code != http.StatusOK {
		t.Fatalf("1000-candidate full regeneration status/body=%d/%s", regenerated.Code, regenerated.Body.String())
	}
	t.Logf("DB-backed full regeneration with 1000 candidates: %s", regenerationLatency)
}

func TestScheduleImpactRegeneratesAssigneesAndRollsBackSourceOnFailure(t *testing.T) {
	fixture := newTodoFixture(t, "Asia/Tokyo", 60)
	ctx := t.Context()
	actorID, projectID, ownerProjectID, memberID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	for _, userID := range []string{actorID, memberID} {
		if _, err := fixture.pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone) VALUES($1,'Schedule','Impact',$2,'unused',now(),'Asia/Tokyo')`, userID, userID+"@schedule-impact.test"); err != nil {
			t.Fatalf("insert Schedule impact user: %v", err)
		}
	}
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other','A-owned Schedule project','low','open',$2),($3,$4,'other','B-owned Schedule project','low','open',$4)`, projectID, actorID, ownerProjectID, fixture.userID); err != nil {
		t.Fatalf("insert Schedule impact Projects: %v", err)
	}
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'editor',$3),($1,$4,'editor',$3),($5,$3,'editor',$4)`, projectID, fixture.userID, actorID, memberID, ownerProjectID); err != nil {
		t.Fatalf("add Schedule impact Project members: %v", err)
	}
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO todos(id,user_id,todo_date,timezone) SELECT $1,id,(now() AT TIME ZONE timezone)::date,timezone FROM users WHERE id=$2`, ulid.Make().String(), memberID); err != nil {
		t.Fatalf("create C Today: %v", err)
	}
	var createdRuleIDs []string
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(cleanupCtx, `DROP TRIGGER IF EXISTS fail_schedule_todo_item_insert ON todo_items`)
		_, _ = fixture.pool.Exec(cleanupCtx, `DROP FUNCTION IF EXISTS fail_schedule_todo_item_insert()`)
		_, _ = fixture.pool.Exec(cleanupCtx, `DELETE FROM schedule_rules WHERE id=ANY($1::text[])`, createdRuleIDs)
		_, _ = fixture.pool.Exec(cleanupCtx, `DELETE FROM project_members WHERE project_id=ANY($1::text[])`, []string{projectID, ownerProjectID})
		_, _ = fixture.pool.Exec(cleanupCtx, `DELETE FROM projects WHERE id=ANY($1::text[])`, []string{projectID, ownerProjectID})
		_, _ = fixture.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=ANY($1::text[])`, []string{actorID, memberID})
	})

	todayResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if todayResponse.Code != http.StatusOK {
		t.Fatalf("create B Today status/body=%d/%s", todayResponse.Code, todayResponse.Body.String())
	}
	initialB := decodeToday(t, todayResponse)
	store := application.NewStore(fixture.pool)
	uow := application.NewUOW(fixture.pool, store.Auth, store.Project, store.Task, store.Schedule, store.Allocation)
	createRule := func(firstID string) *scheduleusecase.CreateRuleUseCase {
		calls := 0
		ids := func() string {
			calls++
			if calls == 1 {
				return firstID
			}
			return ulid.Make().String()
		}
		return scheduleusecase.NewCreateRuleUseCase(uow.Schedule, ids, store.Auth.Users, nil)
	}
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localNow := time.Now().In(location)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 10, 0, 0, 0, time.UTC)
	occurrenceDate := start.Format("2006-01-02")
	input := scheduleusecase.CreateRuleInput{
		Title: "B assigned today", ProjectID: projectID, AssigneeID: fixture.userID, Timezone: "Asia/Tokyo",
		LocalStart: start, LocalEnd: start.Add(time.Hour), Frequency: "once",
	}
	if _, err := createRule(ulid.Make().String()).Execute(ctx, actorID, input); !errors.Is(err, scheduleusecase.ErrTodoRegenerationRequired) {
		t.Fatalf("unconfirmed Schedule create error=%v, want regeneration confirmation", err)
	}
	var ruleCount int
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM schedule_rules rules JOIN schedule_rule_revisions revisions ON revisions.rule_id=rules.id WHERE revisions.title='B assigned today' AND rules.user_id=$1`, actorID).Scan(&ruleCount); err != nil || ruleCount != 0 {
		t.Fatalf("unconfirmed source rule count=%d err=%v, want no save", ruleCount, err)
	}
	var bRevision int32
	if err := fixture.pool.QueryRow(ctx, `SELECT revision FROM todos WHERE user_id=$1 AND todo_date=(now() AT TIME ZONE timezone)::date`, fixture.userID).Scan(&bRevision); err != nil || bRevision != initialB.Revision {
		t.Fatalf("unconfirmed B revision=%d err=%v, want %d", bRevision, err, initialB.Revision)
	}

	input.ConfirmTodoRegeneration = true
	createdRule, err := createRule(ulid.Make().String()).Execute(ctx, actorID, input)
	if err != nil {
		t.Fatalf("confirmed Schedule create: %v", err)
	}
	createdRuleIDs = append(createdRuleIDs, createdRule.ID)
	if err := fixture.pool.QueryRow(ctx, `SELECT revision FROM todos WHERE user_id=$1 AND todo_date=(now() AT TIME ZONE timezone)::date`, fixture.userID).Scan(&bRevision); err != nil || bRevision != initialB.Revision+1 {
		t.Fatalf("confirmed B revision=%d err=%v, want %d", bRevision, err, initialB.Revision+1)
	}
	var scheduleRows, actorLists int
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM todo_items i JOIN todos l ON l.id=i.todo_id WHERE l.user_id=$1 AND l.todo_date=(now() AT TIME ZONE l.timezone)::date AND i.schedule_rule_id=$2`, fixture.userID, createdRule.ID).Scan(&scheduleRows); err != nil || scheduleRows != 1 {
		t.Fatalf("confirmed B Schedule rows=%d err=%v, want 1", scheduleRows, err)
	}
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM todos WHERE user_id=$1 AND todo_date=(now() AT TIME ZONE timezone)::date`, actorID).Scan(&actorLists); err != nil || actorLists != 0 {
		t.Fatalf("actor A Today todos=%d err=%v, want none", actorLists, err)
	}

	assigneeC := memberID
	updated, err := scheduleusecase.NewUpdateOccurrenceUseCase(uow.Schedule, nil, ulid.Make().String).Execute(ctx, actorID, createdRule.ID, occurrenceDate,
		scheduleusecase.UpdateScheduleInput{AssigneeID: &assigneeC, ConfirmTodoRegeneration: true}, 1)
	if err != nil || updated.AssigneeID != memberID {
		t.Fatalf("confirmed B-to-C reassignment occurrence=%+v err=%v", updated, err)
	}
	var cRevision int32
	if err := fixture.pool.QueryRow(ctx, `SELECT revision FROM todos WHERE user_id=$1 AND todo_date=(now() AT TIME ZONE timezone)::date`, memberID).Scan(&cRevision); err != nil || cRevision != 2 {
		t.Fatalf("C revision=%d err=%v, want 2", cRevision, err)
	}
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM todo_items i JOIN todos l ON l.id=i.todo_id WHERE l.user_id=$1 AND l.todo_date=(now() AT TIME ZONE l.timezone)::date AND i.schedule_rule_id=$2`, fixture.userID, createdRule.ID).Scan(&scheduleRows); err != nil || scheduleRows != 0 {
		t.Fatalf("B Schedule rows after reassignment=%d err=%v, want 0", scheduleRows, err)
	}
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM todo_items i JOIN todos l ON l.id=i.todo_id WHERE l.user_id=$1 AND l.todo_date=(now() AT TIME ZONE l.timezone)::date AND i.schedule_rule_id=$2`, memberID, createdRule.ID).Scan(&scheduleRows); err != nil || scheduleRows != 1 {
		t.Fatalf("C Schedule rows after reassignment=%d err=%v, want 1", scheduleRows, err)
	}

	defaultInput := scheduleusecase.CreateRuleInput{
		Title: "Project owner default today", ProjectID: ownerProjectID, Timezone: "Asia/Tokyo",
		LocalStart: start.Add(2 * time.Hour), LocalEnd: start.Add(3 * time.Hour), Frequency: "once",
	}
	if _, err := createRule(ulid.Make().String()).Execute(ctx, actorID, defaultInput); !errors.Is(err, scheduleusecase.ErrTodoRegenerationRequired) {
		t.Fatalf("omitted-assignee unconfirmed create error=%v, want owner B confirmation", err)
	}
	defaultInput.ConfirmTodoRegeneration = true
	defaultRule, err := createRule(ulid.Make().String()).Execute(ctx, actorID, defaultInput)
	if err != nil || defaultRule.AssigneeID != fixture.userID {
		t.Fatalf("confirmed omitted-assignee rule=%+v err=%v, want project owner B", defaultRule, err)
	}
	createdRuleIDs = append(createdRuleIDs, defaultRule.ID)
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM todo_items i JOIN todos l ON l.id=i.todo_id WHERE l.user_id=$1 AND l.todo_date=(now() AT TIME ZONE l.timezone)::date AND i.schedule_rule_id=$2`, fixture.userID, defaultRule.ID).Scan(&scheduleRows); err != nil || scheduleRows != 1 {
		t.Fatalf("default-owner B Schedule rows=%d err=%v, want 1", scheduleRows, err)
	}

	if _, err := fixture.pool.Exec(ctx, `CREATE FUNCTION fail_schedule_todo_item_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind='schedule' THEN RAISE EXCEPTION 'injected Todo regeneration failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatalf("create regeneration failure function: %v", err)
	}
	if _, err := fixture.pool.Exec(ctx, `CREATE TRIGGER fail_schedule_todo_item_insert BEFORE INSERT ON todo_items FOR EACH ROW EXECUTE FUNCTION fail_schedule_todo_item_insert()`); err != nil {
		t.Fatalf("create regeneration failure trigger: %v", err)
	}
	if err := fixture.pool.QueryRow(ctx, `SELECT revision FROM todos WHERE user_id=$1 AND todo_date=(now() AT TIME ZONE timezone)::date`, fixture.userID).Scan(&bRevision); err != nil {
		t.Fatal(err)
	}
	failedRuleID := ulid.Make().String()
	failingInput := scheduleusecase.CreateRuleInput{
		Title: "Must roll back", ProjectID: projectID, AssigneeID: fixture.userID, Timezone: "Asia/Tokyo",
		LocalStart: start.Add(4 * time.Hour), LocalEnd: start.Add(5 * time.Hour), Frequency: "once", ConfirmTodoRegeneration: true,
	}
	if _, err := createRule(failedRuleID).Execute(ctx, actorID, failingInput); err == nil {
		t.Fatal("expected forced Todo regeneration failure")
	}
	if _, err := fixture.pool.Exec(ctx, `DROP TRIGGER fail_schedule_todo_item_insert ON todo_items`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `DROP FUNCTION fail_schedule_todo_item_insert()`); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM schedule_rules WHERE id=$1`, failedRuleID).Scan(&ruleCount); err != nil || ruleCount != 0 {
		t.Fatalf("failed source rule count=%d err=%v, want rollback", ruleCount, err)
	}
	var afterFailureRevision int32
	if err := fixture.pool.QueryRow(ctx, `SELECT revision FROM todos WHERE user_id=$1 AND todo_date=(now() AT TIME ZONE timezone)::date`, fixture.userID).Scan(&afterFailureRevision); err != nil || afterFailureRevision != bRevision {
		t.Fatalf("failed regeneration B revision=%d err=%v, want rollback to %d", afterFailureRevision, err, bRevision)
	}
}

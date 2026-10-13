//go:build integration

package todo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	metricusecase "github.com/Najah7/task2todaytodo/internal/application/metric/usecase"
	restapi "github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/oklog/ulid/v2"
)

func TestTodoRegenerationPicksUpWorkAddedAfterInitialTodo(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 30)
	initialResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial GET Todo status/body=%d/%s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decodeToday(t, initialResponse)
	if len(initial.Items) != 0 {
		t.Fatalf("initial Todo=%+v; want empty Todo before creating Inbox-equivalent work", initial.Items)
	}

	taskID := createProjectlessTodoTask(t, fixture, "Inbox childless Task", 30, "urgent")
	rule := createTodoScheduleRule(t, fixture, "Fixed meeting", 12, 13)
	t.Cleanup(func() { cleanupTodoRegenerationSources(t, fixture, []string{taskID}, []string{rule.ID}) })

	beforeRegeneration := getTodo(t, fixture)
	if beforeRegeneration.ID != initial.ID || len(beforeRegeneration.Items) != 1 || beforeRegeneration.Items[0].Kind != "schedule" || beforeRegeneration.Items[0].ScheduleRuleID != rule.ID {
		t.Fatalf("Todo after source creation=%+v; want Schedule but no newly created Task before explicit regeneration", beforeRegeneration)
	}
	if beforeRegeneration.Revision <= initial.Revision {
		t.Fatalf("Schedule sync revision=%d; want revision after initial Todo %d", beforeRegeneration.Revision, initial.Revision)
	}
	stale := fixture.request(t, http.MethodPost, "/today-todo:regenerate", "", initial.Revision)
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale regeneration status/body=%d/%s; want 412", stale.Code, stale.Body.String())
	}
	afterStale := getTodo(t, fixture)
	if afterStale.ID != beforeRegeneration.ID || afterStale.Revision != beforeRegeneration.Revision || !reflect.DeepEqual(afterStale.Items, beforeRegeneration.Items) {
		t.Fatalf("stale regeneration changed Todo: before=%+v after=%+v", beforeRegeneration, afterStale)
	}

	regenerated := fixture.request(t, http.MethodPost, "/today-todo:regenerate", "", beforeRegeneration.Revision)
	if regenerated.Code != http.StatusOK {
		t.Fatalf("regenerate Todo status/body=%d/%s", regenerated.Code, regenerated.Body.String())
	}
	regeneratedTodo := decodeToday(t, regenerated)
	if regeneratedTodo.ID != initial.ID || regeneratedTodo.Revision != beforeRegeneration.Revision+1 {
		t.Fatalf("regenerated Todo identity/revision=%s/%d; want %s/%d", regeneratedTodo.ID, regeneratedTodo.Revision, initial.ID, beforeRegeneration.Revision+1)
	}
	var taskFrame, scheduleFrame restapi.TodoItemResponse
	for _, item := range regeneratedTodo.Items {
		if item.Kind == "task" && item.TaskID == taskID {
			taskFrame = item
		}
		if item.Kind == "schedule" && item.ScheduleRuleID == rule.ID {
			scheduleFrame = item
		}
	}
	if taskFrame.ID == "" || taskFrame.PlannedMinutes == nil || *taskFrame.PlannedMinutes != 30 || taskFrame.ScheduledStart != nil || taskFrame.ScheduledEnd != nil {
		t.Fatalf("regeneration omitted whole childless Task or exposed its internal times: %+v", regeneratedTodo.Items)
	}
	if scheduleFrame.ID == "" || scheduleFrame.ScheduledStart == nil || scheduleFrame.ScheduledEnd == nil {
		t.Fatalf("regeneration omitted fixed Schedule or its public times: %+v", regeneratedTodo.Items)
	}
	assertPersistedPositiveWorkTime(t, fixture, taskFrame.ID)

	reloaded := getTodo(t, fixture)
	if reloaded.ID != regeneratedTodo.ID || reloaded.Revision != regeneratedTodo.Revision || !reflect.DeepEqual(reloaded.Items, regeneratedTodo.Items) {
		t.Fatalf("GET changed regenerated Todo: before=%+v after=%+v", regeneratedTodo, reloaded)
	}
}

func TestTodoRegenerationDestructivelyRebuildsRowsAndPreservesSourceCompletion(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 30)
	enableInitialActionItemGeneration(t, fixture)
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE users SET active_start_time=TIME '00:00',active_end_time=TIME '23:59' WHERE id=$1`, fixture.userID); err != nil {
		t.Fatal(err)
	}
	taskAID := createProjectlessTodoTask(t, fixture, "Completed Task source", 20, "urgent")
	removedTaskID := createProjectlessTodoTask(t, fixture, "Removed then regenerated Task", 25, "high")
	ruleA := createTodoScheduleRule(t, fixture, "Completed fixed Schedule", 12, 13)
	ruleB := createTodoScheduleRule(t, fixture, "Hidden fixed Schedule", 14, 15)
	t.Cleanup(func() {
		cleanupTodoRegenerationSources(t, fixture, []string{taskAID, removedTaskID}, []string{ruleA.ID, ruleB.ID})
	})

	initial := getTodo(t, fixture)
	if initial.ID == "" || !hasTodoSource(initial.Items, "action_item", fixture.itemID) || !hasTodoSource(initial.Items, "task", taskAID) || !hasTodoSource(initial.Items, "task", removedTaskID) || !hasTodoSource(initial.Items, "schedule", ruleA.ID) || !hasTodoSource(initial.Items, "schedule", ruleB.ID) {
		t.Fatalf("initial Todo lacks eligible whole work and fixed Schedule sources: %+v", initial.Items)
	}
	assertTaskHasPositiveInternalTime(t, fixture, initial.Items, taskAID)

	completedAction := completeTodoSource(t, fixture, initial, "action_item", fixture.itemID, 17)
	completedTask := completeTodoSource(t, fixture, completedAction, "task", taskAID, 12)
	completedSchedule := completeTodoSource(t, fixture, completedTask, "schedule", ruleA.ID, 0)
	hiddenScheduleID := todoItemForSource(t, completedSchedule.Items, "schedule", ruleB.ID).ID
	hide := fixture.request(t, http.MethodPost, "/today-todo/schedules/"+hiddenScheduleID+":hide", "", completedSchedule.Revision)
	if hide.Code != http.StatusOK {
		t.Fatalf("hide Schedule status/body=%d/%s", hide.Code, hide.Body.String())
	}
	current := decodeToday(t, hide)
	removedFrame := todoItemForSource(t, current.Items, "task", removedTaskID)
	remove := fixture.request(t, http.MethodPost, "/today-todo/items/"+removedFrame.ID+":remove", "", current.Revision)
	if remove.Code != http.StatusOK {
		t.Fatalf("remove selected work status/body=%d/%s", remove.Code, remove.Body.String())
	}
	current = decodeToday(t, remove)

	manualTaskID := createProjectlessTodoTask(t, fixture, "Manual Todo addition", 15, "low")
	addManual := fixture.request(t, http.MethodPost, "/today-todo/tasks/"+manualTaskID, "", current.Revision)
	if addManual.Code != http.StatusOK {
		t.Fatalf("manual Task add status/body=%d/%s", addManual.Code, addManual.Body.String())
	}
	current = decodeToday(t, addManual)
	manualFrame := todoItemForSource(t, current.Items, "task", manualTaskID)
	if manualFrame.ScheduledStart != nil || manualFrame.ScheduledEnd != nil {
		t.Fatalf("manual work row exposed internal time: %+v", manualFrame)
	}
	assertPersistedNullWorkTime(t, fixture, manualFrame.ID)

	itemIDs := make([]string, 0, len(current.Items))
	for index := len(current.Items) - 1; index >= 0; index-- {
		itemIDs = append(itemIDs, current.Items[index].ID)
	}
	reorderBody, err := json.Marshal(struct {
		ItemIDs []string `json:"item_ids"`
	}{ItemIDs: itemIDs})
	if err != nil {
		t.Fatal(err)
	}
	reordered := fixture.request(t, http.MethodPost, "/today-todo:reorder", string(reorderBody), current.Revision)
	if reordered.Code != http.StatusOK {
		t.Fatalf("reorder before regeneration status/body=%d/%s", reordered.Code, reordered.Body.String())
	}
	current = decodeToday(t, reordered)
	if !reflect.DeepEqual(todoItemIDs(current.Items), itemIDs) {
		t.Fatalf("pre-regeneration manual order=%v, want %v", todoItemIDs(current.Items), itemIDs)
	}
	if len(current.HiddenSchedules) != 1 || current.HiddenSchedules[0].ID != hiddenScheduleID {
		t.Fatalf("pre-regeneration hidden Schedule=%+v; want %s", current.HiddenSchedules, hiddenScheduleID)
	}
	if current.Revision == initial.Revision {
		t.Fatalf("mutated Todo revision did not advance from %d", initial.Revision)
	}
	staleRevision := initial.Revision
	preResetRows := readPersistedTodoRows(t, fixture, current.ID)
	preResetMetrics := todoMetricsForDate(t, fixture, current.Date)
	if preResetMetrics.PlannedItems != 4 || preResetMetrics.CompletedItems != 3 {
		t.Fatalf("pre-reset metrics=%+v; want 4 planned / 3 completed", preResetMetrics)
	}

	stale := fixture.request(t, http.MethodPost, "/today-todo:regenerate", "", staleRevision)
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale regeneration status/body=%d/%s; want 412", stale.Code, stale.Body.String())
	}
	afterStale := getTodo(t, fixture)
	if afterStale.ID != current.ID || afterStale.Revision != current.Revision || !reflect.DeepEqual(afterStale.Items, current.Items) || !reflect.DeepEqual(afterStale.HiddenSchedules, current.HiddenSchedules) || !reflect.DeepEqual(readPersistedTodoRows(t, fixture, current.ID), preResetRows) {
		t.Fatalf("stale regeneration wrote Todo state: before=%+v after=%+v", current, afterStale)
	}

	triggerName, functionName := installTodoInsertFailure(t, fixture, current.ID)
	failed := fixture.request(t, http.MethodPost, "/today-todo:regenerate", "", current.Revision)
	if failed.Code != http.StatusInternalServerError {
		dropTodoInsertFailure(t, fixture, triggerName, functionName)
		t.Fatalf("forced regeneration insert failure status/body=%d/%s; want 500", failed.Code, failed.Body.String())
	}
	dropTodoInsertFailure(t, fixture, triggerName, functionName)
	afterRollback := getTodo(t, fixture)
	if afterRollback.ID != current.ID || afterRollback.Revision != current.Revision || !reflect.DeepEqual(afterRollback.Items, current.Items) || !reflect.DeepEqual(afterRollback.HiddenSchedules, current.HiddenSchedules) || !reflect.DeepEqual(readPersistedTodoRows(t, fixture, current.ID), preResetRows) {
		t.Fatalf("failed regeneration did not roll back all Todo rows: before=%+v after=%+v", current, afterRollback)
	}
	assertSourceCompletions(t, fixture, taskAID, fixture.taskID, fixture.itemID, ruleA.ID, ruleB.ID)

	regenerated := fixture.request(t, http.MethodPost, "/today-todo:regenerate", "", current.Revision)
	if regenerated.Code != http.StatusOK {
		t.Fatalf("regenerate Todo status/body=%d/%s", regenerated.Code, regenerated.Body.String())
	}
	regeneratedTodo := decodeToday(t, regenerated)
	if regeneratedTodo.ID != current.ID || regeneratedTodo.Revision != current.Revision+1 {
		t.Fatalf("regenerated Todo identity/revision=%s/%d; want %s/%d", regeneratedTodo.ID, regeneratedTodo.Revision, current.ID, current.Revision+1)
	}
	if len(regeneratedTodo.HiddenSchedules) != 0 {
		t.Fatalf("regeneration retained hidden Schedule rows: %+v", regeneratedTodo.HiddenSchedules)
	}
	if hasTodoSource(regeneratedTodo.Items, "task", taskAID) || hasTodoSource(regeneratedTodo.Items, "action_item", fixture.itemID) {
		t.Fatalf("regeneration reselected completed work: %+v", regeneratedTodo.Items)
	}
	if !hasTodoSource(regeneratedTodo.Items, "task", removedTaskID) || !hasTodoSource(regeneratedTodo.Items, "task", manualTaskID) {
		t.Fatalf("regeneration did not replace removed/manual selection with current whole sources: %+v", regeneratedTodo.Items)
	}
	completedScheduleRow := todoItemForSource(t, regeneratedTodo.Items, "schedule", ruleA.ID)
	if !completedScheduleRow.Completed {
		t.Fatalf("completed Schedule source did not reappear completed after regeneration: %+v", completedScheduleRow)
	}
	visibleHiddenSource := todoItemForSource(t, regeneratedTodo.Items, "schedule", ruleB.ID)
	if visibleHiddenSource.Hidden || visibleHiddenSource.Completed {
		t.Fatalf("hidden uncompleted Schedule did not return visible and unchanged: %+v", visibleHiddenSource)
	}
	newManualSourceRow := todoItemForSource(t, regeneratedTodo.Items, "task", manualTaskID)
	if newManualSourceRow.ID == manualFrame.ID {
		t.Fatalf("manual Todo row ID survived destructive regeneration: %s", manualFrame.ID)
	}
	assertTaskHasPositiveInternalTime(t, fixture, regeneratedTodo.Items, removedTaskID)
	assertTaskHasPositiveInternalTime(t, fixture, regeneratedTodo.Items, manualTaskID)
	assertSourceCompletions(t, fixture, taskAID, fixture.taskID, fixture.itemID, ruleA.ID, ruleB.ID)
	afterResetMetrics := todoMetricsForDate(t, fixture, regeneratedTodo.Date)
	if afterResetMetrics.PlannedItems != 4 || afterResetMetrics.CompletedItems != 1 {
		t.Fatalf("post-reset metrics=%+v; want 4 planned / 1 completed from current Todo rows", afterResetMetrics)
	}
	for index, item := range regeneratedTodo.Items {
		if item.Position != int32(index) {
			t.Fatalf("regenerated display positions are not fresh and contiguous: %+v", regeneratedTodo.Items)
		}
	}

	reloaded := getTodo(t, fixture)
	if reloaded.ID != regeneratedTodo.ID || reloaded.Revision != regeneratedTodo.Revision || !reflect.DeepEqual(reloaded.Items, regeneratedTodo.Items) {
		t.Fatalf("GET changed regenerated Todo: before=%+v after=%+v", regeneratedTodo, reloaded)
	}
}

func createProjectlessTodoTask(t *testing.T, fixture *todoFixture, title string, minutes int, priority string) string {
	t.Helper()
	response := fixture.request(t, http.MethodPost, "/tasks", fmt.Sprintf(`{"title":%q,"manual_estimated_minutes":%d,"priority":%q}`, title, minutes, priority), 0)
	if response.Code != http.StatusCreated {
		t.Fatalf("create projectless Task %q status/body=%d/%s", title, response.Code, response.Body.String())
	}
	var task restapi.TaskResponse
	if err := json.Unmarshal(response.Body.Bytes(), &task); err != nil {
		t.Fatalf("decode created Task: %v; body=%s", err, response.Body.String())
	}
	return task.ID
}

func createTodoScheduleRule(t *testing.T, fixture *todoFixture, title string, startHour, endHour int) restapi.ScheduleRuleResponse {
	t.Helper()
	start := time.Now().UTC()
	start = time.Date(start.Year(), start.Month(), start.Day(), startHour, 0, 0, 0, time.UTC)
	end := time.Date(start.Year(), start.Month(), start.Day(), endHour, 0, 0, 0, time.UTC)
	response := fixture.request(t, http.MethodPost, "/schedule-rules", fmt.Sprintf(`{"title":%q,"timezone":"UTC","local_start":%q,"local_end":%q,"all_day":false,"frequency":"once","interval":0}`,
		title, start.Format("2006-01-02T15:04:05"), end.Format("2006-01-02T15:04:05")), 0)
	if response.Code != http.StatusCreated {
		t.Fatalf("create Schedule %q status/body=%d/%s", title, response.Code, response.Body.String())
	}
	var rule restapi.ScheduleRuleResponse
	if err := json.Unmarshal(response.Body.Bytes(), &rule); err != nil {
		t.Fatalf("decode created Schedule rule: %v; body=%s", err, response.Body.String())
	}
	return rule
}

func cleanupTodoRegenerationSources(t *testing.T, fixture *todoFixture, taskIDs, ruleIDs []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if len(ruleIDs) > 0 {
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM schedule_revisions WHERE schedule_id IN (SELECT id FROM schedules WHERE rule_id=ANY($1::text[]))`, ruleIDs)
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM schedule_exceptions WHERE rule_id=ANY($1::text[])`, ruleIDs)
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM schedules WHERE rule_id=ANY($1::text[])`, ruleIDs)
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM schedule_rule_revisions WHERE rule_id=ANY($1::text[])`, ruleIDs)
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM schedule_rules WHERE id=ANY($1::text[])`, ruleIDs)
	}
	if len(taskIDs) > 0 {
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=ANY($1::text[])`, taskIDs)
	}
}

func hasTodoSource(items []restapi.TodoItemResponse, kind, sourceID string) bool {
	for _, item := range items {
		if item.Kind != kind {
			continue
		}
		if (kind == "task" && item.TaskID == sourceID) || (kind == "action_item" && item.ActionItemID == sourceID) || (kind == "schedule" && item.ScheduleRuleID == sourceID) {
			return true
		}
	}
	return false
}

func todoItemForSource(t *testing.T, items []restapi.TodoItemResponse, kind, sourceID string) restapi.TodoItemResponse {
	t.Helper()
	for _, item := range items {
		if item.Kind != kind {
			continue
		}
		if (kind == "task" && item.TaskID == sourceID) || (kind == "action_item" && item.ActionItemID == sourceID) || (kind == "schedule" && item.ScheduleRuleID == sourceID) {
			return item
		}
	}
	t.Fatalf("Todo source row %s/%s missing from %+v", kind, sourceID, items)
	return restapi.TodoItemResponse{}
}

func completeTodoSource(t *testing.T, fixture *todoFixture, todo restapi.TodoTodayResponse, kind, sourceID string, actualMinutes int) restapi.TodoTodayResponse {
	t.Helper()
	item := todoItemForSource(t, todo.Items, kind, sourceID)
	body := ""
	if kind != "schedule" {
		body = fmt.Sprintf(`{"actual_minutes":%d}`, actualMinutes)
	}
	response := fixture.request(t, http.MethodPost, "/today-todo/items/"+item.ID+":complete", body, todo.Revision)
	if response.Code != http.StatusOK {
		t.Fatalf("complete %s source %s status/body=%d/%s", kind, sourceID, response.Code, response.Body.String())
	}
	return decodeToday(t, response)
}

func assertTaskHasPositiveInternalTime(t *testing.T, fixture *todoFixture, items []restapi.TodoItemResponse, taskID string) {
	t.Helper()
	item := todoItemForSource(t, items, "task", taskID)
	if item.ScheduledStart != nil || item.ScheduledEnd != nil {
		t.Fatalf("work timestamps leaked through API: %+v", item)
	}
	assertPersistedPositiveWorkTime(t, fixture, item.ID)
}

func assertPersistedPositiveWorkTime(t *testing.T, fixture *todoFixture, itemID string) {
	t.Helper()
	var start, end sql.NullTime
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, itemID).Scan(&start, &end); err != nil {
		t.Fatal(err)
	}
	if !start.Valid || !end.Valid || !end.Time.After(start.Time) {
		t.Fatalf("persisted work interval=%+v/%+v; want positive internal time", start, end)
	}
}

func assertPersistedNullWorkTime(t *testing.T, fixture *todoFixture, itemID string) {
	t.Helper()
	var start, end sql.NullTime
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, itemID).Scan(&start, &end); err != nil {
		t.Fatal(err)
	}
	if start.Valid || end.Valid {
		t.Fatalf("manual work interval=%+v/%+v; want NULL internal times", start, end)
	}
}

type persistedTodoRow struct {
	ID         string
	Kind       string
	ActionItem string
	Task       string
	Schedule   string
	Completed  bool
	Hidden     bool
	Position   int32
	Start      sql.NullTime
	End        sql.NullTime
}

func readPersistedTodoRows(t *testing.T, fixture *todoFixture, todoID string) []persistedTodoRow {
	t.Helper()
	rows, err := fixture.pool.Query(t.Context(), `SELECT id,kind,COALESCE(action_item_id,''),COALESCE(task_id,''),COALESCE(schedule_rule_id,''),completed,hidden,position,scheduled_start,scheduled_end FROM todo_items WHERE todo_id=$1 ORDER BY position,id`, todoID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := make([]persistedTodoRow, 0)
	for rows.Next() {
		var row persistedTodoRow
		if err := rows.Scan(&row.ID, &row.Kind, &row.ActionItem, &row.Task, &row.Schedule, &row.Completed, &row.Hidden, &row.Position, &row.Start, &row.End); err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func installTodoInsertFailure(t *testing.T, fixture *todoFixture, todoID string) (string, string) {
	t.Helper()
	suffix := strings.ToLower(ulid.Make().String())
	triggerName, functionName := "fail_todo_regeneration_"+suffix, "fail_todo_regeneration_fn_"+suffix
	functionSQL := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $fn$ BEGIN IF NEW.todo_id = '%s' THEN RAISE EXCEPTION 'forced todo regeneration insert failure'; END IF; RETURN NEW; END $fn$`, functionName, todoID)
	if _, err := fixture.pool.Exec(t.Context(), functionSQL); err != nil {
		t.Fatalf("create scoped Todo insert failure function: %v", err)
	}
	triggerSQL := fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON todo_items FOR EACH ROW EXECUTE FUNCTION %s()`, triggerName, functionName)
	if _, err := fixture.pool.Exec(t.Context(), triggerSQL); err != nil {
		_, _ = fixture.pool.Exec(t.Context(), fmt.Sprintf(`DROP FUNCTION %s()`, functionName))
		t.Fatalf("create scoped Todo insert failure trigger: %v", err)
	}
	t.Cleanup(func() { dropTodoInsertFailure(t, fixture, triggerName, functionName) })
	return triggerName, functionName
}

func dropTodoInsertFailure(t *testing.T, fixture *todoFixture, triggerName, functionName string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = fixture.pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON todo_items`, triggerName))
	_, _ = fixture.pool.Exec(ctx, fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, functionName))
}

func assertSourceCompletions(t *testing.T, fixture *todoFixture, directTaskID, actionTaskID, actionItemID, completedScheduleRule, hiddenScheduleRule string) {
	t.Helper()
	var directStatus string
	var directActual sql.NullInt32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT status,actual_minutes FROM tasks WHERE id=$1`, directTaskID).Scan(&directStatus, &directActual); err != nil {
		t.Fatal(err)
	}
	if directStatus != "done" || !directActual.Valid || directActual.Int32 != 12 {
		t.Fatalf("completed Task source=%s/%+v; want done/12", directStatus, directActual)
	}
	var actionTaskStatus string
	var actionCompleted bool
	var actionActual sql.NullInt32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT t.status,ai.completed,ai.actual_minutes FROM tasks t JOIN action_items ai ON ai.task_id=t.id WHERE t.id=$1 AND ai.id=$2`, actionTaskID, actionItemID).Scan(&actionTaskStatus, &actionCompleted, &actionActual); err != nil {
		t.Fatal(err)
	}
	if actionTaskStatus != "done" || !actionCompleted || !actionActual.Valid || actionActual.Int32 != 17 {
		t.Fatalf("completed ActionItem source=%s/%t/%+v; want done/true/17", actionTaskStatus, actionCompleted, actionActual)
	}
	var scheduleACompleted, scheduleBCompleted bool
	if err := fixture.pool.QueryRow(t.Context(), `SELECT completed FROM schedules WHERE rule_id=$1`, completedScheduleRule).Scan(&scheduleACompleted); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT completed FROM schedules WHERE rule_id=$1`, hiddenScheduleRule).Scan(&scheduleBCompleted); err != nil {
		t.Fatal(err)
	}
	if !scheduleACompleted || scheduleBCompleted {
		t.Fatalf("Schedule completion sources=%t/%t; want completed/active", scheduleACompleted, scheduleBCompleted)
	}
}

func todoItemIDs(items []restapi.TodoItemResponse) []string {
	ids := make([]string, len(items))
	for index, item := range items {
		ids[index] = item.ID
	}
	return ids
}

func todoMetricsForDate(t *testing.T, fixture *todoFixture, date string) struct{ PlannedItems, CompletedItems int32 } {
	t.Helper()
	completion, err := metricusecase.NewTodoCompletionUseCase(application.NewStore(fixture.pool).Metric).Execute(t.Context(), fixture.userID, 7)
	if err != nil {
		t.Fatalf("read Todo completion metrics: %v", err)
	}
	for _, day := range completion.Days {
		if day.Date == date {
			return struct{ PlannedItems, CompletedItems int32 }{PlannedItems: day.PlannedItems, CompletedItems: day.CompletedItems}
		}
	}
	t.Fatalf("metrics did not include Todo date %s: %+v", date, completion.Days)
	return struct{ PlannedItems, CompletedItems int32 }{}
}

//go:build integration

package todo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	restapi "github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/oklog/ulid/v2"
)

func TestTodoProjectlessTaskAndActionItemUseOnlyAllocationComplement(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 25)
	date := time.Now().UTC().Format("2006-01-02")
	day, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatal(err)
	}
	projectID, directTaskID, childTaskID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO projects(id,user_id,title,changed_by) VALUES($1,$2,'Allocation blocker',$2)`, projectID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=ANY($1::text[])`, []string{directTaskID, childTaskID})
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
	})
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,priority,status,changed_by) VALUES($1,$2,$2,'Projectless ActionItem parent','high','open',$2)`, childTaskID, fixture.userID); err != nil {
		t.Fatalf("insert ActionItem parent: %v", err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'Projectless direct Task',25,'urgent','open',$2)`, directTaskID, fixture.userID); err != nil {
		t.Fatalf("insert projectless Task: %v", err)
	}
	childItemID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO action_items(id,task_id,title,position,estimated_minutes,priority) VALUES($1,$2,'Projectless ActionItem',0,25,'high')`, childItemID, childTaskID); err != nil {
		t.Fatalf("insert projectless ActionItem: %v", err)
	}
	if err := insertAllocationBlock(t, fixture, projectID, day, 7, 12); err != nil {
		t.Fatal(err)
	}
	if err := insertAllocationBlock(t, fixture, projectID, day, 13, 23); err != nil {
		t.Fatal(err)
	}

	response := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if response.Code != http.StatusOK {
		t.Fatalf("GET projectless allocation plan status/body=%d/%s", response.Code, response.Body.String())
	}
	today := decodeToday(t, response)
	var direct, action *restTodoItem
	for i := range today.Items {
		item := today.Items[i]
		switch {
		case item.Kind == "task" && item.TaskID == directTaskID:
			direct = persistedTodoItemTime(t, fixture, item)
		case item.Kind == "action_item" && item.ActionItemID == childItemID:
			action = persistedTodoItemTime(t, fixture, item)
		}
	}
	if direct == nil || action == nil || direct.Start == nil || direct.End == nil || action.Start == nil || action.End == nil {
		t.Fatalf("projectless work rows=%+v, want both Task and ActionItem scheduled in the free complement", today.Items)
	}
	for _, item := range today.Items {
		if item.Kind != "schedule" && (item.ScheduledStart != nil || item.ScheduledEnd != nil) {
			t.Fatalf("work item exposed internal times in API response: %+v", item)
		}
	}
	for _, row := range []*restTodoItem{direct, action} {
		if row.Start.Before(day.Add(12*time.Hour)) || row.End.After(day.Add(13*time.Hour)) {
			t.Fatalf("projectless row %s scheduled inside Project allocation: %s-%s", row.ID, row.Start, row.End)
		}
	}
	if direct.Start.Before(*action.End) && action.Start.Before(*direct.End) {
		t.Fatalf("projectless Task and ActionItem overlap: Task=%s-%s ActionItem=%s-%s", direct.Start, direct.End, action.Start, action.End)
	}
	first, second := direct, action
	if action.Start.Before(*direct.Start) {
		first, second = action, direct
	}
	if second.Start.Sub(*first.End) < 5*time.Minute {
		t.Fatalf("projectless rows violate minimum gap: first=%s-%s second=%s-%s", first.Start, first.End, second.Start, second.End)
	}
}

func TestTodoAllDayScheduleAddPreservesSelectedWorkAndInternalTimes(t *testing.T) {
	fixture := newTodoFixture(t, "Asia/Tokyo", 45)
	enableInitialActionItemGeneration(t, fixture)
	taskID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'Selected direct Task',20,'urgent','open',$2)`, taskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, taskID)
	})

	initialResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial GET today status/body=%d/%s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decodeToday(t, initialResponse)
	if len(initial.Items) != 2 {
		t.Fatalf("initial selected work=%+v; want ActionItem and direct Task", initial.Items)
	}
	workOrder := make([]string, 0, 2)
	workTimes := make(map[string][2]time.Time, 2)
	for _, item := range initial.Items {
		if item.Kind == "schedule" || item.ScheduledStart != nil || item.ScheduledEnd != nil {
			t.Fatalf("initial work row leaked fixed time or was not work: %+v", item)
		}
		var start, end time.Time
		if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, item.ID).Scan(&start, &end); err != nil {
			t.Fatal(err)
		}
		if !end.After(start) {
			t.Fatalf("selected work %s has no positive internal interval: %s-%s", item.ID, start, end)
		}
		workOrder = append(workOrder, item.ID)
		workTimes[item.ID] = [2]time.Time{start, end}
	}

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localToday := time.Now().In(location)
	start := time.Date(localToday.Year(), localToday.Month(), localToday.Day(), 0, 0, 0, 0, location)
	body := fmt.Sprintf(`{"title":"All-day blocker","timezone":"Asia/Tokyo","local_start":%q,"local_end":%q,"all_day":true,"frequency":"once","interval":0}`,
		start.Format("2006-01-02T15:04:05"), start.AddDate(0, 0, 1).Format("2006-01-02T15:04:05"))
	created := fixture.request(t, http.MethodPost, "/schedule-rules", body, 0)
	if created.Code != http.StatusCreated {
		t.Fatalf("create all-day Schedule status/body=%d/%s", created.Code, created.Body.String())
	}
	updatedResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if updatedResponse.Code != http.StatusOK {
		t.Fatalf("GET Todo after all-day Schedule status/body=%d/%s", updatedResponse.Code, updatedResponse.Body.String())
	}
	updated := decodeToday(t, updatedResponse)
	if updated.ID != initial.ID || len(updated.Items) != 3 {
		t.Fatalf("Todo after all-day Schedule=%+v; want same Todo with Schedule and both selected work rows", updated)
	}
	var schedule restapi.TodoItemResponse
	updatedWorkOrder := make([]string, 0, 2)
	var schedulePosition int32
	for _, item := range updated.Items {
		if item.Kind == "schedule" {
			schedule = item
			if item.ScheduledStart == nil || item.ScheduledEnd == nil {
				t.Fatalf("fixed Schedule lost public actual times: %+v", item)
			}
			if err := fixture.pool.QueryRow(t.Context(), `SELECT position FROM todo_items WHERE id=$1`, item.ID).Scan(&schedulePosition); err != nil {
				t.Fatal(err)
			}
		} else {
			if item.ScheduledStart != nil || item.ScheduledEnd != nil {
				t.Fatalf("work row exposed its internal times after Schedule insertion: %+v", item)
			}
			updatedWorkOrder = append(updatedWorkOrder, item.ID)
		}
	}
	if schedule.ID == "" || !reflect.DeepEqual(updatedWorkOrder, workOrder) {
		t.Fatalf("Schedule insertion changed selected work/order: schedule=%+v before=%v after=%v", schedule, workOrder, updatedWorkOrder)
	}
	if schedulePosition != 0 || schedule.ScheduledStart == nil || schedule.ScheduledEnd == nil {
		t.Fatalf("all-day Schedule position/times=%d/%v/%v; want inserted before first work row with actual fixed boundaries", schedulePosition, schedule.ScheduledStart, schedule.ScheduledEnd)
	}
	parsedStart, err := time.Parse(time.RFC3339Nano, *schedule.ScheduledStart)
	if err != nil {
		t.Fatal(err)
	}
	parsedEnd, err := time.Parse(time.RFC3339Nano, *schedule.ScheduledEnd)
	if err != nil {
		t.Fatal(err)
	}
	if !parsedStart.Equal(start.UTC()) || !parsedEnd.Equal(start.AddDate(0, 0, 1).UTC()) {
		t.Fatalf("all-day Schedule time=%s-%s; want exact fixed interval %s-%s", parsedStart, parsedEnd, start.UTC(), start.AddDate(0, 0, 1).UTC())
	}
	for _, id := range workOrder {
		var position int32
		var gotStart, gotEnd time.Time
		if err := fixture.pool.QueryRow(t.Context(), `SELECT position,scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, id).Scan(&position, &gotStart, &gotEnd); err != nil {
			t.Fatal(err)
		}
		if position < schedulePosition || !gotStart.Equal(workTimes[id][0]) || !gotEnd.Equal(workTimes[id][1]) {
			t.Fatalf("all-day Schedule changed work row %s position/time: %d %s-%s; original %s-%s", id, position, gotStart, gotEnd, workTimes[id][0], workTimes[id][1])
		}
	}
}

func TestTodoScheduleInsertionUpdateAndRestorePreserveWorkOrderAndTimes(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 20)
	enableInitialActionItemGeneration(t, fixture)
	taskID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'Second selected source',15,'urgent','open',$2)`, taskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, taskID)
	})

	initialResponse := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial GET today status/body=%d/%s", initialResponse.Code, initialResponse.Body.String())
	}
	initial := decodeToday(t, initialResponse)
	workIDs, workTimes := todoWorkState(t, fixture, initial)
	if len(workIDs) < 2 {
		t.Fatalf("selected work=%+v; need at least two scheduled sources to exercise anchor changes", initial.Items)
	}
	firstStart := workTimes[workIDs[0]][0]
	secondStart := workTimes[workIDs[1]][0]
	if !firstStart.Before(secondStart) {
		t.Fatalf("selected work not in scheduled order: %s then %s", firstStart, secondStart)
	}

	createStart := firstStart.Add(-time.Minute)
	created := fixture.request(t, http.MethodPost, "/schedule-rules", fmt.Sprintf(`{"title":"Anchor schedule","timezone":"UTC","local_start":%q,"local_end":%q,"all_day":false,"frequency":"once","interval":0}`,
		createStart.UTC().Format("2006-01-02T15:04:05"), createStart.Add(15*time.Minute).UTC().Format("2006-01-02T15:04:05")), 0)
	if created.Code != http.StatusCreated {
		t.Fatalf("create Schedule status/body=%d/%s", created.Code, created.Body.String())
	}
	var rule restapi.ScheduleRuleResponse
	if err := json.Unmarshal(created.Body.Bytes(), &rule); err != nil {
		t.Fatalf("decode Schedule rule: %v; body=%s", err, created.Body.String())
	}
	occurrenceDate := createStart.UTC().Format("2006-01-02")
	todoAfterCreate := getTodo(t, fixture)
	scheduleItem := scheduleTodoItem(t, todoAfterCreate, rule.ID)
	assertWorkState(t, fixture, todoAfterCreate, workIDs, workTimes)
	if got, anchor := todoPosition(t, fixture, scheduleItem.ID), todoPosition(t, fixture, workIDs[0]); got != anchor-1 {
		t.Fatalf("new Schedule position=%d; want insertion immediately before first work at %d", got, anchor-1)
	}

	moveStart := firstStart.Add(time.Minute)
	occurrence := getScheduleOccurrence(t, fixture, rule.ID, occurrenceDate)
	update := fixture.request(t, http.MethodPatch, "/schedules/"+rule.ID+"/"+occurrenceDate,
		fmt.Sprintf(`{"start_at":%q,"end_at":%q}`, moveStart.Format(time.RFC3339Nano), moveStart.Add(15*time.Minute).Format(time.RFC3339Nano)), occurrence.Revision)
	if update.Code != http.StatusOK {
		t.Fatalf("update Schedule start status/body=%d/%s", update.Code, update.Body.String())
	}
	todoAfterUpdate := getTodo(t, fixture)
	scheduleItem = scheduleTodoItem(t, todoAfterUpdate, rule.ID)
	assertWorkState(t, fixture, todoAfterUpdate, workIDs, workTimes)
	if got, anchor := todoPosition(t, fixture, scheduleItem.ID), todoPosition(t, fixture, workIDs[1]); got != anchor-1 {
		t.Fatalf("updated Schedule position=%d; want insertion immediately before second work at %d", got, anchor-1)
	}
	if scheduleItem.ScheduledStart == nil || scheduleItem.ScheduledEnd == nil {
		t.Fatalf("Schedule lost actual public times after update: %+v", scheduleItem)
	}

	for _, path := range []string{"/today-todo/schedules/" + scheduleItem.ID + ":hide", "/today-todo/schedules/" + scheduleItem.ID + ":restore"} {
		response := fixture.request(t, http.MethodPost, path, "", todoAfterUpdate.Revision)
		if response.Code != http.StatusOK {
			t.Fatalf("Todo Schedule state %s status/body=%d/%s", path, response.Code, response.Body.String())
		}
		todoAfterUpdate = decodeToday(t, response)
		if strings.HasSuffix(path, ":restore") {
			scheduleItem = scheduleTodoItem(t, todoAfterUpdate, rule.ID)
			assertWorkState(t, fixture, todoAfterUpdate, workIDs, workTimes)
			if got, anchor := todoPosition(t, fixture, scheduleItem.ID), todoPosition(t, fixture, workIDs[1]); got != anchor-1 {
				t.Fatalf("Todo restore position=%d; want insertion immediately before second work at %d", got, anchor-1)
			}
		}
	}

	occurrence = getScheduleOccurrence(t, fixture, rule.ID, occurrenceDate)
	skipped := fixture.request(t, http.MethodPost, "/schedules/"+rule.ID+"/"+occurrenceDate+":skip", "", occurrence.Revision)
	if skipped.Code != http.StatusNoContent {
		t.Fatalf("skip Schedule occurrence status/body=%d/%s", skipped.Code, skipped.Body.String())
	}
	var skippedRevision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT revision FROM schedules WHERE rule_id=$1 AND occurrence_date=$2`, rule.ID, occurrenceDate).Scan(&skippedRevision); err != nil {
		t.Fatalf("read skipped Schedule revision: %v", err)
	}
	restored := fixture.request(t, http.MethodPost, "/schedules/"+rule.ID+"/"+occurrenceDate+":restore", "", skippedRevision)
	if restored.Code != http.StatusNoContent {
		t.Fatalf("restore skipped Schedule status/body=%d/%s", restored.Code, restored.Body.String())
	}
	todoAfterRestore := getTodo(t, fixture)
	scheduleItem = scheduleTodoItem(t, todoAfterRestore, rule.ID)
	assertWorkState(t, fixture, todoAfterRestore, workIDs, workTimes)
	if got, anchor := todoPosition(t, fixture, scheduleItem.ID), todoPosition(t, fixture, workIDs[1]); got != anchor-1 {
		t.Fatalf("source restore position=%d; want insertion immediately before second work at %d", got, anchor-1)
	}

	occurrence = getScheduleOccurrence(t, fixture, rule.ID, occurrenceDate)
	deleted := fixture.request(t, http.MethodDelete, "/schedules/"+rule.ID+"/"+occurrenceDate, "", occurrence.Revision)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete Schedule occurrence status/body=%d/%s", deleted.Code, deleted.Body.String())
	}
	todoAfterDelete := getTodo(t, fixture)
	assertWorkState(t, fixture, todoAfterDelete, workIDs, workTimes)
	for _, item := range append(todoAfterDelete.Items, todoAfterDelete.HiddenSchedules...) {
		if item.ScheduleRuleID == rule.ID {
			t.Fatalf("deleted Schedule remained in Todo: %+v", item)
		}
	}
}

func todoWorkState(t *testing.T, fixture *todoFixture, todo restapi.TodoTodayResponse) ([]string, map[string][2]time.Time) {
	t.Helper()
	ids := make([]string, 0, len(todo.Items))
	times := make(map[string][2]time.Time, len(todo.Items))
	type workRow struct {
		id       string
		position int32
		start    time.Time
		end      time.Time
	}
	rows := make([]workRow, 0, len(todo.Items))
	for _, item := range todo.Items {
		if item.Kind == "schedule" {
			continue
		}
		if item.ScheduledStart != nil || item.ScheduledEnd != nil {
			t.Fatalf("work timestamps visible in Todo API: %+v", item)
		}
		var row workRow
		row.id = item.ID
		if err := fixture.pool.QueryRow(t.Context(), `SELECT position,scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, item.ID).Scan(&row.position, &row.start, &row.end); err != nil {
			t.Fatalf("read work row %s: %v", item.ID, err)
		}
		if !row.end.After(row.start) {
			t.Fatalf("work row has no internal positive interval: %+v", row)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].position < rows[j].position })
	for _, row := range rows {
		ids = append(ids, row.id)
		times[row.id] = [2]time.Time{row.start, row.end}
	}
	return ids, times
}

func assertWorkState(t *testing.T, fixture *todoFixture, todo restapi.TodoTodayResponse, expectedIDs []string, expectedTimes map[string][2]time.Time) {
	t.Helper()
	gotIDs, gotTimes := todoWorkState(t, fixture, todo)
	if !reflect.DeepEqual(gotIDs, expectedIDs) {
		t.Fatalf("Schedule mutation changed work order: before=%v after=%v", expectedIDs, gotIDs)
	}
	for _, id := range expectedIDs {
		if got, want := gotTimes[id], expectedTimes[id]; !got[0].Equal(want[0]) || !got[1].Equal(want[1]) {
			t.Fatalf("Schedule mutation changed internal work times for %s: %s-%s want %s-%s", id, got[0], got[1], want[0], want[1])
		}
	}
}

func getTodo(t *testing.T, fixture *todoFixture) restapi.TodoTodayResponse {
	t.Helper()
	response := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if response.Code != http.StatusOK {
		t.Fatalf("GET Todo status/body=%d/%s", response.Code, response.Body.String())
	}
	return decodeToday(t, response)
}

func scheduleTodoItem(t *testing.T, todo restapi.TodoTodayResponse, ruleID string) restapi.TodoItemResponse {
	t.Helper()
	for _, item := range todo.Items {
		if item.Kind == "schedule" && item.ScheduleRuleID == ruleID {
			return item
		}
	}
	t.Fatalf("Todo has no visible Schedule for rule %s: %+v", ruleID, todo.Items)
	return restapi.TodoItemResponse{}
}

func getScheduleOccurrence(t *testing.T, fixture *todoFixture, ruleID, date string) restapi.ScheduleOccurrenceResponse {
	t.Helper()
	response := fixture.request(t, http.MethodGet, "/schedules/"+ruleID+"/"+date, "", 0)
	if response.Code != http.StatusOK {
		t.Fatalf("GET Schedule occurrence status/body=%d/%s", response.Code, response.Body.String())
	}
	var occurrence restapi.ScheduleOccurrenceResponse
	if err := json.Unmarshal(response.Body.Bytes(), &occurrence); err != nil {
		t.Fatalf("decode Schedule occurrence: %v; body=%s", err, response.Body.String())
	}
	return occurrence
}

func todoPosition(t *testing.T, fixture *todoFixture, itemID string) int32 {
	t.Helper()
	var position int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT position FROM todo_items WHERE id=$1`, itemID).Scan(&position); err != nil {
		t.Fatalf("read Todo position for %s: %v", itemID, err)
	}
	return position
}

func insertAllocationBlock(t *testing.T, fixture *todoFixture, projectID string, date time.Time, startHour, endHour int) error {
	t.Helper()
	ruleID, revisionID, allocationID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	dateString := date.Format("2006-01-02")
	weekday := int(date.Weekday())
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO project_time_allocation_rules(id,user_id,project_id) VALUES($1,$2,$3)`, ruleID, fixture.userID, projectID); err != nil {
		return err
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO project_time_allocation_rule_revisions(
		id,rule_id,user_id,project_id,revision,effective_from,recurrence_start_date,weekday,local_start,local_end,timezone,frequency,changed_by)
		VALUES($1,$2,$3,$4,1,$5,$5,$6,$7::time,$8::time,'UTC','once',$3)`, revisionID, ruleID, fixture.userID, projectID, dateString, weekday,
		fmt.Sprintf("%02d:00", startHour), fmt.Sprintf("%02d:00", endHour)); err != nil {
		return err
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO project_time_allocations(id,rule_id,source_revision_id,user_id,project_id,occurrence_date,start_at,end_at,timezone,changed_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'UTC',$4)`, allocationID, ruleID, revisionID, fixture.userID, projectID, dateString,
		date.Add(time.Duration(startHour)*time.Hour), date.Add(time.Duration(endHour)*time.Hour)); err != nil {
		return err
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM project_time_allocations WHERE id=$1`, allocationID)
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM project_time_allocation_rule_revisions WHERE id=$1`, revisionID)
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM project_time_allocation_rules WHERE id=$1`, ruleID)
	})
	return nil
}

type restTodoItem struct {
	ID    string
	Start *time.Time
	End   *time.Time
}

func persistedTodoItemTime(t *testing.T, fixture *todoFixture, item restapi.TodoItemResponse) *restTodoItem {
	t.Helper()
	var start, end sql.NullTime
	if err := fixture.pool.QueryRow(t.Context(), `SELECT scheduled_start,scheduled_end FROM todo_items WHERE id=$1`, item.ID).Scan(&start, &end); err != nil {
		t.Fatalf("read Todo work times for %s: %v", item.ID, err)
	}
	if !start.Valid || !end.Valid {
		return &restTodoItem{ID: item.ID}
	}
	return &restTodoItem{ID: item.ID, Start: &start.Time, End: &end.Time}
}

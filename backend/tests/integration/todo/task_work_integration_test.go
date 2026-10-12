//go:build integration

package todo_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

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
			direct = &restTodoItem{ID: item.ID, Start: parseItemTime(t, item.ScheduledStart), End: parseItemTime(t, item.ScheduledEnd)}
		case item.Kind == "action_item" && item.ActionItemID == childItemID:
			action = &restTodoItem{ID: item.ID, Start: parseItemTime(t, item.ScheduledStart), End: parseItemTime(t, item.ScheduledEnd)}
		}
	}
	if direct == nil || action == nil || direct.Start == nil || direct.End == nil || action.Start == nil || action.End == nil {
		t.Fatalf("projectless work rows=%+v, want both Task and ActionItem scheduled in the free complement", today.Items)
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

func TestTodoDirectTaskHistorySplitRegenerationAndOwnerReopen(t *testing.T) {
	fixture := newTodoFixture(t, "UTC", 60)
	taskID := ulid.Make().String()
	date := time.Now().UTC().Format("2006-01-02")
	todoDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatal(err)
	}
	previousDate := todoDate.AddDate(0, 0, -1)
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'Direct Task with history',60,'urgent','pending',$2)`, taskID, fixture.userID); err != nil {
		t.Fatalf("insert direct Task: %v", err)
	}
	historyTodoID, historyFrameID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todos(id,user_id,todo_date,timezone) VALUES($1,$2,$3,'UTC')`, historyTodoID, fixture.userID, previousDate); err != nil {
		t.Fatalf("insert history todo: %v", err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todo_items(id,todo_id,kind,task_id,planned_minutes,completed,position) VALUES($1,$2,'task',$3,15,true,0)`, historyFrameID, historyTodoID, taskID); err != nil {
		t.Fatalf("insert Task history frame: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, taskID)
	})
	initial := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if initial.Code != http.StatusOK {
		t.Fatalf("GET Task with history status/body=%d/%s", initial.Code, initial.Body.String())
	}
	today := decodeToday(t, initial)
	var generatedTaskFrame string
	for _, item := range today.Items {
		if item.Kind == "task" && item.TaskID == taskID && !item.History {
			generatedTaskFrame = item.ID
		}
	}
	if generatedTaskFrame == "" {
		t.Fatalf("Task with history has no generated current work row: %+v", today.Items)
	}
	if _, err := fixture.pool.Exec(t.Context(), `DELETE FROM todo_items WHERE id=$1`, generatedTaskFrame); err != nil {
		t.Fatalf("replace generated Task row with split frames: %v", err)
	}
	firstID, secondID := ulid.Make().String(), ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO todo_items(id,todo_id,kind,task_id,planned_minutes,scheduled_start,scheduled_end,position)
		VALUES($1,$3,'task',$4,20,$5::timestamptz,$5::timestamptz + interval '20 minutes',0),
		      ($2,$3,'task',$4,25,$6::timestamptz,$6::timestamptz + interval '25 minutes',1)`, firstID, secondID, today.ID, taskID,
		todoDate.Add(9*time.Hour), todoDate.Add(10*time.Hour)); err != nil {
		t.Fatalf("insert split Task rows: %v", err)
	}
	response := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if response.Code != http.StatusOK {
		t.Fatalf("GET split Task rows status/body=%d/%s", response.Code, response.Body.String())
	}
	today = decodeToday(t, response)
	if len(today.Items) != 3 || !itemByID(today.Items, historyFrameID).History || itemByID(today.Items, historyFrameID).TaskID != taskID {
		t.Fatalf("Task history/split projection=%+v, want 2 current rows plus prior 15m history", today.Items)
	}
	if itemByID(today.Items, firstID).CompletesSource || itemByID(today.Items, secondID).CompletesSource {
		t.Fatalf("split Task prematurely marked source-completing: %+v", today.Items)
	}
	duplicateAdd := fixture.request(t, http.MethodPost, "/today-todo/tasks/"+taskID, "", today.Revision)
	if duplicateAdd.Code != http.StatusOK {
		t.Fatalf("duplicate AddTask status/body=%d/%s, want idempotent success", duplicateAdd.Code, duplicateAdd.Body.String())
	}
	if duplicateToday := decodeToday(t, duplicateAdd); len(duplicateToday.Items) != 3 || duplicateToday.Revision != today.Revision {
		t.Fatalf("duplicate AddTask changed current plan/revision: %+v", duplicateToday)
	}
	partialActual := fixture.request(t, http.MethodPost, "/today-todo/items/"+firstID+":complete", `{"actual_minutes":20}`, today.Revision)
	if partialActual.Code != http.StatusBadRequest {
		t.Fatalf("partial Task actual status/body=%d/%s, want 400", partialActual.Code, partialActual.Body.String())
	}
	var firstCompleted bool
	var currentRevision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT i.completed,l.revision FROM todo_items i JOIN todos l ON l.id=i.todo_id WHERE i.id=$1`, firstID).Scan(&firstCompleted, &currentRevision); err != nil {
		t.Fatal(err)
	}
	if firstCompleted || currentRevision != today.Revision {
		t.Fatalf("rejected partial actual changed Todo frame/revision to %t/%d", firstCompleted, currentRevision)
	}
	partial := fixture.request(t, http.MethodPost, "/today-todo/items/"+firstID+":complete", "", today.Revision)
	if partial.Code != http.StatusOK {
		t.Fatalf("complete partial direct Task frame status/body=%d/%s", partial.Code, partial.Body.String())
	}
	today = decodeToday(t, partial)
	if !itemByID(today.Items, firstID).Completed || !itemByID(today.Items, secondID).CompletesSource {
		t.Fatalf("after partial Task completion=%+v, want first done and second completing the source", today.Items)
	}
	var taskStatus string
	var taskActual sql.NullInt32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT status,actual_minutes FROM tasks WHERE id=$1`, taskID).Scan(&taskStatus, &taskActual); err != nil {
		t.Fatal(err)
	}
	if taskStatus == "done" || taskActual.Valid {
		t.Fatalf("partial Task completion changed source=%s/%+v, want unfinished/NULL", taskStatus, taskActual)
	}
	regenerated := fixture.request(t, http.MethodPost, "/today-todo:regenerate", "", today.Revision)
	if regenerated.Code != http.StatusOK {
		t.Fatalf("regenerate split Task history status/body=%d/%s", regenerated.Code, regenerated.Body.String())
	}
	today = decodeToday(t, regenerated)
	var completedCount, unfinishedCount int
	var unfinishedID string
	var unfinishedMinutes int
	for _, item := range today.Items {
		if item.Kind != "task" || item.TaskID != taskID || item.History {
			continue
		}
		if item.Completed {
			completedCount++
		} else {
			unfinishedCount++
			unfinishedID = item.ID
			if item.PlannedMinutes != nil {
				unfinishedMinutes = *item.PlannedMinutes
			}
		}
	}
	if completedCount != 1 || unfinishedCount != 1 || unfinishedMinutes != 25 || itemByID(today.Items, historyFrameID).DurationMinutes != 15 {
		t.Fatalf("after Task regenerate=%+v, want completed 20m + remaining 25m + prior history 15m", today.Items)
	}
	removeStarted := fixture.request(t, http.MethodDelete, "/today-todo/items/"+unfinishedID, "", today.Revision)
	if removeStarted.Code != http.StatusConflict {
		t.Fatalf("remove started direct Task status/body=%d/%s, want 409", removeStarted.Code, removeStarted.Body.String())
	}
	actualComplete := fixture.request(t, http.MethodPost, "/today-todo/items/"+unfinishedID+":complete", `{"actual_minutes":50}`, today.Revision)
	if actualComplete.Code != http.StatusOK {
		t.Fatalf("complete last direct Task frame status/body=%d/%s", actualComplete.Code, actualComplete.Body.String())
	}
	var status string
	var actual sql.NullInt32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT status,actual_minutes FROM tasks WHERE id=$1`, taskID).Scan(&status, &actual); err != nil {
		t.Fatal(err)
	}
	if status != "done" || !actual.Valid || actual.Int32 != 50 {
		t.Fatalf("completed direct Task source=%s/%+v, want done/50", status, actual)
	}
	completedToday := decodeToday(t, actualComplete)
	completedTaskRow := itemByID(completedToday.Items, unfinishedID)
	if completedTaskRow.CanAddActionItem || completedTaskRow.EstimateEditable || !completedTaskRow.SourceCompleted {
		t.Fatalf("completed Task capabilities=%+v, want source complete and child/estimate editing locked", completedTaskRow)
	}
	var taskRevision int32
	if err := fixture.pool.QueryRow(t.Context(), `SELECT revision FROM tasks WHERE id=$1`, taskID).Scan(&taskRevision); err != nil {
		t.Fatal(err)
	}
	ownerReopen := fixture.request(t, http.MethodPost, "/tasks/"+taskID+":reopen", "", taskRevision)
	if ownerReopen.Code != http.StatusOK {
		t.Fatalf("Task-owner reopen status/body=%d/%s", ownerReopen.Code, ownerReopen.Body.String())
	}
	var completedTodoFrames int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM todo_items WHERE kind='task' AND task_id=$1 AND completed=true`, taskID).Scan(&completedTodoFrames); err != nil {
		t.Fatal(err)
	}
	if completedTodoFrames != 0 {
		t.Fatalf("Task-owner reopen left %d completed direct Task frames across today/history", completedTodoFrames)
	}
	if err := fixture.pool.QueryRow(t.Context(), `SELECT status,actual_minutes FROM tasks WHERE id=$1`, taskID).Scan(&status, &actual); err != nil {
		t.Fatal(err)
	}
	if status == "done" || actual.Valid {
		t.Fatalf("Task-owner reopen source=%s/%+v, want unfinished/NULL actual", status, actual)
	}
}

func TestTodoAllDayScheduleSuppressesDirectTaskWork(t *testing.T) {
	fixture := newTodoFixture(t, "Asia/Tokyo", 45)
	taskID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO tasks(id,user_id,assignee_id,title,manual_estimated_minutes,priority,status,changed_by) VALUES($1,$2,$2,'All-day blocked direct Task',20,'urgent','open',$2)`, taskID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = fixture.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, taskID)
	})
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	localDate := time.Now().In(location)
	start := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 0, 0, 0, 0, location)
	body := fmt.Sprintf(`{"title":"All-day blocker","timezone":"Asia/Tokyo","local_start":%q,"local_end":%q,"all_day":true,"frequency":"once","interval":0}`, start.Format("2006-01-02T15:04:05"), start.AddDate(0, 0, 1).Format("2006-01-02T15:04:05"))
	created := fixture.request(t, http.MethodPost, "/schedule-rules", body, 0)
	if created.Code != http.StatusCreated {
		t.Fatalf("create all-day Schedule status/body=%d/%s", created.Code, created.Body.String())
	}
	response := fixture.request(t, http.MethodGet, "/today-todo", "", 0)
	if response.Code != http.StatusOK {
		t.Fatalf("GET all-day direct Task status/body=%d/%s", response.Code, response.Body.String())
	}
	today := decodeToday(t, response)
	if len(today.Items) != 1 || today.Items[0].Kind != "schedule" {
		t.Fatalf("all-day direct Task snapshot=%+v, want only Schedule and no Task work", today.Items)
	}
	var taskRows int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM todo_items WHERE todo_id=$1 AND kind='task' AND task_id=$2`, today.ID, taskID).Scan(&taskRows); err != nil {
		t.Fatal(err)
	}
	if taskRows != 0 {
		t.Fatalf("all-day Task persisted despite being hidden: %d rows", taskRows)
	}
	blocked := fixture.request(t, http.MethodPost, "/today-todo/tasks/"+taskID, "", today.Revision)
	if blocked.Code != http.StatusConflict {
		t.Fatalf("AddTask during visible all-day Schedule status/body=%d/%s, want 409", blocked.Code, blocked.Body.String())
	}
	allDayID := today.Items[0].ID
	hidden := fixture.request(t, http.MethodPost, "/today-todo/schedules/"+allDayID+":hide", "", today.Revision)
	if hidden.Code != http.StatusOK {
		t.Fatalf("hide all-day Schedule status/body=%d/%s", hidden.Code, hidden.Body.String())
	}
	hiddenToday := decodeToday(t, hidden)
	added := fixture.request(t, http.MethodPost, "/today-todo/tasks/"+taskID, "", hiddenToday.Revision)
	if added.Code != http.StatusOK {
		t.Fatalf("AddTask after all-day hidden status/body=%d/%s", added.Code, added.Body.String())
	}
	withTask := decodeToday(t, added)
	foundTask := false
	for _, item := range withTask.Items {
		if item.Kind == "task" && item.TaskID == taskID {
			foundTask = true
		}
	}
	if !foundTask {
		t.Fatalf("hidden all-day Schedule did not permit Task row: %+v", withTask.Items)
	}
	allDayID = withTask.HiddenSchedules[0].ID
	restored := fixture.request(t, http.MethodPost, "/today-todo/schedules/"+allDayID+":restore", "", withTask.Revision)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore all-day Schedule status/body=%d/%s", restored.Code, restored.Body.String())
	}
	restoredToday := decodeToday(t, restored)
	for _, item := range restoredToday.Items {
		if item.Kind == "task" && item.TaskID == taskID {
			t.Fatalf("restored visible all-day Schedule retained Task work: %+v", restoredToday.Items)
		}
	}
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

func parseItemTime(t *testing.T, value *string) *time.Time {
	t.Helper()
	if value == nil {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		t.Fatalf("parse scheduled time %q: %v", *value, err)
	}
	return &parsed
}

//go:build integration

package metric_test

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	metricusecase "github.com/Najah7/task2todaytodo/internal/application/metric/usecase"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/oklog/ulid/v2"
)

func TestMetricsProjectTodoFramesSchedulesTimezoneDueDatesAndAccess(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	actorID, ownerID, inaccessibleOwnerID := newID(), newID(), newID()
	for _, user := range []struct{ id, timezone string }{
		{actorID, "Asia/Tokyo"}, {ownerID, "Asia/Tokyo"}, {inaccessibleOwnerID, "Asia/Tokyo"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone) VALUES($1,'Metrics','Test',$2,'unused',now(),$3)`, user.id, user.id+"@metrics.test", user.timezone); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	todayTime := time.Now().In(location)
	today := time.Date(todayTime.Year(), todayTime.Month(), todayTime.Day(), 0, 0, 0, 0, location)
	todayDate := today.Format("2006-01-02")
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysAgo := today.AddDate(0, 0, -2).Format("2006-01-02")
	oldDue := today.AddDate(0, 0, -1).Format("2006-01-02")
	futureDue := today.AddDate(0, 0, 1).Format("2006-01-02")

	accessibleProjectID, inaccessibleProjectID, doneProjectID := newID(), newID(), newID()
	for _, project := range []struct{ id, owner, title, status string }{
		{accessibleProjectID, ownerID, "Accessible project", "open"},
		{inaccessibleProjectID, inaccessibleOwnerID, "Inaccessible project", "open"},
		{doneProjectID, actorID, "Done project", "done"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,title,status,changed_by) VALUES($1,$2,$3,$4,$2)`, project.id, project.owner, project.title, project.status); err != nil {
			t.Fatalf("insert project: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'viewer',$3)`, accessibleProjectID, actorID, ownerID); err != nil {
		t.Fatalf("add project viewer: %v", err)
	}

	taskIDs := map[string]string{}
	insertTask := func(key, owner, assignee, projectID, title, status string, due *string) string {
		t.Helper()
		id := newID()
		if _, err := pool.Exec(ctx, `INSERT INTO tasks(id,user_id,assignee_id,project_id,title,due_date,priority,status,changed_by) VALUES($1,$2,$3,$4,$5,$6,'low',$7,$2)`, id, owner, assignee, nullable(projectID), title, due, status); err != nil {
			t.Fatalf("insert Task %s: %v", key, err)
		}
		taskIDs[key] = id
		return id
	}
	frameTaskID := insertTask("frames", actorID, actorID, "", "Grouped Todo Task", "open", nil)
	frameActionTaskID := insertTask("frame_action", actorID, actorID, "", "Grouped Todo ActionItem task", "open", nil)
	suppressedTaskID := insertTask("suppressed", actorID, actorID, "", "All-day suppressed work", "open", nil)

	makeTodo := func(date string) string {
		t.Helper()
		id := newID()
		if _, err := pool.Exec(ctx, `INSERT INTO todos(id,user_id,todo_date,timezone) VALUES($1,$2,$3,'Asia/Tokyo')`, id, actorID, date); err != nil {
			t.Fatalf("insert Todo %s: %v", date, err)
		}
		return id
	}
	todayTodo, yesterdayTodo, oldTodo := makeTodo(todayDate), makeTodo(yesterday), makeTodo(twoDaysAgo)
	insertTaskFrame := func(todoID, taskID string, position int, completed bool) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO todo_items(id,todo_id,kind,task_id,planned_minutes,completed,position) VALUES($1,$2,'task',$3,30,$4,$5)`, newID(), todoID, taskID, completed, position); err != nil {
			t.Fatalf("insert Task frame: %v", err)
		}
	}
	insertActionFrame := func(todoID, actionItemID string, position int, completed bool) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO todo_items(id,todo_id,kind,action_item_id,planned_minutes,completed,position) VALUES($1,$2,'action_item',$3,15,$4,$5)`, newID(), todoID, actionItemID, completed, position); err != nil {
			t.Fatalf("insert ActionItem frame: %v", err)
		}
	}
	insertAction := func(taskID, title string, position int, due *string, completed bool) string {
		t.Helper()
		id := newID()
		if _, err := pool.Exec(ctx, `INSERT INTO action_items(id,task_id,title,due_date,completed,position,estimated_minutes,priority) VALUES($1,$2,$3,$4,$5,$6,15,'low')`, id, taskID, title, due, completed, position); err != nil {
			t.Fatalf("insert ActionItem: %v", err)
		}
		return id
	}

	// Each source appears at most once per Todo. ActionItem and Task completion
	// comes from the whole-source Todo row, while Schedule completion comes from
	// the current occurrence row.
	insertTaskFrame(todayTodo, frameTaskID, 0, false)
	insertTaskFrame(yesterdayTodo, frameTaskID, 0, true)
	groupedActionID := insertAction(frameActionTaskID, "Grouped action", 0, nil, false)
	insertActionFrame(todayTodo, groupedActionID, 1, true)
	insertTaskFrame(oldTodo, suppressedTaskID, 0, true)
	suppressedActionID := insertAction(suppressedTaskID, "Suppressed action", 0, nil, true)
	insertActionFrame(oldTodo, suppressedActionID, 1, true)

	scheduleKeys := []string{}
	addSchedule := func(date string, title string, allDay, hidden, completed bool, skipped, deleted bool) string {
		t.Helper()
		ruleID, revisionID, scheduleID := newID(), newID(), newID()
		scheduleKeys = append(scheduleKeys, ruleID)
		if _, err := pool.Exec(ctx, `INSERT INTO schedule_rules(id,user_id) VALUES($1,$2)`, ruleID, actorID); err != nil {
			t.Fatalf("insert schedule rule: %v", err)
		}
		occurrenceDate, err := time.ParseInLocation("2006-01-02", date, location)
		if err != nil {
			t.Fatal(err)
		}
		start, end := occurrenceDate.Add(9*time.Hour), occurrenceDate.Add(10*time.Hour)
		localStart, localEnd := occurrenceDate.Add(9*time.Hour), occurrenceDate.Add(10*time.Hour)
		if allDay {
			start, end = occurrenceDate, occurrenceDate.AddDate(0, 0, 1)
			localStart, localEnd = occurrenceDate, occurrenceDate.AddDate(0, 0, 1)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schedule_rule_revisions(id,rule_id,user_id,revision,effective_from,recurrence_anchor_date,local_start,local_end,all_day,timezone,frequency,interval_count,title,assignee_id,changed_by) VALUES($1,$2,$3,1,$4,$4,$5,$6,$7,'Asia/Tokyo','once',0,$8,$3,$3)`, revisionID, ruleID, actorID, date, localStart, localEnd, allDay, title); err != nil {
			t.Fatalf("insert schedule revision: %v", err)
		}
		var skippedAt, deletedAt any
		if skipped {
			skippedAt = time.Now()
		}
		if deleted {
			deletedAt = time.Now()
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schedules(id,rule_id,source_revision_id,user_id,occurrence_date,start_at,end_at,all_day,title,assignee_id,timezone,completed,skipped_at,deleted_at,changed_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$4,'Asia/Tokyo',$10,$11,$12,$4)`, scheduleID, ruleID, revisionID, actorID, date, start, end, allDay, title, completed, skippedAt, deletedAt); err != nil {
			t.Fatalf("insert schedule occurrence: %v", err)
		}
		todoID := todayTodo
		if date == twoDaysAgo {
			todoID = oldTodo
		}
		if _, err := pool.Exec(ctx, `INSERT INTO todo_items(id,todo_id,kind,schedule_rule_id,occurrence_date,scheduled_start,scheduled_end,completed,hidden,position) VALUES($1,$2,'schedule',$3,$4,$5,$6,false,$7,$8)`, newID(), todoID, ruleID, date, start, end, hidden, 4+len(scheduleKeys)); err != nil {
			t.Fatalf("insert Schedule Todo row: %v", err)
		}
		return scheduleID
	}
	addSchedule(todayDate, "Timed current complete", false, false, true, false, false)
	addSchedule(todayDate, "Hidden timed", false, true, false, false, false)
	addSchedule(todayDate, "Skipped timed", false, false, false, true, false)
	addSchedule(todayDate, "Deleted timed", false, false, false, false, true)
	addSchedule(twoDaysAgo, "Visible all day", true, false, false, false, false)

	// Overdue work counts incomplete children with inherited due dates, and
	// excludes parent duplication, completed children, inherited dates overridden
	// by a future child date, done tasks, inactive projects, and unreadable work.
	parentDue := oldDue
	childrenTaskID := insertTask("children", actorID, actorID, "", "Parent with children", "waiting_on_others", &parentDue)
	insertAction(childrenTaskID, "Inherited overdue", 0, nil, false)
	insertAction(childrenTaskID, "Completed child", 1, &oldDue, true)
	insertTask("childless", actorID, actorID, "", "Childless overdue", "pending", &oldDue)
	overriddenTaskID := insertTask("overridden", actorID, actorID, "", "Overridden parent due", "open", &oldDue)
	insertAction(overriddenTaskID, "Future child", 0, &futureDue, false)
	incompleteTaskID := insertTask("incomplete_child", actorID, actorID, "", "Incomplete child overdue", "open", nil)
	insertAction(incompleteTaskID, "Overdue child", 0, &oldDue, false)
	insertTask("done", actorID, actorID, "", "Done Task", "done", &oldDue)
	insertTask("accessible_project", ownerID, actorID, accessibleProjectID, "Shared overdue Task", "open", &oldDue)
	insertTask("inaccessible_project", inaccessibleOwnerID, actorID, inaccessibleProjectID, "Unreadable overdue Task", "open", &oldDue)
	insertTask("done_project", actorID, actorID, doneProjectID, "Task in done Project", "open", &oldDue)
	todayDue := todayDate
	insertTask("due_today", actorID, actorID, "", "Due today", "open", &todayDue)

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		clean := func(query string, args ...any) {
			if _, err := pool.Exec(cleanupCtx, query, args...); err != nil {
				t.Errorf("integration cleanup: %v", err)
			}
		}
		clean(`DELETE FROM todos WHERE user_id=$1`, actorID)
		clean(`DELETE FROM schedule_revisions WHERE schedule_id IN (SELECT id FROM schedules WHERE rule_id=ANY($1::text[]))`, scheduleKeys)
		clean(`DELETE FROM schedules WHERE rule_id=ANY($1::text[])`, scheduleKeys)
		clean(`DELETE FROM schedule_rule_revisions WHERE rule_id=ANY($1::text[])`, scheduleKeys)
		clean(`DELETE FROM schedule_rules WHERE id=ANY($1::text[])`, scheduleKeys)
		taskCleanup := []string{
			taskIDs["frames"], taskIDs["frame_action"], taskIDs["suppressed"], taskIDs["children"], taskIDs["childless"], taskIDs["overridden"], taskIDs["incomplete_child"], taskIDs["done"], taskIDs["accessible_project"], taskIDs["inaccessible_project"], taskIDs["done_project"], taskIDs["due_today"],
		}
		clean(`DELETE FROM task_revisions WHERE id=ANY($1::text[])`, taskCleanup)
		clean(`DELETE FROM tasks WHERE id=ANY($1::text[])`, taskCleanup)
		clean(`DELETE FROM project_members WHERE project_id=$1 AND user_id=$2`, accessibleProjectID, actorID)
		clean(`DELETE FROM project_revisions WHERE id=ANY($1::text[])`, []string{accessibleProjectID, inaccessibleProjectID, doneProjectID})
		clean(`DELETE FROM projects WHERE id=ANY($1::text[])`, []string{accessibleProjectID, inaccessibleProjectID, doneProjectID})
		clean(`DELETE FROM users WHERE id=ANY($1::text[])`, []string{actorID, ownerID, inaccessibleOwnerID})
	})

	metrics := application.NewStore(pool).Metric
	completion, err := metricusecase.NewTodoCompletionUseCase(metrics).Execute(ctx, actorID, 7)
	if err != nil {
		t.Fatalf("ReadTodoCompletion: %v", err)
	}
	if completion.Timezone != "Asia/Tokyo" || completion.StartDate != today.AddDate(0, 0, -6).Format("2006-01-02") || completion.EndDate != todayDate {
		t.Fatalf("completion period=%+v, want local seven-day period ending %s", completion, todayDate)
	}
	if completion.TotalPlannedItems != 5 || completion.CompletedItems != 3 || completion.CompletionRatio == nil || *completion.CompletionRatio != 0.6 {
		t.Fatalf("completion totals=%+v, want 5 planned, 3 completed, ratio .6", completion)
	}
	if len(completion.Days) != 7 || completion.Days[0].Date != today.AddDate(0, 0, -6).Format("2006-01-02") || completion.Days[0].CompletionRatio != nil {
		t.Fatalf("daily rows=%+v, want all dates including zero-data day", completion.Days)
	}
	todayMetrics := completion.Days[6]
	if todayMetrics.PlannedItems != 3 || todayMetrics.CompletedItems != 2 || todayMetrics.CompletionRatio == nil || *todayMetrics.CompletionRatio != (2.0/3.0) {
		t.Fatalf("today=%+v, want one Task, one ActionItem, and one Schedule with two complete", todayMetrics)
	}
	oldMetrics := completion.Days[4]
	if oldMetrics.PlannedItems != 1 || oldMetrics.CompletedItems != 0 {
		t.Fatalf("all-day day=%+v, want only visible Schedule and no work frames", oldMetrics)
	}

	overdue, err := metricusecase.NewOverdueWorkUseCase(metrics).Execute(ctx, actorID)
	if err != nil {
		t.Fatalf("ReadOverdueWork: %v", err)
	}
	if overdue.Timezone != "Asia/Tokyo" || overdue.AsOfDate != todayDate || overdue.TotalCount != 4 {
		t.Fatalf("overdue snapshot=%+v, want four eligible units on %s", overdue, todayDate)
	}
	counts := map[string]int32{}
	for _, project := range overdue.Projects {
		key := "personal"
		if project.ProjectTitle != nil {
			key = *project.ProjectTitle
		}
		counts[key] = project.Count
	}
	if counts["personal"] != 3 || counts["Accessible project"] != 1 || counts["Inaccessible project"] != 0 {
		t.Fatalf("overdue project counts=%v, want personal 3, accessible 1, inaccessible 0", counts)
	}
}

func newID() string { return ulid.Make().String() }

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

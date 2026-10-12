//go:build integration

package todo_test

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	tododao "github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/oklog/ulid/v2"
)

func TestTodoSourceReaderProjectsVirtualScheduleAndAllocationInUsersLocalDay(t *testing.T) {
	pool := testdb.Open(t)
	ctx := t.Context()
	userID, viewerID, projectID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	taskID, actionItemID := ulid.Make().String(), ulid.Make().String()
	directTaskID := ulid.Make().String()
	viewerTaskID, viewerActionID := ulid.Make().String(), ulid.Make().String()
	allocationRuleID, allocationRevisionID := ulid.Make().String(), ulid.Make().String()
	scheduleRuleID, scheduleRevisionID := ulid.Make().String(), ulid.Make().String()
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(location)
	localDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	dateString := localDate.Format("2006-01-02")
	previousWeek := localDate.AddDate(0, 0, -7)
	weekday := int(today.Weekday())

	if _, err := pool.Exec(ctx, `
		INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone)
		VALUES($1,'Todo','Source',$2,'unused',now(),'America/Los_Angeles'),
		      ($3,'Todo','Viewer',$4,'unused',now(),'America/Los_Angeles')`, userID, userID+"@todo-source.test", viewerID, viewerID+"@todo-viewer.test"); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO projects(id,user_id,title,changed_by)
		VALUES($1,$2,'Virtual allocation project',$2)`, projectID, userID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'viewer',$3)`, projectID, viewerID, userID); err != nil {
		t.Fatalf("add read-only project member: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tasks(id,user_id,assignee_id,project_id,title,priority,status,changed_by)
		VALUES($1,$2,$2,$3,'Local due task','high','open',$2)`, taskID, userID, projectID); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO action_items(id,task_id,title,position,estimated_minutes,due_date,priority)
		VALUES($1,$2,'Due today',0,30,$3,'high')`, actionItemID, taskID, dateString); err != nil {
		t.Fatalf("insert ActionItem: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tasks(id,user_id,assignee_id,project_id,title,manual_estimated_minutes,priority,status,changed_by)
		VALUES($1,$2,$2,$3,'Direct work Task',45,'urgent','open',$2)`, directTaskID, userID, projectID); err != nil {
		t.Fatalf("insert childless Task: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tasks(id,user_id,assignee_id,project_id,title,priority,status,changed_by)
		VALUES($1,$2,$3,$4,'Viewer assigned task','high','open',$2)`, viewerTaskID, userID, viewerID, projectID); err != nil {
		t.Fatalf("insert viewer assigned task: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO action_items(id,task_id,title,position,estimated_minutes,due_date,priority)
		VALUES($1,$2,'Read-only item',0,15,$3,'high')`, viewerActionID, viewerTaskID, dateString); err != nil {
		t.Fatalf("insert viewer ActionItem: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO project_time_allocation_rules(id,user_id,project_id)
		VALUES($1,$2,$3)`, allocationRuleID, userID, projectID); err != nil {
		t.Fatalf("insert allocation rule: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO project_time_allocation_rule_revisions(
		  id,rule_id,user_id,project_id,revision,effective_from,recurrence_start_date,
		  weekday,local_start,local_end,end_day_offset,timezone,frequency,changed_by)
		VALUES($1,$2,$3,$4,1,$5,$5,$6,TIME '09:00',TIME '17:00',0,'America/Los_Angeles','weekly',$3)`,
		allocationRevisionID, allocationRuleID, userID, projectID, previousWeek.Format("2006-01-02"), weekday); err != nil {
		t.Fatalf("insert recurring allocation revision: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schedule_rules(id,user_id) VALUES($1,$2)`, scheduleRuleID, userID); err != nil {
		t.Fatalf("insert schedule rule: %v", err)
	}
	localStart := time.Date(today.Year(), today.Month(), today.Day(), 20, 30, 0, 0, time.UTC)
	localEnd := localStart.Add(30 * time.Minute)
	if _, err := pool.Exec(ctx, `
		INSERT INTO schedule_rule_revisions(
		  id,rule_id,user_id,revision,effective_from,recurrence_anchor_date,local_start,local_end,
		  timezone,frequency,interval_count,title,project_id,assignee_id,changed_by)
		VALUES($1,$2,$3,1,$4,$4,$5,$6,'America/Los_Angeles','daily',1,'Virtual recurring schedule',$7,$3,$3)`,
		scheduleRevisionID, scheduleRuleID, userID, dateString, localStart, localEnd, projectID); err != nil {
		t.Fatalf("insert recurring schedule revision: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM schedules WHERE rule_id=$1`, scheduleRuleID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM schedule_rule_revisions WHERE rule_id=$1`, scheduleRuleID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM schedule_rules WHERE id=$1`, scheduleRuleID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_time_allocation_rule_revisions WHERE rule_id=$1`, allocationRuleID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_time_allocation_rules WHERE id=$1`, allocationRuleID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM tasks WHERE id=$1`, taskID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM tasks WHERE id=$1`, directTaskID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM tasks WHERE id=$1`, viewerTaskID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::text[])`, []string{userID, viewerID})
	})

	store := application.NewStore(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	sources, err := application.NewTodoSourceReader(store, tx).ReadPlanningSources(ctx, userID, localDate, "America/Los_Angeles")
	if err != nil {
		t.Fatalf("ReadPlanningSources(): %v", err)
	}
	if len(sources.Candidates) != 2 {
		t.Fatalf("candidates=%+v, want writable ActionItem plus childless direct Task", sources.Candidates)
	}
	var candidate *tododao.WorkSource
	for index := range sources.Candidates {
		if sources.Candidates[index].ID == actionItemID {
			candidate = &sources.Candidates[index]
		}
	}
	if candidate == nil || candidate.Kind != "action_item" {
		t.Fatalf("ActionItem candidate not projected: %+v", sources.Candidates)
	}
	if candidate.DueAt == nil || candidate.DueAt.In(location).Format("2006-01-02") != dateString {
		t.Fatalf("DueAt=%v, want local calendar date %s in America/Los_Angeles", candidate.DueAt, dateString)
	}
	if candidate.PriorityWeight != 50 {
		t.Fatalf("PriorityWeight=%d, want database high-priority weight 50", candidate.PriorityWeight)
	}
	var direct *tododao.WorkSource
	for index := range sources.Candidates {
		if sources.Candidates[index].ID == directTaskID {
			direct = &sources.Candidates[index]
		}
	}
	if direct == nil || direct.Kind != "task" || direct.EstimatedMinutes != 45 || direct.PriorityWeight != 100 {
		t.Fatalf("direct Task=%+v, want Task source, 45m, urgent weight 100", direct)
	}
	foundAllocation := false
	for _, allocation := range sources.Allocations {
		if allocation.ProjectID == projectID && allocation.StartAt.In(location).Format("2006-01-02") == dateString {
			foundAllocation = true
		}
	}
	if !foundAllocation {
		t.Fatalf("allocations=%+v, want today's virtual recurring allocation for project %s", sources.Allocations, projectID)
	}
	if len(sources.Schedules) != 1 || sources.Schedules[0].RuleID != scheduleRuleID || sources.Schedules[0].OccurrenceDate.Format("2006-01-02") != dateString {
		t.Fatalf("schedules=%+v, want today's virtual occurrence keyed by rule/date", sources.Schedules)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET timezone='Asia/Tokyo' WHERE id=$1`, userID); err != nil {
		t.Fatalf("change profile timezone: %v", err)
	}
	savedCalendar, err := application.NewTodoSourceReader(store, tx).ReadPlanningSources(ctx, userID, localDate, "America/Los_Angeles")
	if err != nil {
		t.Fatalf("ReadPlanningSources() in persisted todo timezone: %v", err)
	}
	if savedCalendar.Timezone != "America/Los_Angeles" {
		t.Fatalf("saved calendar timezone=%q, want America/Los_Angeles", savedCalendar.Timezone)
	}
	foundSavedAllocation, foundSavedSleep := false, false
	for _, allocation := range savedCalendar.Allocations {
		if allocation.ProjectID == projectID && allocation.StartAt.In(location).Format("2006-01-02 15:04") == dateString+" 09:00" && allocation.EndAt.In(location).Format("15:04") == "17:00" {
			foundSavedAllocation = true
		}
	}
	for _, sleep := range savedCalendar.Sleep {
		if sleep.Start.Equal(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)) && sleep.End.Equal(time.Date(today.Year(), today.Month(), today.Day(), 7, 0, 0, 0, location)) {
			foundSavedSleep = true
		}
	}
	if !foundSavedAllocation || !foundSavedSleep {
		t.Fatalf("saved todo calendar after profile timezone change: allocations=%+v sleep=%+v, want LA 09:00-17:00 allocation and 00:00-07:00 sleep boundary", savedCalendar.Allocations, savedCalendar.Sleep)
	}
	viewerSources, err := application.NewTodoSourceReader(store, tx).ReadPlanningSources(ctx, viewerID, localDate, "America/Los_Angeles")
	if err != nil {
		t.Fatalf("ReadPlanningSources() as read-only member: %v", err)
	}
	if len(viewerSources.Candidates) != 0 {
		t.Fatalf("read-only member candidates=%+v, want no auto-plannable sources", viewerSources.Candidates)
	}
	viewerItems, err := application.NewTodoSourceReader(store, tx).ReadWorkSources(ctx, viewerID, localDate, "America/Los_Angeles", []tododao.WorkSourceRef{{Kind: "action_item", ID: viewerActionID}})
	if err != nil {
		t.Fatalf("ReadWorkSources() as read-only member: %v", err)
	}
	if len(viewerItems) != 1 || viewerItems[0].CanAddToToday || viewerItems[0].CanComplete || viewerItems[0].CanReopen {
		t.Fatalf("read-only member source capabilities=%+v, want readable and not mutable", viewerItems)
	}
}

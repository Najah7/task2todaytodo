package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
)

func TestTodayViewKeepsSavedWorkVisibleWithAllDaySchedule(t *testing.T) {
	date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	start := date.Add(9 * time.Hour)
	end := start.Add(time.Hour)
	minutes := 30
	todo := dao.Todo{ID: "todo", UserID: "user", Date: date, Timezone: "UTC", Revision: 1}
	saved := []dao.Item{
		{ID: "action", TodoID: "todo", Kind: "action_item", ActionItemID: "a", PlannedMinutes: &minutes},
		{ID: "schedule", TodoID: "todo", Kind: "schedule", ScheduleRuleID: "r", OccurrenceDate: date, ScheduledStart: &start, ScheduledEnd: &end, Completed: true},
	}
	sources := dao.PlanningSources{
		Schedules: []dao.ScheduleOccurrence{{RuleID: "r", OccurrenceDate: date, Title: "Holiday", StartAt: start, EndAt: end, AllDay: true, Completed: true}},
		History:   []dao.HistoryFrame{{ID: "history", ActionItemID: "a", PlannedMinutes: 20, CompletedAt: date.AddDate(0, 0, -1)}},
	}
	view, err := buildTodaySnapshot(todo, saved, sources, []dao.WorkSource{
		{Kind: "action_item", ID: "a", Title: "Review", EstimatedMinutes: 50, CompletedBeforeDayMinutes: 20},
	}, start.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Items) != 2 || view.Items[0].ActionItemID != "a" || view.Items[1].ScheduleRuleID != "r" || !view.Items[1].Completed {
		t.Fatalf("visible items=%+v, want saved work and completed all-day schedule", view.Items)
	}
	if len(view.HiddenSchedules) != 0 || view.Progress.TotalItems != 2 || view.Progress.TotalMinutes != 90 {
		t.Fatalf("hidden/progress=%+v %+v", view.HiddenSchedules, view.Progress)
	}
	if view.Items[0].History {
		t.Fatalf("history artifact was exposed: %+v", view.Items[0])
	}
}

func TestTodayScheduleCompletionUsesCurrentOwnerProjection(t *testing.T) {
	date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	start, end := date.Add(9*time.Hour), date.Add(10*time.Hour)
	todo := dao.Todo{ID: "todo", UserID: "user", Date: date, Timezone: "UTC", Revision: 1}
	saved := []dao.Item{{ID: "schedule", TodoID: "todo", Kind: "schedule", ScheduleRuleID: "rule", OccurrenceDate: date, ScheduledStart: &start, ScheduledEnd: &end, Completed: true}}
	sources := dao.PlanningSources{Schedules: []dao.ScheduleOccurrence{{RuleID: "rule", OccurrenceDate: date, StartAt: start, EndAt: end, Completed: false, CanComplete: true}}}
	view, err := buildTodaySnapshot(todo, saved, sources, nil, start.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Items) != 1 || view.Items[0].Completed || !view.Items[0].CanComplete {
		t.Fatalf("item=%+v, want current uncompleted source projection", view.Items)
	}
}

func TestTodayActionRemainsRemovableAfterEarlierHistory(t *testing.T) {
	date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	minutes := 30
	start := date.Add(10 * time.Hour)
	end := start.Add(30 * time.Minute)
	row := workTodayItem(dao.Item{ID: "frame", Kind: "action_item", ActionItemID: "a", PlannedMinutes: &minutes, ScheduledStart: &start, ScheduledEnd: &end},
		dao.WorkSource{Kind: "action_item", ID: "a", EstimatedMinutes: 60, HasCompletedFrame: true, CanRemoveFromToday: true})
	if !row.CanRemove {
		t.Fatalf("work row=%+v, want removable regardless of earlier history", row)
	}
}

func TestTodayWorkCompletionReflectsWholeSourceState(t *testing.T) {
	minutes := 30
	source := dao.WorkSource{Kind: "action_item", ID: "a", EstimatedMinutes: 60, Completed: false}
	item := workTodayItem(dao.Item{ID: "whole", Kind: "action_item", ActionItemID: "a", PlannedMinutes: &minutes}, source)
	if item.WorkComplete || item.PlannedMinutes == nil || *item.PlannedMinutes != minutes {
		t.Fatalf("work row=%+v, want persisted whole-source state and estimate", item)
	}
	source.Completed, source.CanReopen = true, true
	completed := workTodayItem(dao.Item{ID: "whole", Kind: "action_item", ActionItemID: "a", PlannedMinutes: &minutes, Completed: true}, source)
	if !completed.WorkComplete || completed.CanComplete || !completed.CanReopen {
		t.Fatalf("completed row=%+v, want whole-source completion and reopen state", completed)
	}
}

func TestTodayProgressExcludesHistoryAndHiddenSchedule(t *testing.T) {
	start := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	actionMinutes, historyMinutes := 40, 90
	progress := calculateProgress([]TodayItem{
		{Kind: "action_item", ActionItemID: "a", PlannedMinutes: &actionMinutes, DurationMinutes: actionMinutes, Completed: true, WorkComplete: false},
		{Kind: "action_item", ActionItemID: "a", PlannedMinutes: &historyMinutes, Completed: true, History: true, WorkComplete: true},
		{Kind: "schedule", ScheduledStart: &start, ScheduledEnd: &end, DurationMinutes: 30, Hidden: true},
		{Kind: "schedule", ScheduledStart: &start, ScheduledEnd: &end, DurationMinutes: 30, Completed: true},
	})
	if progress.TotalItems != 2 || progress.CompletedItems != 1 || progress.TotalMinutes != 70 || progress.CompletedMinutes != 70 || progress.RemainingMinutes != 0 {
		t.Fatalf("progress=%+v", progress)
	}
}

func TestTodayScheduleDurationClipsToListTimezoneDay(t *testing.T) {
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 10, 11, 23, 30, 0, 0, location)
	end := time.Date(2026, 10, 12, 1, 0, 0, 0, location)
	snapshot, err := buildTodaySnapshot(
		dao.Todo{ID: "todo", UserID: "user", Date: date, Timezone: "Asia/Tokyo"},
		[]dao.Item{{ID: "schedule-frame", TodoID: "todo", Kind: "schedule", ScheduleRuleID: "rule", OccurrenceDate: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC), ScheduledStart: &start, ScheduledEnd: &end}},
		dao.PlanningSources{Schedules: []dao.ScheduleOccurrence{{RuleID: "rule", OccurrenceDate: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC), StartAt: start, EndAt: end}}}, nil, time.Date(2026, 10, 12, 0, 0, 0, 0, location),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 1 || snapshot.Items[0].DurationMinutes != 60 || snapshot.Progress.TotalMinutes != 60 {
		t.Fatalf("items=%+v progress=%+v, want one 60-minute local-day overlap", snapshot.Items, snapshot.Progress)
	}
	if !snapshot.Items[0].ScheduledStart.Equal(start) || !snapshot.Items[0].ScheduledEnd.Equal(end) {
		t.Fatalf("fixed Schedule times changed: %+v", snapshot.Items[0])
	}
}

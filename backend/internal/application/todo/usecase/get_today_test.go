package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
)

func TestTodayViewSuppressesActionItemsAndHistoryForAllDaySchedule(t *testing.T) {
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
		{ID: "a", Title: "Review", EstimatedMinutes: 50, CompletedBeforeDayMinutes: 20},
	}, start.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Items) != 1 || view.Items[0].ScheduleRuleID != "r" || !view.Items[0].Completed {
		t.Fatalf("visible items=%+v, want completed all-day schedule only", view.Items)
	}
	if len(view.HiddenSchedules) != 0 || view.Progress.TotalItems != 1 || view.Progress.TotalMinutes != 60 {
		t.Fatalf("hidden/progress=%+v %+v", view.HiddenSchedules, view.Progress)
	}
	for _, item := range view.Items {
		if item.Kind == "action_item" || item.History {
			t.Fatalf("all-day view exposed invisible action history: %+v", item)
		}
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

func TestTodayActionStartedReflectsCompletedFrameHistory(t *testing.T) {
	date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	minutes := 30
	start := date.Add(10 * time.Hour)
	end := start.Add(30 * time.Minute)
	row := workTodayItem(dao.Item{ID: "frame", Kind: "action_item", ActionItemID: "a", PlannedMinutes: &minutes, ScheduledStart: &start, ScheduledEnd: &end},
		dao.WorkSource{Kind: "action_item", ID: "a", EstimatedMinutes: 60, HasCompletedFrame: true, CanRemoveFromToday: true}, 0)
	if !row.Started || row.CanRemove {
		t.Fatalf("started row=%+v, want started and not removable", row)
	}
}

func TestTodayCompletesSourceUsesCurrentFrameAndPriorCompletedMinutes(t *testing.T) {
	minutes := 30
	source := dao.WorkSource{Kind: "action_item", ID: "a", EstimatedMinutes: 60}
	first := workTodayItem(dao.Item{ID: "first", Kind: "action_item", ActionItemID: "a", PlannedMinutes: &minutes}, source, 0)
	second := workTodayItem(dao.Item{ID: "second", Kind: "action_item", ActionItemID: "a", PlannedMinutes: &minutes}, source, 0)
	if first.CompletesSource || second.CompletesSource {
		t.Fatalf("initial split rows=%+v/%+v, want neither frame alone to complete source", first, second)
	}
	secondAfterFirstCompleted := workTodayItem(dao.Item{ID: "second", Kind: "action_item", ActionItemID: "a", PlannedMinutes: &minutes}, source, 30)
	if !secondAfterFirstCompleted.CompletesSource {
		t.Fatalf("remaining row=%+v, want it to complete source after the other 30 minutes are complete", secondAfterFirstCompleted)
	}
	completed := workTodayItem(dao.Item{ID: "first", Kind: "action_item", ActionItemID: "a", PlannedMinutes: &minutes, Completed: true}, source, 30)
	if completed.CompletesSource {
		t.Fatalf("completed row=%+v, capability should only indicate completing an unfinished row", completed)
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

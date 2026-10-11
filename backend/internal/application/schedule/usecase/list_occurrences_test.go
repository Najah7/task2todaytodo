package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
)

func TestProjectionKeepsRecurrencePhaseAndLocalEndOffsetAfterEffectiveDate(t *testing.T) {
	until := "2026-10-31"
	revision := dao.BlueprintRevision{Blueprint: dao.Blueprint{
		ID: "blueprint", UserID: "owner", Revision: 2, EffectiveFrom: "2026-10-10",
		RecurrenceAnchorDate: "2026-10-01", LocalStart: "2026-10-01 23:00:00",
		LocalEnd: "2026-10-02 01:00:00", Timezone: "UTC", Frequency: "daily", Interval: 2,
		Title: "overnight", AssigneeID: "owner",
	}, RevisionID: "revision-2", EffectiveUntil: &until}
	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	rows, err := projectOccurrences([]dao.BlueprintRevision{revision}, nil, nil, from, to, false)
	if err != nil {
		t.Fatalf("projectOccurrences() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("projected %d occurrences, want cadence dates 2026-10-11 and 2026-10-13: %+v", len(rows), rows)
	}
	wantDates := []string{"2026-10-11", "2026-10-13"}
	for i, row := range rows {
		if row.OccurrenceDate != wantDates[i] {
			t.Errorf("occurrence %d date = %s, want %s", i, row.OccurrenceDate, wantDates[i])
		}
		start := time.Unix(row.StartAt, 0).UTC()
		end := time.Unix(row.EndAt, 0).UTC()
		if start.Hour() != 23 || end.Day() != start.Day()+1 || end.Hour() != 1 {
			t.Errorf("occurrence %s lost overnight local span: start=%s end=%s", row.OccurrenceDate, start, end)
		}
	}
}

func TestAllDayProjectionUsesLocalCalendarDaysAcrossDST(t *testing.T) {
	revision := dao.BlueprintRevision{Blueprint: dao.Blueprint{
		ID: "blueprint", UserID: "owner", Revision: 1, EffectiveFrom: "2026-03-08",
		RecurrenceAnchorDate: "2026-03-08", LocalStart: "2026-03-08 00:00:00",
		LocalEnd: "2026-03-10 00:00:00", Timezone: "America/New_York", Frequency: "once",
		AllDay: true, Title: "two days", AssigneeID: "owner",
	}, RevisionID: "revision-1"}
	from := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)
	rows, err := projectOccurrences([]dao.BlueprintRevision{revision}, nil, nil, from, to, false)
	if err != nil {
		t.Fatalf("projectOccurrences() error = %v", err)
	}
	if len(rows) != 1 || !rows[0].AllDay {
		t.Fatalf("projected occurrences = %+v, want one all-day occurrence", rows)
	}
	start, end := time.Unix(rows[0].StartAt, 0), time.Unix(rows[0].EndAt, 0)
	if got := end.Sub(start); got != 47*time.Hour {
		t.Errorf("UTC duration = %s, want 47h for two local days spanning spring DST", got)
	}
	if got := start.In(mustLocation(t, "America/New_York")).Format("2006-01-02 15:04"); got != "2026-03-08 00:00" {
		t.Errorf("local start = %s", got)
	}
}

func TestOneDayAllDayProjectionSpansDSTBoundary(t *testing.T) {
	revision := dao.BlueprintRevision{Blueprint: dao.Blueprint{
		ID: "blueprint", UserID: "owner", Revision: 1, EffectiveFrom: "2026-03-08",
		RecurrenceAnchorDate: "2026-03-08", LocalStart: "2026-03-08 00:00:00",
		LocalEnd: "2026-03-09 00:00:00", Timezone: "America/New_York", Frequency: "once",
		AllDay: true, Title: "one day", AssigneeID: "owner",
	}, RevisionID: "revision-1"}
	from := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	rows, err := projectOccurrences([]dao.BlueprintRevision{revision}, nil, nil, from, from.Add(48*time.Hour), false)
	if err != nil || len(rows) != 1 {
		t.Fatalf("projected rows=%+v err=%v", rows, err)
	}
	if got := (rows[0].EndAt - rows[0].StartAt) / 3600; got != 23 {
		t.Errorf("one local day duration = %d hours, want 23 across DST", got)
	}
}

func TestIncludeSkippedProjectsSavedOccurrenceButKeepsDeletedHidden(t *testing.T) {
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	saved := []dao.ScheduleOccurrence{
		{BlueprintID: "skipped", OccurrenceDate: "2026-03-01", StartAt: from.Unix(), EndAt: from.Add(time.Hour).Unix(), Skipped: true},
		{BlueprintID: "deleted", OccurrenceDate: "2026-03-01", StartAt: from.Unix(), EndAt: from.Add(time.Hour).Unix(), Deleted: true},
	}
	without, err := projectOccurrences(nil, saved, nil, from, to, false)
	if err != nil || len(without) != 0 {
		t.Fatalf("default projection = %+v, err %v; want no rows", without, err)
	}
	with, err := projectOccurrences(nil, saved, nil, from, to, true)
	if err != nil || len(with) != 1 || with[0].BlueprintID != "skipped" {
		t.Fatalf("include-skipped projection = %+v, err %v; want skipped only", with, err)
	}
}

func TestAllDayOccurrenceToggleNormalizesOnlyOmittedBounds(t *testing.T) {
	occurrence := dao.ScheduleOccurrence{
		Timezone: "America/New_York",
		StartAt:  time.Date(2026, 3, 8, 9, 0, 0, 0, mustLocation(t, "America/New_York")).Unix(),
		EndAt:    time.Date(2026, 3, 8, 10, 0, 0, 0, mustLocation(t, "America/New_York")).Unix(),
	}
	if err := normalizeOccurrenceAllDayBounds(&occurrence, UpdateScheduleInput{}); err != nil {
		t.Fatalf("normalize omitted bounds: %v", err)
	}
	if got := (occurrence.EndAt - occurrence.StartAt) / 3600; got != 23 {
		t.Errorf("converted one-day duration = %d hours, want 23 across DST", got)
	}
	start := time.Date(2026, 3, 8, 9, 0, 0, 0, mustLocation(t, "America/New_York"))
	occurrence.StartAt = start.Unix()
	if err := normalizeOccurrenceAllDayBounds(&occurrence, UpdateScheduleInput{StartAt: &start}); err != domain.ErrBlueprintAllDayTime {
		t.Errorf("explicit non-midnight start error = %v, want all-day boundary error", err)
	}
	midnight := time.Date(2026, 3, 8, 0, 0, 0, 0, mustLocation(t, "America/New_York"))
	occurrence.StartAt, occurrence.EndAt = midnight.Unix(), midnight.Unix()
	if err := normalizeOccurrenceAllDayBounds(&occurrence, UpdateScheduleInput{StartAt: &midnight, EndAt: &midnight}); err != ErrOccurrenceTimeInvalid {
		t.Errorf("same explicit bounds error = %v, want invalid interval", err)
	}
}

func TestFollowingPermissionUsesActiveBlueprintStartInsteadOfMovedOccurrenceTime(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	revision := func() dao.BlueprintRevision {
		return dao.BlueprintRevision{Blueprint: dao.Blueprint{
			ID: "blueprint", EffectiveFrom: "2026-10-01", RecurrenceAnchorDate: "2026-10-01",
			LocalStart: "2026-10-01 09:00:00", LocalEnd: "2026-10-01 10:00:00",
			Timezone: "UTC", Frequency: "daily", Interval: 1,
		}, RevisionID: "revision"}
	}
	pastOccurrence := dao.ScheduleOccurrence{BlueprintID: "blueprint", OccurrenceDate: "2026-10-11", StartAt: now.Add(24 * time.Hour).Unix()}
	pastScheduledStart := activeOccurrenceStart([]dao.BlueprintRevision{revision()}, pastOccurrence)
	if followingAllowed(pastScheduledStart, now) {
		t.Error("moved-to-future occurrence should not grant following access when the rule start is past")
	}
	futureOccurrence := dao.ScheduleOccurrence{BlueprintID: "blueprint", OccurrenceDate: "2026-10-12", StartAt: now.Add(-24 * time.Hour).Unix()}
	futureScheduledStart := activeOccurrenceStart([]dao.BlueprintRevision{revision()}, futureOccurrence)
	if !followingAllowed(futureScheduledStart, now) {
		t.Error("moved-to-past occurrence should retain following access when the rule start is future")
	}
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return location
}

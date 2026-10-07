package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
)

func TestScheduleListKeepsRecurringRootAvailableAfterOccurrenceCursor(t *testing.T) {
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	startAt := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	root := dao.Schedule{
		ID: "series", UserID: "owner", AssigneeID: "owner", Title: "Weekly focus",
		IntervalWeeks: 1, Frequencies: []dao.Frequency{{Value: "mon"}}, RepeatState: repeatStateActive,
		FrequencyAnchorDate: rootDate.Unix(), StartAt: startAt.Unix(), EndAt: startAt.Add(time.Hour).Unix(),
		SeriesID: "series", OccurrenceDate: rootDate.Format("2006-01-02"), Timezone: "UTC",
		CursorStartAt: startAt.UTC().Format(time.RFC3339Nano),
	}
	asOf := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	firstPage, err := expandScheduleRows([]dao.Schedule{root}, CursorPageRequest{Size: 1}, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage) < 2 {
		t.Fatalf("expanded schedule rows = %d, want enough occurrences for first page and next marker", len(firstPage))
	}
	anchor := firstPage[0]
	secondPageRows, err := expandScheduleRows([]dao.Schedule{root}, CursorPageRequest{
		Size:   1,
		Anchor: &CursorAnchor{At: anchor.CursorStartAt, ID: anchor.ID, SeriesID: anchor.SeriesID, Date: anchor.OccurrenceDate},
	}, asOf)
	if err != nil {
		t.Fatal(err)
	}
	var after []dao.Schedule
	for _, row := range secondPageRows {
		if afterScheduleAnchor(row, &CursorAnchor{At: anchor.CursorStartAt, ID: anchor.ID, SeriesID: anchor.SeriesID, Date: anchor.OccurrenceDate}) {
			after = append(after, row)
		}
	}
	if len(after) == 0 || after[0].OccurrenceDate != "2026-10-12" {
		t.Fatalf("second page rows = %+v, want next virtual occurrence 2026-10-12", after)
	}
}

func TestScheduleListUsesMovedOverridesAndSkipsForCursorPages(t *testing.T) {
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	startAt := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	root := dao.Schedule{
		ID: "series", UserID: "owner", AssigneeID: "owner", Title: "Weekly focus",
		IntervalWeeks: 1, Frequencies: []dao.Frequency{{Value: "mon"}}, RepeatState: repeatStateActive,
		FrequencyAnchorDate: rootDate.Unix(), StartAt: startAt.Unix(), EndAt: startAt.Add(time.Hour).Unix(),
		SeriesID: "series", OccurrenceDate: rootDate.Format("2006-01-02"), Timezone: "UTC",
		CursorStartAt: startAt.UTC().Format(time.RFC3339Nano),
	}
	movedStart := time.Date(2026, 10, 13, 11, 0, 0, 0, time.UTC)
	override := root
	override.ID, override.OccurrenceDate, override.IsException = "moved", "2026-10-12", true
	override.StartAt, override.EndAt = movedStart.Unix(), movedStart.Add(time.Hour).Unix()
	skipped := map[string]map[string]bool{"series": {"2026-10-19": true}}
	asOf := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	anchor := &CursorAnchor{At: root.CursorStartAt, ID: root.ID, SeriesID: root.SeriesID, Date: root.OccurrenceDate}
	rows, err := expandScheduleRowsWithSkipped([]dao.Schedule{root, override}, CursorPageRequest{Size: 6, Anchor: anchor}, asOf, skipped)
	if err != nil {
		t.Fatal(err)
	}
	foundMoved, foundSkipped := false, false
	for _, row := range rows {
		if !afterScheduleAnchor(row, anchor) {
			continue
		}
		if row.ID == "moved" && row.StartAt == movedStart.Unix() {
			foundMoved = true
		}
		if row.OccurrenceDate == "2026-10-19" {
			foundSkipped = true
		}
	}
	if !foundMoved || foundSkipped {
		t.Fatalf("expanded rows = %+v; moved override found=%t skipped date found=%t", rows, foundMoved, foundSkipped)
	}
}

func TestCompletingRecurringRootDoesNotCompleteFutureVirtualOccurrences(t *testing.T) {
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	startAt := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	root := dao.Schedule{
		ID: "series", UserID: "owner", AssigneeID: "owner", Title: "Weekly focus", Completed: true,
		IntervalWeeks: 1, Frequencies: []dao.Frequency{{Value: "mon"}}, RepeatState: repeatStateActive,
		FrequencyAnchorDate: rootDate.Unix(), StartAt: startAt.Unix(), EndAt: startAt.Add(time.Hour).Unix(),
		SeriesID: "series", OccurrenceDate: rootDate.Format("2006-01-02"), Timezone: "UTC",
	}
	asOf := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	rows, err := expandScheduleRows([]dao.Schedule{root}, CursorPageRequest{Size: 3}, asOf)
	if err != nil {
		t.Fatal(err)
	}
	future := 0
	for _, row := range rows {
		if row.OccurrenceDate != "2026-10-05" {
			future++
			if row.Completed {
				t.Errorf("future virtual occurrence %s was completed", row.OccurrenceDate)
			}
		}
	}
	if future == 0 {
		t.Fatalf("expanded rows = %+v, want future occurrences after root completion", rows)
	}
}

func TestDeletedRootStillProjectsLiveOverrideWithRootOwnership(t *testing.T) {
	rootDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	startAt := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	root := dao.Schedule{
		ID: "series", UserID: "project-owner", ProjectID: "project", AssigneeID: "project-owner",
		Title: "Weekly focus", Deleted: true, IntervalWeeks: 1,
		Frequencies: []dao.Frequency{{Value: "mon"}}, RepeatState: repeatStateActive,
		FrequencyAnchorDate: rootDate.Unix(), StartAt: startAt.Unix(), EndAt: startAt.Add(time.Hour).Unix(),
		SeriesID: "series", OccurrenceDate: rootDate.Format("2006-01-02"), Timezone: "UTC",
	}
	override := root
	override.ID, override.OccurrenceDate, override.IsException = "saved", "2026-10-12", true
	override.UserID, override.ProjectID, override.AssigneeID = "stale-owner", "stale-project", "stale-assignee"
	override.Deleted = false
	override.StartAt = time.Date(2026, 10, 12, 11, 0, 0, 0, time.UTC).Unix()
	override.EndAt = override.StartAt + 3600
	rows, err := expandScheduleRows([]dao.Schedule{root, override}, CursorPageRequest{Size: 10}, time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == override.ID {
			if row.UserID != root.UserID || row.ProjectID != root.ProjectID || row.AssigneeID != root.AssigneeID {
				t.Fatalf("saved override ownership = (%s,%s,%s), want series root (%s,%s,%s)", row.UserID, row.ProjectID, row.AssigneeID, root.UserID, root.ProjectID, root.AssigneeID)
			}
			return
		}
	}
	t.Fatalf("live override was hidden after root deletion: %+v", rows)
}

package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
)

func recurringTodoRoot() dao.TodoItem {
	return dao.TodoItem{ID: "series", TaskID: "task", Title: "work", Position: 3, IntervalWeeks: 1, Frequencies: []dao.TaskFrequency{{Value: "mon"}}, RepeatState: repeatStateActive, FrequencyAnchorDate: mustParseDate("2026-10-05").Unix(), SeriesID: "series", OccurrenceDate: "2026-10-05", Timezone: "UTC"}
}
func TestTodoListClampsVirtualGenerationToLocalTodayAndKeepsRequestedSavedHistory(t *testing.T) {
	root := recurringTodoRoot()
	saved := root
	saved.ID = "edited"
	saved.OccurrenceDate = "2026-10-12"
	saved.IsException = true
	saved.Title = "edited history"
	asOf := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	rows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root, saved}, CursorPageRequest{Size: 20, FromDate: "2026-10-01"}, asOf, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	var dates []string
	foundSaved := false
	foundRoot := false
	for _, row := range rows {
		dates = append(dates, row.OccurrenceDate)
		if row.ID == "edited" {
			foundSaved = true
		}
		if row.ID == "series" && row.OccurrenceDate == "2026-10-05" {
			foundRoot = true
		}
		if row.ID == VirtualOccurrenceID && row.OccurrenceDate < "2026-10-19" {
			t.Fatalf("past virtual leaked: %#v", row)
		}
	}
	if !foundSaved || !foundRoot {
		t.Fatalf("saved past history missing, rows=%#v", rows)
	}
	rows, err = expandTodoItemRowsWithSkipped([]dao.TodoItem{root, saved}, CursorPageRequest{Size: 20}, asOf, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == "series" {
			t.Fatalf("untouched past root returned without explicit from_date: %#v", row)
		}
	}
	foundSaved = false
	for _, row := range rows {
		if row.ID == "edited" {
			foundSaved = true
		}
	}
	if !foundSaved {
		t.Fatalf("saved edited history disappeared: %#v", rows)
	}
}

func TestTodoListFrequencyAnchorIsPhaseNotGuaranteedOccurrence(t *testing.T) {
	root := recurringTodoRoot()
	root.OccurrenceDate = "2026-10-06"
	root.FrequencyAnchorDate = mustParseDate("2026-10-06").Unix()
	root.Frequencies = []dao.TaskFrequency{{Value: "mon"}}
	rows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root}, CursorPageRequest{Size: 3}, time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == VirtualOccurrenceID && row.OccurrenceDate == "2026-10-06" {
			t.Fatalf("Tuesday phase incorrectly emitted; rows=%#v", rows)
		}
	}
	if len(rows) < 2 || rows[1].OccurrenceDate != "2026-10-12" {
		t.Fatalf("Monday after Tuesday phase missing; rows=%#v", rows)
	}
}

func TestTodoListKeepsFutureRootSnapshotWhenFrequencyDoesNotMatchRootDay(t *testing.T) {
	root := recurringTodoRoot()
	root.OccurrenceDate = "2026-10-20"
	root.FrequencyAnchorDate = mustParseDate("2026-10-19").Unix()
	rows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root}, CursorPageRequest{Size: 5}, time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || rows[0].ID != root.ID || rows[0].OccurrenceDate != "2026-10-20" {
		t.Fatalf("first root snapshot omitted: %#v", rows)
	}
}

func TestTodoListVirtualPositionAdvancesFromBasePosition(t *testing.T) {
	root := recurringTodoRoot()
	rows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root}, CursorPageRequest{Size: 4}, time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 4 {
		t.Fatalf("rows=%#v", rows)
	}
	for i, row := range rows[:4] {
		if row.Position != root.Position+i {
			t.Errorf("%s position=%d want %d", row.OccurrenceDate, row.Position, root.Position+i)
		}
	}
}

func TestDeletedOccurrenceTombstonesSuppressVirtualsWithoutLeakingIntoProjection(t *testing.T) {
	t.Run("todo item", func(t *testing.T) {
		root := recurringTodoRoot()
		deleted := root
		deleted.ID = "deleted-occurrence"
		deleted.OccurrenceDate = "2026-10-12"
		deleted.Deleted = true
		rows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root, deleted}, CursorPageRequest{Size: 5}, time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC), false, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Deleted || row.OccurrenceDate == deleted.OccurrenceDate {
				t.Fatalf("deleted occurrence leaked or was regenerated: %#v", row)
			}
		}
	})
}

func TestTodoListPositionRemainsStableAcrossPages(t *testing.T) {
	root := recurringTodoRoot()
	asOf := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	rows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root}, CursorPageRequest{Size: 2, Anchor: &CursorAnchor{OccurrenceDate: "2026-10-12", SeriesID: "series", AsOf: asOf.Format(time.RFC3339Nano)}}, asOf, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.OccurrenceDate == "2026-10-19" {
			if row.Position != 5 {
				t.Fatalf("page two position=%d want 5", row.Position)
			}
			return
		}
	}
	t.Fatalf("page two missing expected occurrence: %#v", rows)
}

func TestOneOffListsSavedCurrentOverrideInsteadOfRootSnapshot(t *testing.T) {
	root := recurringTodoRoot()
	root.IntervalWeeks, root.Frequencies, root.RepeatState = 0, nil, repeatStateOneOff
	child := root
	child.ID, child.Title, child.IsException = "edited-child", "edited", true
	rows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root, child}, CursorPageRequest{Size: 5}, mustParseDate(root.OccurrenceDate), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != child.ID || rows[0].Title != child.Title {
		t.Fatalf("one-off override projection=%#v", rows)
	}
}

func TestReopenedTaskCanProjectTodayAgainUnlessSkipped(t *testing.T) {
	root := recurringTodoRoot()
	asOf := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	doneRows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root}, CursorPageRequest{Size: 5}, asOf, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(doneRows) != 0 {
		t.Fatalf("done task virtual rows=%#v", doneRows)
	}
	reopenedRows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root}, CursorPageRequest{Size: 5}, asOf, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopenedRows) == 0 || reopenedRows[0].OccurrenceDate != "2026-10-05" {
		t.Fatalf("reopened task did not resume today: %#v", reopenedRows)
	}
	skipped := map[string]map[string]bool{"series": {"2026-10-05": true}}
	restoredRows, err := expandTodoItemRowsWithSkipped([]dao.TodoItem{root}, CursorPageRequest{Size: 5}, asOf, false, skipped)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range restoredRows {
		if row.OccurrenceDate == "2026-10-05" {
			t.Fatalf("skipped occurrence reappeared: %#v", restoredRows)
		}
	}
}

func TestDoneProjectSuppressesTodoVirtualsButKeepsSavedOccurrences(t *testing.T) {
	root := recurringTodoRoot()
	asOf := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	request := CursorPageRequest{Size: 10, FromDate: "2026-10-07"}
	rows, err := expandTodoItemRowsWithProjectState([]dao.TodoItem{root}, request, asOf, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("done Project emitted virtual TodoItems: %#v", rows)
	}

	saved := root
	saved.ID, saved.OccurrenceDate, saved.Completed, saved.IsException = "saved-occurrence", "2026-10-12", true, true
	rows, err = expandTodoItemRowsWithProjectState([]dao.TodoItem{root, saved}, request, asOf, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != saved.ID || rows[0].OccurrenceDate != saved.OccurrenceDate {
		t.Fatalf("done Project saved-occurrence projection = %#v; want saved occurrence only", rows)
	}
}

package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
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
	rows, err := projectOccurrences([]dao.BlueprintRevision{revision}, nil, nil, from, to)
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

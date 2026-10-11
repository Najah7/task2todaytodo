package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/inbox/dao"
)

type inboxLoggerFake struct {
	errors int
	fields []any
}

func (*inboxLoggerFake) Debug(context.Context, string, ...any) {}
func (*inboxLoggerFake) Info(context.Context, string, ...any)  {}
func (*inboxLoggerFake) Warn(context.Context, string, ...any)  {}
func (f *inboxLoggerFake) Error(_ context.Context, _ string, fields ...any) {
	f.errors++
	f.fields = append([]any(nil), fields...)
}

type candidateReaderFake struct {
	timezone string
	items    []dao.Item
	err      error
	reads    int
}

func (f *candidateReaderFake) Timezone(context.Context, string) (string, error) {
	return f.timezone, f.err
}

func (f *candidateReaderFake) ReadCandidates(_ context.Context, _ string, _ Snapshot, _ CandidateRange) ([]dao.Item, error) {
	f.reads++
	return append([]dao.Item(nil), f.items...), f.err
}

func TestAllDayInboxWindowUsesScheduleCalendarDatesAcrossDST(t *testing.T) {
	viewer, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	startDate, _ := time.Parse("2006-01-02", "2026-03-08")
	windowStart := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, viewer)
	windowEnd := windowStart.AddDate(0, 0, 7)
	if got := windowEnd.Sub(windowStart); got != 167*time.Hour {
		t.Fatalf("seven local dates span %s, want 167h across spring DST", got)
	}

	// Kiritimati midnight on March 8 is still March 7 in New York. The
	// all-day calendar date must remain March 8 when compared with the Inbox.
	kiritimati, err := time.LoadLocation("Pacific/Kiritimati")
	if err != nil {
		t.Fatal(err)
	}
	scheduleStart := time.Date(2026, time.March, 8, 0, 0, 0, 0, kiritimati)
	scheduleEnd := time.Date(2026, time.March, 9, 0, 0, 0, 0, kiritimati)
	item := &dao.SchedulePayload{AllDay: true, Timezone: kiritimati.String(), StartAt: scheduleStart.Unix(), EndAt: scheduleEnd.Unix()}
	if !scheduleOverlapsInboxWindow(item, windowStart, windowEnd) {
		t.Fatal("all-day occurrence on viewer's first date was excluded after timezone conversion")
	}
}

func TestListFiltersAndCountsBeforeTypeAndPagination(t *testing.T) {
	reader := &candidateReaderFake{timezone: "UTC", items: []dao.Item{
		{Type: dao.ItemTypeTask, ID: "t1", Title: "Call dentist", ProjectID: "p1", CreatedAt: "2026-10-12T03:00:00.000000001Z", Task: &dao.TaskPayload{}},
		{Type: dao.ItemTypeSchedule, ID: "s1", Title: "dentist followup", ProjectID: "p1", CreatedAt: "2026-10-12T02:00:00Z", Schedule: &dao.SchedulePayload{BlueprintID: "b1", OccurrenceDate: "2026-10-13", StartAt: time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC).Unix(), EndAt: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC).Unix(), Timezone: "UTC"}},
		{Type: dao.ItemTypeTask, ID: "t2", Title: "Buy milk", ProjectID: "p2", CreatedAt: "2026-10-12T01:00:00Z", Task: &dao.TaskPayload{}},
	}}
	page, err := NewListUseCase(reader, nil, func() time.Time { return time.Date(2026, 10, 12, 4, 0, 0, 0, time.UTC) }).Execute(context.Background(), "user-1", ListRequest{
		Size: 1, TypeFilter: "task", ProjectFilter: ProjectFilter{Mode: ProjectFilterProject, ProjectID: "p1"}, Title: "DENTIST",
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Counts != (Counts{All: 2, Task: 1, Schedule: 1}) {
		t.Fatalf("counts = %#v", page.Counts)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "t1" || page.Next != nil || page.Previous != nil {
		t.Fatalf("page = %#v", page)
	}
	if page.Snapshot.Today != "2026-10-12" || page.Snapshot.Timezone != "UTC" || !page.Snapshot.AsOf.Equal(time.Date(2026, 10, 12, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("snapshot = %#v", page.Snapshot)
	}
}

func TestListMaintainsSnapshotAndBidirectionalCursor(t *testing.T) {
	reader := &candidateReaderFake{timezone: "UTC", items: []dao.Item{
		{Type: dao.ItemTypeTask, ID: "a", CreatedAt: "2026-10-12T03:00:00.000001Z", Task: &dao.TaskPayload{}},
		{Type: dao.ItemTypeSchedule, ID: "b/2026-10-12", CreatedAt: "2026-10-12T03:00:00.000001Z", Schedule: &dao.SchedulePayload{BlueprintID: "b", OccurrenceDate: "2026-10-12", StartAt: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC).Unix(), EndAt: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC).Unix(), Timezone: "UTC"}},
		{Type: dao.ItemTypeTask, ID: "c", CreatedAt: "2026-10-12T03:00:00Z", Task: &dao.TaskPayload{}},
	}}
	now := func() time.Time { return time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC) }
	uc := NewListUseCase(reader, nil, now)
	first, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1})
	if err != nil {
		t.Fatal(err)
	}
	if first.Items[0].ID != "a" || first.Next == nil || first.Previous != nil {
		t.Fatalf("first page = %#v", first)
	}
	second, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, Snapshot: first.Snapshot, Anchor: first.Next})
	if err != nil {
		t.Fatal(err)
	}
	if second.Items[0].ID != "b/2026-10-12" || second.Previous == nil || second.Next == nil {
		t.Fatalf("second page = %#v", second)
	}
	back, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, Snapshot: first.Snapshot, Anchor: second.Previous})
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Items) != 1 || back.Items[0].ID != first.Items[0].ID || reader.reads != 3 {
		t.Fatalf("back page = %#v; reads=%d", back, reader.reads)
	}
}

func TestListOrdersPreciseCreationTimesAndStableScheduleStartsAcrossPages(t *testing.T) {
	reader := &candidateReaderFake{timezone: "UTC", items: []dao.Item{
		{Type: dao.ItemTypeTask, ID: "exact", CreatedAt: "2026-10-12T03:00:00Z", Task: &dao.TaskPayload{}},
		{Type: dao.ItemTypeTask, ID: "fraction", CreatedAt: "2026-10-12T03:00:00.000000001Z", Task: &dao.TaskPayload{}},
		{Type: dao.ItemTypeSchedule, ID: "series/late", CreatedAt: "2026-10-12T02:00:00Z", Schedule: &dao.SchedulePayload{BlueprintID: "series", OccurrenceDate: "2026-10-13", StartAt: time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC).Unix(), EndAt: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC).Unix(), Timezone: "UTC"}},
		{Type: dao.ItemTypeSchedule, ID: "series/early", CreatedAt: "2026-10-12T02:00:00Z", Schedule: &dao.SchedulePayload{BlueprintID: "series", OccurrenceDate: "2026-10-12", StartAt: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC).Unix(), EndAt: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC).Unix(), Timezone: "UTC"}},
	}}
	uc := NewListUseCase(reader, nil, func() time.Time { return time.Date(2026, 10, 12, 4, 0, 0, 0, time.UTC) })
	page, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1})
	if err != nil || page.Items[0].ID != "fraction" {
		t.Fatalf("first page = %#v, err=%v", page, err)
	}
	page, err = uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, Snapshot: page.Snapshot, Anchor: page.Next})
	if err != nil || page.Items[0].ID != "exact" {
		t.Fatalf("second page = %#v, err=%v", page, err)
	}
	page, err = uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, Snapshot: page.Snapshot, Anchor: page.Next})
	if err != nil || page.Items[0].ID != "series/early" || page.Next == nil || page.Next.ScheduleStartAt != time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("third page = %#v, err=%v", page, err)
	}
	page, err = uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, Snapshot: page.Snapshot, Anchor: page.Next})
	if err != nil || page.Items[0].ID != "series/late" {
		t.Fatalf("fourth page = %#v, err=%v", page, err)
	}
	back, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, Snapshot: page.Snapshot, Anchor: page.Previous})
	if err != nil || len(back.Items) != 1 || back.Items[0].ID != "series/early" {
		t.Fatalf("back page = %#v, err=%v", back, err)
	}
}

func TestListRejectsInvalidFilterAndPropagatesCandidateFailure(t *testing.T) {
	reader := &candidateReaderFake{timezone: "UTC"}
	uc := NewListUseCase(reader, nil, nil)
	if _, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, TypeFilter: "project"}); !errors.Is(err, ErrInvalidFilter) {
		t.Fatalf("invalid type filter error = %v", err)
	}
	want := errors.New("read failed")
	reader.err = want
	if _, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1}); !errors.Is(err, want) {
		t.Fatalf("candidate error = %v", err)
	}
}

func TestListLogsUnexpectedFailureOnceAndSkipsExpectedFailures(t *testing.T) {
	snapshot := Snapshot{AsOf: time.Date(2026, 10, 12, 4, 0, 0, 0, time.UTC), Timezone: "UTC", Today: "2026-10-12"}
	t.Run("unexpected", func(t *testing.T) {
		logger := &inboxLoggerFake{}
		reader := &candidateReaderFake{err: errors.New("database unavailable")}
		uc := NewListUseCase(reader, logger, nil)
		_, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, Snapshot: snapshot})
		if err == nil || logger.errors != 1 {
			t.Fatalf("err=%v, error logs=%d; want failure and exactly one log", err, logger.errors)
		}
		if got := fieldValue(logger.fields, "operation"); got != "ListUseCase.Execute" {
			t.Fatalf("operation field = %v", got)
		}
		if got := fieldValue(logger.fields, "error_type"); got != "*errors.errorString" {
			t.Fatalf("error_type field = %v", got)
		}
	})
	t.Run("validation and cancellation", func(t *testing.T) {
		logger := &inboxLoggerFake{}
		reader := &candidateReaderFake{}
		uc := NewListUseCase(reader, logger, nil)
		if _, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, TypeFilter: "unknown"}); !errors.Is(err, ErrInvalidFilter) {
			t.Fatalf("validation error = %v", err)
		}
		reader.err = context.Canceled
		if _, err := uc.Execute(context.Background(), "user-1", ListRequest{Size: 1, Snapshot: snapshot}); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v", err)
		}
		if logger.errors != 0 {
			t.Fatalf("error logs = %d; want none", logger.errors)
		}
	})
}

func fieldValue(fields []any, key string) any {
	for i := 0; i+1 < len(fields); i += 2 {
		if fields[i] == key {
			return fields[i+1]
		}
	}
	return nil
}

func TestFilterProjectModes(t *testing.T) {
	items := []dao.Item{{ID: "none"}, {ID: "p1", ProjectID: "p1"}, {ID: "p2", ProjectID: "p2"}}
	if got := itemIDs(filterItems(items, ProjectFilter{Mode: ProjectFilterNoProject}, "")); !reflect.DeepEqual(got, []string{"none"}) {
		t.Fatalf("no project filter = %v", got)
	}
	if got := itemIDs(filterItems(items, ProjectFilter{Mode: ProjectFilterProject, ProjectID: "p2"}, "")); !reflect.DeepEqual(got, []string{"p2"}) {
		t.Fatalf("specific project filter = %v", got)
	}
}

func itemIDs(items []dao.Item) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

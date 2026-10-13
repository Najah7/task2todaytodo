package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/metric/dao"
)

type todoCompletionReaderFake struct {
	gotActor string
	gotDays  int
	result   dao.TodoCompletion
	err      error
	calls    int
}

func (f *todoCompletionReaderFake) ReadTodoCompletion(_ context.Context, actor string, days int) (dao.TodoCompletion, error) {
	f.calls++
	f.gotActor, f.gotDays = actor, days
	return f.result, f.err
}

func TestTodoCompletionUseCaseValidatesDaysAndActor(t *testing.T) {
	reader := &todoCompletionReaderFake{}
	uc := NewTodoCompletionUseCase(reader, paidPlanAccess{})
	for _, days := range []int{0, 6, 8, 14, 31} {
		if _, err := uc.Execute(context.Background(), "actor", days); !errors.Is(err, ErrInvalidDays) {
			t.Errorf("days=%d error=%v, want ErrInvalidDays", days, err)
		}
	}
	if _, err := uc.Execute(context.Background(), "", 7); !errors.Is(err, ErrInvalidActor) {
		t.Fatalf("empty actor error=%v, want ErrInvalidActor", err)
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls=%d, want zero for invalid requests", reader.calls)
	}
}

func TestTodoCompletionUseCasePassesSupportedPeriodAndResult(t *testing.T) {
	want := dao.TodoCompletion{
		Timezone: "Asia/Tokyo", StartDate: "2026-10-07", EndDate: "2026-10-13",
		Days: []dao.CompletionDay{
			{Date: "2026-10-07", PlannedItems: 3, CompletedItems: 2},
			{Date: "2026-10-08", PlannedItems: 1, CompletedItems: 0},
			{Date: "2026-10-09", PlannedItems: 0, CompletedItems: 0},
		},
	}
	reader := &todoCompletionReaderFake{result: want}
	got, err := NewTodoCompletionUseCase(reader, paidPlanAccess{}).Execute(context.Background(), "user-1", 7)
	if err != nil {
		t.Fatal(err)
	}
	if reader.gotActor != "user-1" || reader.gotDays != 7 || reader.calls != 1 {
		t.Fatalf("reader call actor/days/calls=%q/%d/%d", reader.gotActor, reader.gotDays, reader.calls)
	}
	if got.Timezone != want.Timezone || got.StartDate != want.StartDate || got.EndDate != want.EndDate {
		t.Fatalf("result=%+v, want %+v", got, want)
	}
	if got.TotalPlannedItems != 4 || got.CompletedItems != 2 || got.CompletionRatio == nil || *got.CompletionRatio != 0.5 {
		t.Fatalf("period completion=%+v, want weighted 1/2 ratio", got)
	}
	if got.Days[0].CompletionRatio == nil || *got.Days[0].CompletionRatio != (2.0/3.0) || got.Days[1].CompletionRatio == nil || *got.Days[1].CompletionRatio != 0 || got.Days[2].CompletionRatio != nil {
		t.Fatalf("daily ratios=%+v, want computed and null zero-day ratios", got.Days)
	}
	if _, err := NewTodoCompletionUseCase(reader, paidPlanAccess{}).Execute(context.Background(), "user-1", 30); err != nil {
		t.Fatalf("30-day period error=%v", err)
	}
}

package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/metric/dao"
)

type overdueWorkReaderFake struct {
	result dao.OverdueWork
	err    error
	actor  string
	calls  int
}

func (f *overdueWorkReaderFake) ReadOverdueWork(_ context.Context, actor string) (dao.OverdueWork, error) {
	f.calls++
	f.actor = actor
	return f.result, f.err
}

func TestOverdueWorkUseCaseSortsProjectsAndAggregatesRemainder(t *testing.T) {
	projects := []dao.OverdueProject{
		{ProjectTitle: strPtr("Delta"), Count: 2},
		{ProjectID: strPtr("personal"), Count: 1},
		{ProjectTitle: strPtr("Bravo"), Count: 4},
		{ProjectTitle: strPtr("Charlie"), Count: 3},
		{ProjectTitle: strPtr("Echo"), Count: 1},
		{ProjectTitle: strPtr("Alpha"), Count: 5},
		{ProjectTitle: strPtr("Foxtrot"), Count: 1},
	}
	reader := &overdueWorkReaderFake{result: dao.OverdueWork{Timezone: "UTC", Projects: projects}}
	got, err := NewOverdueWorkUseCase(reader).Execute(context.Background(), "actor")
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 || reader.actor != "actor" {
		t.Fatalf("reader calls/actor=%d/%q", reader.calls, reader.actor)
	}
	if len(got.Projects) != OverdueProjectLimit || got.OtherCount != 2 {
		t.Fatalf("projects/other_count=%d/%d, want 5/2", len(got.Projects), got.OtherCount)
	}
	wantTitles := []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"}
	for index, title := range wantTitles {
		if got.Projects[index].ProjectTitle == nil || *got.Projects[index].ProjectTitle != title {
			t.Fatalf("project[%d]=%+v, want title %q", index, got.Projects[index], title)
		}
	}
}

func TestOverdueWorkUseCaseKeepsPersonalGroupAndEmptyBreakdown(t *testing.T) {
	reader := &overdueWorkReaderFake{result: dao.OverdueWork{Projects: []dao.OverdueProject{{Count: 2}}}}
	got, err := NewOverdueWorkUseCase(reader).Execute(context.Background(), "actor")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Projects) != 1 || got.Projects[0].ProjectID != nil || got.Projects[0].ProjectTitle != nil || got.OtherCount != 0 {
		t.Fatalf("personal breakdown=%+v other=%d", got.Projects, got.OtherCount)
	}

	reader.result.Projects = nil
	got, err = NewOverdueWorkUseCase(reader).Execute(context.Background(), "actor")
	if err != nil || got.Projects == nil || len(got.Projects) != 0 {
		t.Fatalf("empty breakdown=%+v err=%v, want non-nil empty projects", got.Projects, err)
	}
}

func strPtr(value string) *string { return &value }

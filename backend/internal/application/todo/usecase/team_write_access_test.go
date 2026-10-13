package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
)

type testProjectWriteAccessReader struct {
	values map[string]shared.ProjectWriteAccess
	calls  map[string]int
}

func (reader *testProjectWriteAccessReader) GetProjectWriteAccess(_ context.Context, _, projectID string) (shared.ProjectWriteAccess, error) {
	reader.calls[projectID]++
	return reader.values[projectID], nil
}

type testProjectWriteAccessRepositories struct {
	Repositories
	reader shared.ProjectWriteAccessReader
}

func (repositories testProjectWriteAccessRepositories) ProjectWriteAccess() shared.ProjectWriteAccessReader {
	return repositories.reader
}

func TestFilterSharedPlanningCandidatesRequiresCurrentProjectWriteAccess(t *testing.T) {
	reader := &testProjectWriteAccessReader{
		values: map[string]shared.ProjectWriteAccess{
			"personal": {MemberCount: 0},
			"shared":   {MemberCount: 2, ActorPlanAccess: shared.PlanAccessTeam, OwnerPlanAccess: shared.PlanAccessTeam},
			"blocked":  {MemberCount: 2, ActorPlanAccess: shared.PlanAccessPro, OwnerPlanAccess: shared.PlanAccessTeam},
		},
		calls: make(map[string]int),
	}
	repositories := testProjectWriteAccessRepositories{reader: reader}
	candidates := []dao.WorkSource{
		{ID: "standalone", Kind: "action_item"},
		{ID: "personal-work", ProjectID: "personal"},
		{ID: "shared-work", ProjectID: "shared"},
		{ID: "blocked-work", ProjectID: "blocked"},
		{ID: "shared-again", ProjectID: "shared"},
	}

	got, err := filterSharedPlanningCandidates(context.Background(), repositories, "actor", candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[3].ID != "shared-again" {
		t.Fatalf("filtered candidates = %#v, want standalone/personal/shared/shared-again", got)
	}
	if reader.calls["shared"] != 1 || reader.calls["blocked"] != 1 || reader.calls["personal"] != 1 {
		t.Fatalf("Project write access queried more than once per Project: %#v", reader.calls)
	}
}

func TestFilterSharedPlanningCandidatesFailsClosedWithoutReader(t *testing.T) {
	got, err := filterSharedPlanningCandidates(context.Background(), testProjectWriteAccessRepositories{}, "actor", []dao.WorkSource{
		{ID: "standalone"}, {ID: "project-work", ProjectID: "shared"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "standalone" {
		t.Fatalf("filtered candidates = %#v, want standalone only", got)
	}
}

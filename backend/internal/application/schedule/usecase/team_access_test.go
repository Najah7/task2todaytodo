package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type teamAccessTestRepositories struct {
	Repositories
	reader shared.ProjectWriteAccessReader
}

func (r teamAccessTestRepositories) ProjectWriteAccess() shared.ProjectWriteAccessReader {
	return r.reader
}

type teamAccessTestReader struct{ access shared.ProjectWriteAccess }

func (r teamAccessTestReader) GetProjectWriteAccess(context.Context, string, string) (shared.ProjectWriteAccess, error) {
	return r.access, nil
}

func TestRestrictScheduleWriteCapabilitiesWhenOwnerLosesTeam(t *testing.T) {
	occurrence := dao.ScheduleOccurrence{
		ProjectID: "project-1", CanUpdate: true, CanDelete: true, CanSkip: true,
		CanAssign: true, CanUpdateFollowing: true, CanDeleteFollowing: true,
	}
	repositories := teamAccessTestRepositories{reader: teamAccessTestReader{access: shared.ProjectWriteAccess{
		MemberCount: 1, ActorPlanAccess: shared.PlanAccessTeam, OwnerPlanAccess: shared.PlanAccessFree,
	}}}
	if err := restrictScheduleWriteCapabilities(context.Background(), repositories, "member-1", &occurrence); err != nil {
		t.Fatal(err)
	}
	if occurrence.CanUpdate || occurrence.CanDelete || occurrence.CanSkip || occurrence.CanAssign || occurrence.CanUpdateFollowing || occurrence.CanDeleteFollowing {
		t.Fatalf("shared Project capabilities remain writable: %+v", occurrence)
	}
}

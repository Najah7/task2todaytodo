package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
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

func TestRestrictTaskWriteCapabilitiesWhenSharedProjectLosesTeam(t *testing.T) {
	task := dao.Task{ProjectID: "project-1", CanUpdate: true, CanAssign: true, CanAddActionItem: true, CanAddToToday: true, EstimateEditable: true}
	repositories := teamAccessTestRepositories{reader: teamAccessTestReader{access: shared.ProjectWriteAccess{
		MemberCount: 1, ActorPlanAccess: shared.PlanAccessTeam, OwnerPlanAccess: shared.PlanAccessPro,
	}}}
	if err := restrictTaskWriteCapabilities(context.Background(), repositories, "member-1", &task); err != nil {
		t.Fatal(err)
	}
	if task.CanUpdate || task.CanAssign || task.CanAddActionItem || task.CanAddToToday || task.EstimateEditable {
		t.Fatalf("shared Project capabilities remain writable: %+v", task)
	}
}

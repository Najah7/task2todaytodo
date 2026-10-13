package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type projectWriteAccessReaderStub struct {
	access shared.ProjectWriteAccess
	err    error
}

func (reader projectWriteAccessReaderStub) GetProjectWriteAccess(context.Context, string, string) (shared.ProjectWriteAccess, error) {
	return reader.access, reader.err
}

func TestProjectWriteAccessAllowsPersonalPlansAndRequiresTeamForSharedProject(t *testing.T) {
	tests := []struct {
		name   string
		access shared.ProjectWriteAccess
		want   bool
	}{
		{name: "personal free", access: shared.ProjectWriteAccess{MemberCount: 0, ActorPlanAccess: shared.PlanAccessFree, OwnerPlanAccess: shared.PlanAccessFree}, want: true},
		{name: "shared actor free", access: shared.ProjectWriteAccess{MemberCount: 1, ActorPlanAccess: shared.PlanAccessFree, OwnerPlanAccess: shared.PlanAccessTeam}},
		{name: "shared owner free", access: shared.ProjectWriteAccess{MemberCount: 1, ActorPlanAccess: shared.PlanAccessTeam, OwnerPlanAccess: shared.PlanAccessPro}},
		{name: "shared team", access: shared.ProjectWriteAccess{MemberCount: 1, ActorPlanAccess: shared.PlanAccessTeam, OwnerPlanAccess: shared.PlanAccessTeam}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.access.AllowsWrite(); got != test.want {
				t.Fatalf("AllowsWrite() = %v, want %v", got, test.want)
			}
		})
	}
}

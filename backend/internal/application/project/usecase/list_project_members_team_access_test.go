package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type projectRosterAccessRepository struct {
	Repository
	actorRole string
	access    shared.ProjectWriteAccess
}

func (*projectRosterAccessRepository) LockByUserIDWithPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (dao.Project, error) {
	return dao.Project{}, nil
}

func (repo *projectRosterAccessRepository) GetProjectMemberActorRole(context.Context, domain.UserID, domain.ProjectID) (string, error) {
	return repo.actorRole, nil
}

func (*projectRosterAccessRepository) ListProjectRosterPage(context.Context, domain.UserID, domain.ProjectID, int, *CursorAnchor) (dao.ProjectRosterPage, error) {
	return dao.ProjectRosterPage{Items: []dao.ProjectMember{
		{UserID: "owner-1", CanEditRole: false, CanRemove: false},
		{UserID: "member-1", CanEditRole: true, CanRemove: true},
		{UserID: "", Status: "pending", InvitationID: "invitation-1", CanEditRole: true, CanRemove: true},
	}}, nil
}

func (repo *projectRosterAccessRepository) GetProjectWriteAccess(context.Context, string, string) (shared.ProjectWriteAccess, error) {
	return repo.access, nil
}

type projectRosterAccessUOW struct {
	repo *projectRosterAccessRepository
}

func (uow projectRosterAccessUOW) Do(ctx context.Context, callback func(context.Context, Repository, ProjectChildrenDeleter) error) error {
	return callback(ctx, uow.repo, nil)
}

func TestListProjectMembersHidesWriteActionsAfterOwnerTeamLossButAllowsOwnerRemoval(t *testing.T) {
	repo := &projectRosterAccessRepository{
		actorRole: "owner",
		access: shared.ProjectWriteAccess{
			OwnerUserID:     "owner-1",
			MemberCount:     1,
			ActorPlanAccess: shared.PlanAccessFree,
			OwnerPlanAccess: shared.PlanAccessFree,
		},
	}
	page, err := NewListProjectMembersUseCase(projectRosterAccessUOW{repo: repo}, nil).Execute(
		context.Background(), "owner-1", "project-1", ProjectRosterPageRequest{},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if page.CanInvite || len(page.AssignableRoles) != 0 {
		t.Fatalf("invite=%v assignable roles=%v; want invite and role changes hidden", page.CanInvite, page.AssignableRoles)
	}
	if len(page.Items) != 3 {
		t.Fatalf("items=%d; want 3", len(page.Items))
	}
	if page.Items[0].CanEditRole || page.Items[0].CanRemove || page.Items[1].CanEditRole || !page.Items[1].CanRemove || page.Items[2].CanEditRole || page.Items[2].CanRemove {
		t.Fatalf("member controls = %#v; want accepted member removable only", page.Items)
	}
}

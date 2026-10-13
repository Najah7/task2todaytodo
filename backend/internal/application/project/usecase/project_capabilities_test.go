package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type projectReadAccessRepository struct {
	Repository
	projects map[domain.ProjectID]dao.Project
}

func (repo *projectReadAccessRepository) GetByUserID(_ context.Context, _ domain.UserID, id domain.ProjectID) (dao.Project, error) {
	return repo.projects[id], nil
}

func (*projectReadAccessRepository) HasPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (bool, error) {
	return true, nil
}

func (repo *projectReadAccessRepository) GetProjectWriteAccess(_ context.Context, actorID, projectID string) (shared.ProjectWriteAccess, error) {
	return shared.ProjectWriteAccess{
		OwnerUserID:     "owner-1",
		MemberCount:     1,
		ActorPlanAccess: shared.PlanAccessFree,
		OwnerPlanAccess: shared.PlanAccessTeam,
	}, nil
}

type projectReadAccessUnitOfWork struct{ repo *projectReadAccessRepository }

func (uow projectReadAccessUnitOfWork) Do(ctx context.Context, callback func(context.Context, Repository, ProjectChildrenDeleter) error) error {
	return callback(ctx, uow.repo, nil)
}

func TestListProjectsUseCaseHidesWriteCapabilitiesForSharedProjectWithoutTeam(t *testing.T) {
	const userID = domain.UserID("member-1")
	project := dao.Project{ID: "project-1", UserID: "owner-1", CanUpdate: true, CanDelete: true}
	repo := &listProjectsRepository{projectsByUserID: map[domain.UserID][]dao.Project{userID: {project}}}
	reader := &projectReadAccessRepository{}

	got, err := NewListProjectsUseCase(repo, &projectProgressSourceFake{}, nil, projectReadAccessUnitOfWork{repo: reader}).Execute(context.Background(), userID)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(got) != 1 || got[0].CanUpdate || got[0].CanDelete {
		t.Fatalf("write capabilities = (%v, %v), want both false", got[0].CanUpdate, got[0].CanDelete)
	}
}

func TestFilteredProjectListHidesWriteCapabilitiesForSharedProjectWithoutTeam(t *testing.T) {
	const userID = domain.UserID("member-1")
	repo := &filteredProjectListRepositoryFake{
		pageRows: []dao.Project{{ID: "project-1", UserID: "owner-1", CanUpdate: true, CanDelete: true}},
		summary:  dao.ProjectListSummary{Today: "2026-10-08", Timezone: "Asia/Tokyo"},
	}
	reader := &projectReadAccessRepository{}
	page, err := NewListProjectsUseCase(repo, &listPageProgressReaderFake{}, nil, projectReadAccessUnitOfWork{repo: reader}).ExecuteFilteredPage(
		context.Background(), userID, ProjectListRequest{Size: 10, SortBy: "title", SortOrder: "asc", AsOf: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)},
	)
	if err != nil {
		t.Fatalf("ExecuteFilteredPage() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].CanUpdate || page.Items[0].CanDelete {
		t.Fatalf("write capabilities = (%v, %v), want both false", page.Items[0].CanUpdate, page.Items[0].CanDelete)
	}
}

func TestGetProjectUseCaseHidesWriteCapabilitiesForSharedProjectWithoutTeam(t *testing.T) {
	const userID = domain.UserID("member-1")
	const projectID = domain.ProjectID("project-1")
	reader := &projectReadAccessRepository{projects: map[domain.ProjectID]dao.Project{
		projectID: {ID: string(projectID), UserID: "owner-1", CanUpdate: true, CanDelete: true},
	}}

	got, err := NewGetProjectUseCase(reader, &projectProgressSourceFake{}, nil, projectReadAccessUnitOfWork{repo: reader}).Execute(context.Background(), userID, projectID)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.CanUpdate || got.CanDelete {
		t.Fatalf("write capabilities = (%v, %v), want both false", got.CanUpdate, got.CanDelete)
	}
}

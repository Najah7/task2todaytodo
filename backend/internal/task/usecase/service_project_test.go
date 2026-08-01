package usecase

import (
	"context"
	"errors"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	"testing"
	"time"
)

func TestProjectServiceCreateDefaultsTypeAndProgress(t *testing.T) {
	repo := &fakeProjectRepository{}
	service := NewProjectService(repo)
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(24 * time.Hour)

	got, err := service.Create(context.Background(), fixedTaskIDGen("project-1"), "user-1", "", "", "Build API", "", "", startAt, endAt)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got.ID != "project-1" || got.UserID != "user-1" || got.Type.String() != "other" {
		t.Fatalf("project = %+v, want generated ID, user ID, and default type", got)
	}
	if got.Progress != 0 || got.Priority.String() != "low" || got.StartAt != startAt || got.EndAt != endAt {
		t.Fatalf("project = %+v, want default progress, priority, and requested time range", got)
	}
	if repo.created != got {
		t.Fatalf("created project = %+v, want %+v", repo.created, got)
	}
}

func TestProjectServiceGetAggregateScopesByUser(t *testing.T) {
	repo := &fakeProjectRepository{
		aggregate: domain.ProjectAggregate{
			Project: mustExistingProjectForService(t, "project-1", "user-1", "work", "Project", "", "", 0, "low"),
			Tasks: []domain.Task{
				mustExistingTask(t, "task-1", "user-1", "project-1", "Task", "", nil, nil, 0, "low", "open"),
			},
		},
	}
	service := NewProjectService(repo)

	got, err := service.GetAggregate(context.Background(), "user-1", "project-1")
	if err != nil {
		t.Fatalf("GetAggregate() error = %v", err)
	}

	if got.Project.ID != "project-1" || len(got.Tasks) != 1 {
		t.Fatalf("aggregate = %+v, want project with tasks", got)
	}
	if repo.aggregateUserID != "user-1" || repo.aggregateProjectID != "project-1" {
		t.Fatalf("repo lookup user=%q project=%q, want scoped lookup", repo.aggregateUserID, repo.aggregateProjectID)
	}
}

func TestProjectServiceUpdateBasicMergesAllowedFields(t *testing.T) {
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(24 * time.Hour)
	project := mustExistingProjectWithTimesForService(t, "project-1", "user-1", "work", "Old title", "Old goal", "Old description", 42, "low", startAt, endAt)
	repo := &fakeProjectRepository{
		projectByUser: project,
	}
	service := NewProjectService(repo)
	projectType := "study"
	title := "New title"
	description := "New description"

	got, err := service.UpdateBasic(context.Background(), "user-1", "project-1", ProjectBasicUpdate{
		Type:        &projectType,
		Title:       &title,
		Description: &description,
	})
	if err != nil {
		t.Fatalf("UpdateBasic() error = %v", err)
	}

	if got.Type.String() != "study" || got.Title != "New title" || got.Description != "New description" {
		t.Fatalf("project = %+v, want requested fields updated", got)
	}
	if got.Goal != "Old goal" || got.Progress != 42 || got.Priority.String() != "low" || got.StartAt != startAt || got.EndAt != endAt {
		t.Fatalf("project = %+v, want omitted fields preserved", got)
	}
	if repo.getByUserID != "user-1" || repo.getByProjectID != "project-1" || repo.updateByUserID != "user-1" {
		t.Fatalf("repo calls = %+v, want user-scoped get/update", repo)
	}
}

func TestProjectServiceUpdateScheduleMergesAllowedFields(t *testing.T) {
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(24 * time.Hour)
	project := mustExistingProjectWithTimesForService(t, "project-1", "user-1", "work", "Old title", "Old goal", "Old description", 42, "low", startAt, endAt)
	repo := &fakeProjectRepository{
		projectByUser: project,
	}
	service := NewProjectService(repo)
	newEndAt := endAt.Add(24 * time.Hour)

	got, err := service.UpdateSchedule(context.Background(), "user-1", "project-1", ProjectScheduleUpdate{
		EndAt: &newEndAt,
	})
	if err != nil {
		t.Fatalf("UpdateSchedule() error = %v", err)
	}

	if got.EndAt != newEndAt {
		t.Fatalf("project = %+v, want requested schedule updated", got)
	}
	if got.Type != project.Type || got.Priority != project.Priority || got.Title != project.Title || got.StartAt != startAt {
		t.Fatalf("project = %+v, want non-schedule fields preserved", got)
	}
	if repo.getByUserID != "user-1" || repo.getByProjectID != "project-1" || repo.updateByUserID != "user-1" {
		t.Fatalf("repo calls = %+v, want user-scoped get/update", repo)
	}
}

func TestProjectServiceUpdatePriorityMergesAllowedFields(t *testing.T) {
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(24 * time.Hour)
	project := mustExistingProjectWithTimesForService(t, "project-1", "user-1", "work", "Old title", "Old goal", "Old description", 42, "low", startAt, endAt)
	repo := &fakeProjectRepository{
		projectByUser: project,
	}
	service := NewProjectService(repo)

	got, err := service.UpdatePriority(context.Background(), "user-1", "project-1", "high")
	if err != nil {
		t.Fatalf("UpdatePriority() error = %v", err)
	}

	if got.Priority.String() != "high" {
		t.Fatalf("project = %+v, want requested priority updated", got)
	}
	if got.Type != project.Type || got.Title != project.Title || got.StartAt != startAt || got.EndAt != endAt {
		t.Fatalf("project = %+v, want non-priority fields preserved", got)
	}
	if repo.getByUserID != "user-1" || repo.getByProjectID != "project-1" || repo.updateByUserID != "user-1" {
		t.Fatalf("repo calls = %+v, want user-scoped get/update", repo)
	}
}

func TestProjectServiceUpdateBasicPropagatesProjectNotFound(t *testing.T) {
	repo := &fakeProjectRepository{getByUserErr: domain.ErrProjectNotFound}
	service := NewProjectService(repo)

	_, err := service.UpdateBasic(context.Background(), "user-1", "project-1", ProjectBasicUpdate{})
	if !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("UpdateBasic() error = %v, want domain.ErrProjectNotFound", err)
	}
}

func TestProjectServiceDeleteScopesByUser(t *testing.T) {
	repo := &fakeProjectRepository{}
	service := NewProjectService(repo)

	if err := service.Delete(context.Background(), "user-1", "project-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if repo.deleteUserID != "user-1" || repo.deleteProjectID != "project-1" {
		t.Fatalf("delete user=%q project=%q, want scoped delete", repo.deleteUserID, repo.deleteProjectID)
	}
}

func TestProjectServiceDeletePropagatesProjectNotFound(t *testing.T) {
	repo := &fakeProjectRepository{deleteErr: domain.ErrProjectNotFound}
	service := NewProjectService(repo)

	err := service.Delete(context.Background(), "user-1", "project-1")
	if !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("Delete() error = %v, want domain.ErrProjectNotFound", err)
	}
}

type fakeProjectRepository struct {
	ProjectRepository

	created            domain.Project
	aggregate          domain.ProjectAggregate
	aggregateUserID    domain.UserID
	aggregateProjectID domain.ProjectID
	projectByUser      domain.Project
	getByUserID        domain.UserID
	getByProjectID     domain.ProjectID
	getByUserErr       error
	updated            domain.Project
	updateByUserID     domain.UserID
	deleteUserID       domain.UserID
	deleteProjectID    domain.ProjectID
	deleteErr          error
}

func (r *fakeProjectRepository) Create(_ context.Context, project domain.Project) (domain.Project, error) {
	r.created = project
	return project, nil
}

func (r *fakeProjectRepository) GetAggregateByUser(_ context.Context, userID domain.UserID, id domain.ProjectID) (domain.ProjectAggregate, error) {
	r.aggregateUserID = userID
	r.aggregateProjectID = id
	return r.aggregate, nil
}

func (r *fakeProjectRepository) GetByUser(_ context.Context, userID domain.UserID, id domain.ProjectID) (domain.Project, error) {
	r.getByUserID = userID
	r.getByProjectID = id
	if r.getByUserErr != nil {
		return domain.NewZeroProject(), r.getByUserErr
	}
	return r.projectByUser, nil
}

func (r *fakeProjectRepository) UpdateByUser(_ context.Context, userID domain.UserID, project domain.Project) (domain.Project, error) {
	r.updateByUserID = userID
	r.updated = project
	return project, nil
}

func (r *fakeProjectRepository) DeleteByUser(_ context.Context, userID domain.UserID, id domain.ProjectID) error {
	r.deleteUserID = userID
	r.deleteProjectID = id
	return r.deleteErr
}

func mustExistingProjectForService(t *testing.T, id domain.ProjectID, userID domain.UserID, projectTypeValue string, title string, goal string, description string, progress int, priorityValue string) domain.Project {
	t.Helper()
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	return mustExistingProjectWithTimesForService(t, id, userID, projectTypeValue, title, goal, description, progress, priorityValue, startAt, startAt.Add(24*time.Hour))
}

func mustExistingProjectWithTimesForService(
	t *testing.T,
	id domain.ProjectID,
	userID domain.UserID,
	projectTypeValue string,
	title string,
	goal string,
	description string,
	progress int,
	priorityValue string,
	startAt time.Time,
	endAt time.Time,
) domain.Project {
	t.Helper()
	project, err := domain.NewExistingProject(
		id,
		userID,
		mustProjectType(t, projectTypeValue),
		title,
		goal,
		description,
		progress,
		mustTaskPriority(t, priorityValue),
		startAt,
		endAt,
		startAt.Add(-time.Hour),
		startAt.Add(-time.Minute),
	)
	if err != nil {
		t.Fatalf("domain.NewExistingProject() error = %v", err)
	}
	return project
}

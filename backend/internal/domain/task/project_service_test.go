package task

import (
	"context"
	"errors"
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
		aggregate: ProjectAggregate{
			Project: mustExistingProjectForService(t, "project-1", "user-1", "work", "Project", "", "", 0, "low"),
			Tasks: []Task{
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

func TestProjectServiceUpdateMergesAllowedFields(t *testing.T) {
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(24 * time.Hour)
	repo := &fakeProjectRepository{
		projectByUser: mustExistingProjectWithTimesForService(t, "project-1", "user-1", "work", "Old title", "Old goal", "Old description", 42, "low", startAt, endAt),
	}
	service := NewProjectService(repo)
	projectType := "study"
	priority := "high"
	title := "New title"
	description := "New description"
	newEndAt := endAt.Add(24 * time.Hour)

	got, err := service.Update(context.Background(), "user-1", "project-1", ProjectUpdate{
		Type:        &projectType,
		Priority:    &priority,
		Title:       &title,
		Description: &description,
		EndAt:       &newEndAt,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if got.Type.String() != "study" || got.Priority.String() != "high" || got.Title != "New title" || got.Description != "New description" || got.EndAt != newEndAt {
		t.Fatalf("project = %+v, want requested fields updated", got)
	}
	if got.Goal != "Old goal" || got.Progress != 42 || got.StartAt != startAt {
		t.Fatalf("project = %+v, want omitted fields preserved", got)
	}
	if repo.getByUserID != "user-1" || repo.getByProjectID != "project-1" || repo.updateByUserID != "user-1" {
		t.Fatalf("repo calls = %+v, want user-scoped get/update", repo)
	}
}

func TestProjectServiceUpdatePropagatesProjectNotFound(t *testing.T) {
	repo := &fakeProjectRepository{getByUserErr: ErrProjectNotFound}
	service := NewProjectService(repo)

	_, err := service.Update(context.Background(), "user-1", "project-1", ProjectUpdate{})
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("Update() error = %v, want ErrProjectNotFound", err)
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
	repo := &fakeProjectRepository{deleteErr: ErrProjectNotFound}
	service := NewProjectService(repo)

	err := service.Delete(context.Background(), "user-1", "project-1")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("Delete() error = %v, want ErrProjectNotFound", err)
	}
}

type fakeProjectRepository struct {
	ProjectRepository

	created            Project
	aggregate          ProjectAggregate
	aggregateUserID    UserID
	aggregateProjectID ProjectID
	projectByUser      Project
	getByUserID        UserID
	getByProjectID     ProjectID
	getByUserErr       error
	updated            Project
	updateByUserID     UserID
	deleteUserID       UserID
	deleteProjectID    ProjectID
	deleteErr          error
}

func (r *fakeProjectRepository) Create(_ context.Context, project Project) (Project, error) {
	r.created = project
	return project, nil
}

func (r *fakeProjectRepository) GetAggregateByUser(_ context.Context, userID UserID, id ProjectID) (ProjectAggregate, error) {
	r.aggregateUserID = userID
	r.aggregateProjectID = id
	return r.aggregate, nil
}

func (r *fakeProjectRepository) GetByUser(_ context.Context, userID UserID, id ProjectID) (Project, error) {
	r.getByUserID = userID
	r.getByProjectID = id
	if r.getByUserErr != nil {
		return NewZeroProject(), r.getByUserErr
	}
	return r.projectByUser, nil
}

func (r *fakeProjectRepository) UpdateByUser(_ context.Context, userID UserID, project Project) (Project, error) {
	r.updateByUserID = userID
	r.updated = project
	return project, nil
}

func (r *fakeProjectRepository) DeleteByUser(_ context.Context, userID UserID, id ProjectID) error {
	r.deleteUserID = userID
	r.deleteProjectID = id
	return r.deleteErr
}

func mustExistingProjectForService(t *testing.T, id ProjectID, userID UserID, projectTypeValue string, title string, goal string, description string, progress int, priorityValue string) Project {
	t.Helper()
	startAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	return mustExistingProjectWithTimesForService(t, id, userID, projectTypeValue, title, goal, description, progress, priorityValue, startAt, startAt.Add(24*time.Hour))
}

func mustExistingProjectWithTimesForService(
	t *testing.T,
	id ProjectID,
	userID UserID,
	projectTypeValue string,
	title string,
	goal string,
	description string,
	progress int,
	priorityValue string,
	startAt time.Time,
	endAt time.Time,
) Project {
	t.Helper()
	project, err := NewExistingProject(
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
		t.Fatalf("NewExistingProject() error = %v", err)
	}
	return project
}

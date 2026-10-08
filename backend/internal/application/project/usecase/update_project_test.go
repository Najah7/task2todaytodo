package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type updateProjectRepositoryFake struct {
	project          dao.Project
	getErr           error
	updateErr        error
	getCalls         int
	updateCalls      int
	getUserID        domain.UserID
	getProjectID     domain.ProjectID
	updateUserID     domain.UserID
	expectedRevision int32
	updated          domain.Project
	callOrder        []string
}

func (repo *updateProjectRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, projectID domain.ProjectID) (dao.Project, error) {
	repo.getCalls++
	repo.getUserID = userID
	repo.getProjectID = projectID
	repo.callOrder = append(repo.callOrder, "get")
	if repo.getErr != nil {
		return dao.Project{}, repo.getErr
	}
	if repo.project.ID == "" || repo.project.ID != string(projectID) || repo.project.UserID != string(userID) {
		return dao.Project{}, domain.ErrProjectNotFound
	}
	return repo.project, nil
}

func (repo *updateProjectRepositoryFake) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, _ shared.Capability) (dao.Project, error) {
	return repo.GetByUserID(ctx, userID, projectID)
}

func (*updateProjectRepositoryFake) HasPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (bool, error) {
	return true, nil
}

func (repo *updateProjectRepositoryFake) UpdateByUserID(_ context.Context, userID domain.UserID, project domain.Project, expectedRevision int32) (dao.Project, error) {
	repo.updateCalls++
	repo.updateUserID = userID
	repo.expectedRevision = expectedRevision
	repo.updated = project
	repo.callOrder = append(repo.callOrder, "update")
	if repo.updateErr != nil {
		return dao.Project{}, repo.updateErr
	}
	result := projectToDAO(project)
	result.Revision = expectedRevision + 1
	return result, nil
}

func TestUpdateProjectUseCaseExecuteUpdatesSpecifiedFieldsAndRecomputesProgress(t *testing.T) {
	userID := domain.UserID("user-1")
	projectID := domain.ProjectID("project-1")
	repo := &updateProjectRepositoryFake{project: projectFixture()}
	newTitle := "Updated project"
	got, err := NewUpdateProjectUseCase(repo, updateProjectProgressSource(), nil).Execute(
		context.Background(), userID, projectID, 1,
		PatchField[string]{Present: true, Value: &newTitle},
		PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{},
		PatchField[time.Time]{}, PatchField[time.Time]{},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got.Title != newTitle || got.Goal != "Original goal" || got.Description != "Original description" || got.Progress != 50 {
		t.Errorf("updated project fields = %#v, want title changed and progress recomputed to 50", got)
	}
	if repo.updated.UserID != userID || repo.updated.Progress != 37 {
		t.Errorf("updated domain project user/progress = %q/%d, want %q/37", repo.updated.UserID, repo.updated.Progress, userID)
	}
	if !reflect.DeepEqual(repo.callOrder, []string{"get", "update"}) {
		t.Errorf("repository call order = %v, want [get update]", repo.callOrder)
	}
	if repo.getUserID != userID || repo.getProjectID != projectID || repo.updateUserID != userID {
		t.Errorf("repository user/project arguments = %q/%q/%q", repo.getUserID, repo.getProjectID, repo.updateUserID)
	}
}

func TestUpdateProjectUseCaseExecuteAppliesEachPatchField(t *testing.T) {
	start := time.Date(2026, 2, 1, 15, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	end := time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		execute func(*updateProjectRepositoryFake) error
		assert  func(*testing.T, domain.Project)
	}{
		{
			name: "goal",
			execute: func(repo *updateProjectRepositoryFake) error {
				value := "New goal"
				_, err := updateProject(repo, PatchField[string]{}, PatchField[string]{Present: true, Value: &value}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[time.Time]{})
				return err
			},
			assert: func(t *testing.T, p domain.Project) {
				if p.Goal != "New goal" {
					t.Errorf("Goal = %q", p.Goal)
				}
			},
		},
		{
			name: "description",
			execute: func(repo *updateProjectRepositoryFake) error {
				value := "New description"
				_, err := updateProject(repo, PatchField[string]{}, PatchField[string]{}, PatchField[string]{Present: true, Value: &value}, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[time.Time]{})
				return err
			},
			assert: func(t *testing.T, p domain.Project) {
				if p.Description != "New description" {
					t.Errorf("Description = %q", p.Description)
				}
			},
		},
		{
			name: "type",
			execute: func(repo *updateProjectRepositoryFake) error {
				value := "study"
				_, err := updateProject(repo, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{Present: true, Value: &value}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[time.Time]{})
				return err
			},
			assert: func(t *testing.T, p domain.Project) {
				if p.Type.Value != "study" {
					t.Errorf("Type = %q", p.Type.Value)
				}
			},
		},
		{
			name: "priority",
			execute: func(repo *updateProjectRepositoryFake) error {
				value := "urgent"
				_, err := updateProject(repo, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{Present: true, Value: &value}, PatchField[time.Time]{}, PatchField[time.Time]{})
				return err
			},
			assert: func(t *testing.T, p domain.Project) {
				if p.Priority.Value != "urgent" {
					t.Errorf("Priority = %q", p.Priority.Value)
				}
			},
		},
		{
			name: "start date",
			execute: func(repo *updateProjectRepositoryFake) error {
				_, err := updateProject(repo, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{Present: true, Value: &start}, PatchField[time.Time]{})
				return err
			},
			assert: func(t *testing.T, p domain.Project) {
				if p.Schedule.StartDate == nil || p.Schedule.StartDate.Format("2006-01-02") != "2026-02-01" {
					t.Errorf("StartDate = %v", p.Schedule.StartDate)
				}
			},
		},
		{
			name: "end date",
			execute: func(repo *updateProjectRepositoryFake) error {
				_, err := updateProject(repo, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[time.Time]{Present: true, Value: &end})
				return err
			},
			assert: func(t *testing.T, p domain.Project) {
				if p.Schedule.EndDate == nil || p.Schedule.EndDate.Format("2006-01-02") != "2026-02-04" {
					t.Errorf("EndDate = %v", p.Schedule.EndDate)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &updateProjectRepositoryFake{project: projectFixture()}
			if err := test.execute(repo); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			test.assert(t, repo.updated)
		})
	}
}

func TestUpdateProjectUseCaseExecuteClearsNullableFieldsAndPreservesOmittedFields(t *testing.T) {
	repo := &updateProjectRepositoryFake{project: projectFixture()}
	_, err := updateProject(repo,
		PatchField[string]{},
		PatchField[string]{Present: true},
		PatchField[string]{Present: true},
		PatchField[string]{}, PatchField[string]{},
		PatchField[time.Time]{Present: true}, PatchField[time.Time]{},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.updated.Goal != "" || repo.updated.Description != "" || repo.updated.Schedule.StartDate != nil {
		t.Errorf("cleared nullable fields = goal %q, description %q, start %v", repo.updated.Goal, repo.updated.Description, repo.updated.Schedule.StartDate)
	}
	if repo.updated.Title != "Original project" || repo.updated.Type.Value != "work" || repo.updated.Priority.Value != "medium" || repo.updated.Schedule.EndDate == nil {
		t.Errorf("omitted fields not preserved: %#v", repo.updated)
	}
}

func TestUpdateProjectUseCaseExecuteAllowsSameDayRange(t *testing.T) {
	sameDay := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	repo := &updateProjectRepositoryFake{project: projectFixture()}
	_, err := updateProject(repo,
		PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{},
		PatchField[time.Time]{Present: true, Value: &sameDay}, PatchField[time.Time]{Present: true, Value: &sameDay},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, same-day range should be valid", err)
	}
}

func TestUpdateProjectUseCaseExecuteRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*updateProjectRepositoryFake) error
		want error
	}{
		{
			name: "null title",
			run: func(repo *updateProjectRepositoryFake) error {
				_, err := updateProject(repo, PatchField[string]{Present: true}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[time.Time]{})
				return err
			},
			want: ErrProjectPatchRequiredFieldNull,
		},
		{
			name: "null type",
			run: func(repo *updateProjectRepositoryFake) error {
				_, err := updateProject(repo, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{Present: true}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[time.Time]{})
				return err
			},
			want: ErrProjectPatchRequiredFieldNull,
		},
		{
			name: "null priority",
			run: func(repo *updateProjectRepositoryFake) error {
				_, err := updateProject(repo, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{Present: true}, PatchField[time.Time]{}, PatchField[time.Time]{})
				return err
			},
			want: ErrProjectPatchRequiredFieldNull,
		},
		{
			name: "end before start",
			run: func(repo *updateProjectRepositoryFake) error {
				start := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
				end := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
				_, err := updateProject(repo, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{Present: true, Value: &start}, PatchField[time.Time]{Present: true, Value: &end})
				return err
			},
			want: domain.ErrProjectEndDateBeforeStartDate,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &updateProjectRepositoryFake{project: projectFixture()}
			if err := test.run(repo); !errors.Is(err, test.want) {
				t.Errorf("Execute() error = %v, want %v", err, test.want)
			}
			if repo.updateCalls != 0 {
				t.Errorf("UpdateByUserID() calls = %d, want 0", repo.updateCalls)
			}
		})
	}
}

func TestUpdateProjectUseCaseExecuteReturnsNotFoundAndRepositoryErrors(t *testing.T) {
	getErr := errors.New("get failed")
	updateErr := errors.New("update failed")
	for _, test := range []struct {
		name string
		repo *updateProjectRepositoryFake
		want error
	}{
		{name: "missing project", repo: &updateProjectRepositoryFake{}, want: domain.ErrProjectNotFound},
		{name: "project belongs to another user", repo: &updateProjectRepositoryFake{project: projectFixtureForUser("other-user")}, want: domain.ErrProjectNotFound},
		{name: "get failure", repo: &updateProjectRepositoryFake{getErr: getErr}, want: getErr},
		{name: "update failure", repo: &updateProjectRepositoryFake{project: projectFixture(), updateErr: updateErr}, want: updateErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := updateProject(test.repo, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[time.Time]{})
			if !errors.Is(err, test.want) {
				t.Errorf("Execute() error = %v, want %v", err, test.want)
			}
			if test.name == "missing project" || test.name == "project belongs to another user" || test.name == "get failure" {
				if test.repo.updateCalls != 0 {
					t.Errorf("UpdateByUserID() calls = %d, want 0", test.repo.updateCalls)
				}
			}
		})
	}
}

func updateProject(
	repo *updateProjectRepositoryFake,
	title, goal, description, projectType, priority PatchField[string],
	startDate, endDate PatchField[time.Time],
) (dao.Project, error) {
	return NewUpdateProjectUseCase(repo, updateProjectProgressSource(), nil).Execute(
		context.Background(), domain.UserID("user-1"), domain.ProjectID("project-1"), 1,
		title, goal, description, projectType, priority, startDate, endDate,
	)
}

func updateProjectProgressSource() *projectProgressSourceFake {
	return &projectProgressSourceFake{sources: dao.ProjectProgressSources{
		Tasks: []dao.ProjectTaskProgress{{ProjectID: "project-1", TaskID: "task-1", Total: 2, Completed: 1}},
	}}
}

func projectFixture() dao.Project {
	return projectFixtureForUser("user-1")
}

func projectFixtureForUser(userID string) dao.Project {
	start, end := "2026-01-10", "2026-12-20"
	return dao.Project{
		ID: "project-1", UserID: userID,
		Type: dao.ProjectType{Value: "work"}, Status: "open", Title: "Original project",
		Goal: "Original goal", Description: "Original description", Progress: 37,
		Priority: dao.Priority{Value: "medium"}, StartDate: &start, EndDate: &end,
		CreatedAt: 100, UpdatedAt: 200, Revision: 1,
	}
}

func projectToDAO(project domain.Project) dao.Project {
	var startDate, endDate *string
	if project.Schedule.StartDate != nil {
		value := project.Schedule.StartDate.Format("2006-01-02")
		startDate = &value
	}
	if project.Schedule.EndDate != nil {
		value := project.Schedule.EndDate.Format("2006-01-02")
		endDate = &value
	}
	return dao.Project{
		ID: string(project.ID), UserID: string(project.UserID),
		Type: dao.ProjectType{Value: project.Type.Value}, Status: project.Status.String(), Title: project.Title,
		Goal: project.Goal, Description: project.Description, Progress: project.Progress,
		Priority: dao.Priority{Value: project.Priority.Value}, StartDate: startDate, EndDate: endDate,
		CreatedAt: project.CreatedAt.Unix(), UpdatedAt: project.UpdatedAt.Unix(),
	}
}

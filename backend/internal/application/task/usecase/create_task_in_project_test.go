package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type createTaskInProjectProjectRepositoryFake struct {
	project TaskProject
	err     error
	calls   int
	userID  domain.UserID
	id      domain.ProjectID
}

func (repo *createTaskInProjectProjectRepositoryFake) GetProjectByUserID(_ context.Context, userID, id string) (TaskProject, error) {
	repo.calls++
	repo.userID, repo.id = domain.UserID(userID), domain.ProjectID(id)
	return repo.project, repo.err
}

func (repo *createTaskInProjectProjectRepositoryFake) LockProjectByUserIDWithPermission(ctx context.Context, userID, id string, _ shared.Capability) (TaskProject, error) {
	project, err := repo.GetProjectByUserID(ctx, userID, id)
	if err != nil {
		return TaskProject{}, err
	}
	if project.ID != id || project.OwnerID != userID {
		return TaskProject{}, ErrTaskProjectNotFound
	}
	return project, nil
}

func (repo *createTaskInProjectProjectRepositoryFake) GetProjectByUserIDWithPermission(ctx context.Context, userID, id string, _ shared.Capability) (TaskProject, error) {
	return repo.GetProjectByUserID(ctx, userID, id)
}

type createTaskInProjectTaskRepositoryFake struct {
	taskProgressTestRepository
	task   domain.Task
	result dao.Task
	err    error
	calls  int
}

func (repo *createTaskInProjectTaskRepositoryFake) CreateInProject(_ context.Context, _ domain.UserID, task domain.Task) (dao.Task, error) {
	repo.calls++
	repo.task = task
	return repo.result, repo.err
}

type createTaskInProjectRepositoriesFake struct {
	taskProgressTestRepositories
	projects *createTaskInProjectProjectRepositoryFake
	tasks    *createTaskInProjectTaskRepositoryFake
}

func (repos *createTaskInProjectRepositoriesFake) TaskProjects() TaskProjectRepository {
	return repos.projects
}
func (repos *createTaskInProjectRepositoriesFake) Tasks() TaskRepository { return repos.tasks }

type createTaskInProjectUOWFake struct {
	repos *createTaskInProjectRepositoriesFake
	err   error
	calls int
}

func (uow *createTaskInProjectUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

func newCreateTaskInProjectFakes() (*createTaskInProjectUOWFake, *createTaskInProjectProjectRepositoryFake, *createTaskInProjectTaskRepositoryFake) {
	projectRepo := &createTaskInProjectProjectRepositoryFake{project: TaskProject{
		ID:              "project-1",
		OwnerID:         "user-1",
		DefaultPriority: "high",
	}}
	taskRepo := &createTaskInProjectTaskRepositoryFake{}
	repos := &createTaskInProjectRepositoriesFake{projects: projectRepo, tasks: taskRepo}
	return &createTaskInProjectUOWFake{repos: repos}, projectRepo, taskRepo
}

func validCreateTaskInProjectInput() CreateTaskInProjectInput {
	return CreateTaskInProjectInput{
		ID:        domain.TaskID("task-1"),
		UserID:    domain.UserID("user-1"),
		ProjectID: domain.ProjectID("project-1"),
		Title:     "Task title",
	}
}

func TestCreateTaskInProjectUseCaseExecuteCreatesOwnedProjectTaskAndInheritsPriority(t *testing.T) {
	want := dao.Task{ID: "task-1", UserID: "user-1", ProjectID: "project-1", Title: "Task title"}
	uow, projectRepo, taskRepo := newCreateTaskInProjectFakes()
	taskRepo.result = want

	got, err := NewCreateTaskInProjectUseCase(uow, nil).Execute(context.Background(), validCreateTaskInProjectInput())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got != want {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if uow.calls != 1 || projectRepo.calls != 1 || taskRepo.calls != 1 {
		t.Fatalf("calls = UOW %d, project %d, task %d; want 1 each", uow.calls, projectRepo.calls, taskRepo.calls)
	}
	if projectRepo.userID != "user-1" || projectRepo.id != "project-1" {
		t.Errorf("GetByUserID() arguments = (%q, %q), want (%q, %q)", projectRepo.userID, projectRepo.id, "user-1", "project-1")
	}
	if taskRepo.task.ID != "task-1" || taskRepo.task.UserID != "user-1" || taskRepo.task.ProjectID != "project-1" {
		t.Errorf("created task identity = (%q, %q, %q), want (task-1, user-1, project-1)", taskRepo.task.ID, taskRepo.task.UserID, taskRepo.task.ProjectID)
	}
	if taskRepo.task.Priority.String() != "high" || taskRepo.task.Status.String() != "open" || taskRepo.task.Progress != 0 {
		t.Errorf("created defaults = priority %q, status %q, progress %d; want high, open, 0", taskRepo.task.Priority.String(), taskRepo.task.Status.String(), taskRepo.task.Progress)
	}
}

func TestCreateTaskInProjectUseCaseExecuteUsesExplicitPriorityAndOptionalFields(t *testing.T) {
	uow, _, taskRepo := newCreateTaskInProjectFakes()
	input := validCreateTaskInProjectInput()
	input.Priority = "urgent"
	input.Description = "Details"
	input.DueDate = time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	estimated := 45
	input.EstimatedMinutes = &estimated

	_, err := NewCreateTaskInProjectUseCase(uow, nil).Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if taskRepo.task.Priority.String() != "urgent" || taskRepo.task.Description != "Details" || !taskRepo.task.DueDate.Equal(input.DueDate) || taskRepo.task.EstimatedMinutes == nil || *taskRepo.task.EstimatedMinutes != estimated {
		t.Errorf("created task optional values = priority %q, description %q, due %v, estimate %v; want urgent, Details, %v, %d", taskRepo.task.Priority.String(), taskRepo.task.Description, taskRepo.task.DueDate, taskRepo.task.EstimatedMinutes, input.DueDate, estimated)
	}
}

func TestCreateTaskInProjectUseCaseExecuteReturnsNotFoundForMissingOrForeignProject(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*createTaskInProjectProjectRepositoryFake)
	}{
		{name: "missing", setup: func(repo *createTaskInProjectProjectRepositoryFake) { repo.err = ErrTaskProjectNotFound }},
		{name: "foreign owner", setup: func(repo *createTaskInProjectProjectRepositoryFake) { repo.project.OwnerID = "other-user" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uow, projectRepo, taskRepo := newCreateTaskInProjectFakes()
			test.setup(projectRepo)

			_, err := NewCreateTaskInProjectUseCase(uow, nil).Execute(context.Background(), validCreateTaskInProjectInput())
			if !errors.Is(err, ErrTaskProjectNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskProjectNotFound)
			}
			if taskRepo.calls != 0 {
				t.Errorf("CreateInProject() calls = %d, want 0", taskRepo.calls)
			}
		})
	}
}

func TestCreateTaskInProjectUseCaseExecuteRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CreateTaskInProjectInput)
		wantErr error
	}{
		{name: "empty title", mutate: func(input *CreateTaskInProjectInput) { input.Title = " \t " }, wantErr: domain.ErrTaskTitleEmpty},
		{name: "invalid priority", mutate: func(input *CreateTaskInProjectInput) { input.Priority = "invalid" }, wantErr: domain.ErrTaskPriorityInvalid},
		{name: "negative estimate", mutate: func(input *CreateTaskInProjectInput) { negative := -1; input.EstimatedMinutes = &negative }, wantErr: domain.ErrTaskEstimatedMinutesInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uow, _, taskRepo := newCreateTaskInProjectFakes()
			input := validCreateTaskInProjectInput()
			test.mutate(&input)

			_, err := NewCreateTaskInProjectUseCase(uow, nil).Execute(context.Background(), input)
			if !errors.Is(err, test.wantErr) {
				t.Errorf("Execute() error = %v, want %v", err, test.wantErr)
			}
			if taskRepo.calls != 0 {
				t.Errorf("CreateInProject() calls = %d, want 0", taskRepo.calls)
			}
		})
	}
}

func TestCreateTaskInProjectUseCaseExecutePropagatesFailures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*createTaskInProjectUOWFake, *createTaskInProjectProjectRepositoryFake, *createTaskInProjectTaskRepositoryFake, error)
	}{
		{name: "unit of work", setup: func(uow *createTaskInProjectUOWFake, _ *createTaskInProjectProjectRepositoryFake, _ *createTaskInProjectTaskRepositoryFake, want error) {
			uow.err = want
		}},
		{name: "project repository", setup: func(_ *createTaskInProjectUOWFake, projectRepo *createTaskInProjectProjectRepositoryFake, _ *createTaskInProjectTaskRepositoryFake, want error) {
			projectRepo.err = want
		}},
		{name: "task repository", setup: func(_ *createTaskInProjectUOWFake, _ *createTaskInProjectProjectRepositoryFake, taskRepo *createTaskInProjectTaskRepositoryFake, want error) {
			taskRepo.err = want
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wantErr := errors.New("database unavailable")
			uow, projectRepo, taskRepo := newCreateTaskInProjectFakes()
			test.setup(uow, projectRepo, taskRepo, wantErr)

			_, err := NewCreateTaskInProjectUseCase(uow, nil).Execute(context.Background(), validCreateTaskInProjectInput())
			if !errors.Is(err, wantErr) {
				t.Errorf("Execute() error = %v, want %v", err, wantErr)
			}
		})
	}
}

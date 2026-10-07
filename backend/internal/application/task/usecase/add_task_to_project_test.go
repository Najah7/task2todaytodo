package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type addTaskToProjectUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *addTaskToProjectUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type addTaskToProjectRepositoriesFake struct {
	taskProgressTestRepositories
	projects TaskProjectRepository
	tasks    TaskRepository
	accesses *[]string
}

func (repos addTaskToProjectRepositoriesFake) TaskProjects() TaskProjectRepository {
	*repos.accesses = append(*repos.accesses, "projects")
	return repos.projects
}

func (repos addTaskToProjectRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

type addTaskToProjectProjectRepositoryFake struct {
	project   TaskProject
	projects  map[string]TaskProject
	err       error
	userID    domain.UserID
	projectID domain.ProjectID
	calls     int
	accesses  *[]string
}

func (repo *addTaskToProjectProjectRepositoryFake) LockProjectByUserIDWithPermission(_ context.Context, userID, projectID string, _ shared.Capability) (TaskProject, error) {
	repo.calls++
	repo.userID, repo.projectID = domain.UserID(userID), domain.ProjectID(projectID)
	*repo.accesses = append(*repo.accesses, "lock-project")
	if repo.err != nil {
		return TaskProject{}, repo.err
	}
	project := repo.project
	if candidate, ok := repo.projects[projectID]; ok {
		project = candidate
	}
	if project.ID != projectID || project.OwnerID != userID {
		return TaskProject{}, ErrTaskProjectNotFound
	}
	return project, nil
}

func (repo *addTaskToProjectProjectRepositoryFake) GetProjectByUserID(_ context.Context, userID, projectID string) (TaskProject, error) {
	repo.calls++
	repo.userID, repo.projectID = domain.UserID(userID), domain.ProjectID(projectID)
	*repo.accesses = append(*repo.accesses, "get-project")
	if repo.err != nil {
		return TaskProject{}, repo.err
	}
	return repo.project, nil
}

func (repo *addTaskToProjectProjectRepositoryFake) GetProjectByUserIDWithPermission(ctx context.Context, userID, projectID string, _ shared.Capability) (TaskProject, error) {
	return repo.GetProjectByUserID(ctx, userID, projectID)
}

type addTaskToProjectTaskRepositoryFake struct {
	taskProgressTestRepository
	task            dao.Task
	assignedTask    dao.Task
	getErr          error
	assignErr       error
	userID          domain.UserID
	taskID          domain.TaskID
	assignUserID    domain.UserID
	assignTaskID    domain.TaskID
	assignProjectID domain.ProjectID
	assignExpected  int32
	getCalls        int
	assignCalls     int
	accesses        *[]string
}

func (repo *addTaskToProjectTaskRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.getCalls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "get-task")
	if repo.getErr != nil {
		return dao.Task{}, repo.getErr
	}
	return repo.task, nil
}

func (repo *addTaskToProjectTaskRepositoryFake) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}

func (repo *addTaskToProjectTaskRepositoryFake) AssignToProjectByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error) {
	repo.assignCalls++
	repo.assignUserID, repo.assignTaskID, repo.assignProjectID = userID, taskID, projectID
	repo.assignExpected = expectedRevision
	*repo.accesses = append(*repo.accesses, "assign-task")
	if repo.assignErr != nil {
		return dao.Task{}, repo.assignErr
	}
	return repo.assignedTask, nil
}

func newAddTaskToProjectFixture(project legacyProjectFixture, task dao.Task) (*addTaskToProjectUOWFake, *addTaskToProjectProjectRepositoryFake, *addTaskToProjectTaskRepositoryFake, *[]string) {
	var accesses []string
	projectRepo := &addTaskToProjectProjectRepositoryFake{project: TaskProject{ID: project.ID, OwnerID: project.UserID, DefaultPriority: project.Priority.Value}, accesses: &accesses}
	taskRepo := &addTaskToProjectTaskRepositoryFake{task: task, accesses: &accesses}
	uow := &addTaskToProjectUOWFake{repos: addTaskToProjectRepositoriesFake{
		projects: projectRepo,
		tasks:    taskRepo,
		accesses: &accesses,
	}}
	return uow, projectRepo, taskRepo, &accesses
}

func TestAddTaskToProjectUseCaseExecuteAddsStandaloneTask(t *testing.T) {
	userID, projectID, taskID := domain.UserID("user-1"), domain.ProjectID("project-1"), domain.TaskID("task-1")
	want := dao.Task{ID: string(taskID), UserID: string(userID), ProjectID: string(projectID), Title: "Task", Status: dao.TaskStatus{Value: "open"}, Revision: 2}
	uow, projectRepo, taskRepo, accesses := newAddTaskToProjectFixture(
		legacyProjectFixture{ID: string(projectID), UserID: string(userID)},
		dao.Task{ID: string(taskID), UserID: string(userID), Revision: 1},
	)
	taskRepo.assignedTask = want

	got, err := NewAddTaskToProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), userID, projectID, taskID, 1)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got != want {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if uow.calls != 1 || projectRepo.calls != 1 || taskRepo.getCalls != 2 || taskRepo.assignCalls != 1 {
		t.Errorf("calls = UOW:%d project:%d task get:%d assign:%d, want one project lock, initial+post-lock task reads, and one assignment", uow.calls, projectRepo.calls, taskRepo.getCalls, taskRepo.assignCalls)
	}
	if projectRepo.userID != userID || projectRepo.projectID != projectID || taskRepo.userID != userID || taskRepo.taskID != taskID || taskRepo.assignUserID != userID || taskRepo.assignTaskID != taskID || taskRepo.assignProjectID != projectID {
		t.Errorf("repository arguments do not match requested owner, project, and task")
	}
	if taskRepo.assignExpected != 1 {
		t.Errorf("assignment expected revision = %d, want 1", taskRepo.assignExpected)
	}
	if !reflect.DeepEqual(*accesses, []string{"tasks", "get-task", "projects", "lock-project", "get-task", "assign-task"}) {
		t.Errorf("repository access order = %v, want task read, project lock, task re-read, then assignment", *accesses)
	}
}

func TestAddTaskToProjectUseCaseExecuteMovesTaskFromAnotherProject(t *testing.T) {
	userID, projectID, taskID := domain.UserID("user-1"), domain.ProjectID("project-new"), domain.TaskID("task-1")
	want := dao.Task{ID: string(taskID), UserID: string(userID), ProjectID: string(projectID), Title: "Task", Status: dao.TaskStatus{Value: "open"}, Revision: 2}
	uow, projectRepo, taskRepo, _ := newAddTaskToProjectFixture(
		legacyProjectFixture{ID: string(projectID), UserID: string(userID)},
		dao.Task{ID: string(taskID), UserID: string(userID), ProjectID: "project-old", Revision: 1},
	)
	projectRepo.projects = map[string]TaskProject{
		"project-new": {ID: "project-new", OwnerID: string(userID)},
		"project-old": {ID: "project-old", OwnerID: string(userID)},
	}
	taskRepo.assignedTask = want

	got, err := NewAddTaskToProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), userID, projectID, taskID, 1)
	if err != nil || got != want {
		t.Fatalf("Execute() = %#v, %v; want %#v, nil", got, err, want)
	}
	if taskRepo.assignCalls != 1 {
		t.Errorf("AssignToProjectByUserID() calls = %d, want 1", taskRepo.assignCalls)
	}
}

func TestAddTaskToProjectUseCaseExecuteDoesNotSaveWhenAlreadyInProject(t *testing.T) {
	userID, projectID, taskID := domain.UserID("user-1"), domain.ProjectID("project-1"), domain.TaskID("task-1")
	want := dao.Task{ID: string(taskID), UserID: string(userID), ProjectID: string(projectID), Title: "Task", Status: dao.TaskStatus{Value: "open"}, Revision: 1}
	uow, _, taskRepo, accesses := newAddTaskToProjectFixture(legacyProjectFixture{ID: string(projectID), UserID: string(userID)}, want)

	got, err := NewAddTaskToProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), userID, projectID, taskID, 1)
	if err != nil || got != want {
		t.Fatalf("Execute() = %#v, %v; want %#v, nil", got, err, want)
	}
	if taskRepo.assignCalls != 0 {
		t.Errorf("AssignToProjectByUserID() calls = %d, want 0", taskRepo.assignCalls)
	}
	if !reflect.DeepEqual(*accesses, []string{"tasks", "get-task", "projects", "lock-project", "get-task"}) {
		t.Errorf("repository access order = %v, want task check, project lock, and fresh task check", *accesses)
	}
}

func TestAddTaskToProjectUseCaseExecuteReturnsNotFoundForUnownedOrMissingProject(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		project legacyProjectFixture
		err     error
	}{
		{name: "missing project", err: ErrTaskProjectNotFound},
		{name: "project owned by another user", project: legacyProjectFixture{ID: "project-1", UserID: "other-user"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			uow, projectRepo, taskRepo, _ := newAddTaskToProjectFixture(testCase.project, dao.Task{ID: "task-1", UserID: "user-1", Revision: 1})
			projectRepo.err = testCase.err

			got, err := NewAddTaskToProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1)
			if !errors.Is(err, ErrTaskProjectNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskProjectNotFound)
			}
			if got != (dao.Task{}) || taskRepo.getCalls != 1 {
				t.Errorf("result = %#v and task reads = %d, want zero result after the initial task read", got, taskRepo.getCalls)
			}
		})
	}
}

func TestAddTaskToProjectUseCaseExecuteReturnsNotFoundForUnownedOrMissingTask(t *testing.T) {
	for _, testCase := range []struct {
		name string
		task dao.Task
		err  error
	}{
		{name: "missing task", err: ErrTaskNotFound},
		{name: "task owned by another user", task: dao.Task{ID: "task-1", UserID: "other-user", Revision: 1}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			uow, _, taskRepo, _ := newAddTaskToProjectFixture(legacyProjectFixture{ID: "project-1", UserID: "user-1"}, testCase.task)
			taskRepo.getErr = testCase.err

			got, err := NewAddTaskToProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1)
			if !errors.Is(err, ErrTaskNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
			}
			if got != (dao.Task{}) || taskRepo.assignCalls != 0 {
				t.Errorf("result = %#v and assignments = %d, want zero result and no assignment", got, taskRepo.assignCalls)
			}
		})
	}
}

func TestAddTaskToProjectUseCaseExecutePropagatesUOWError(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &addTaskToProjectUOWFake{err: wantErr}

	got, err := NewAddTaskToProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1)
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if got != (dao.Task{}) || uow.calls != 1 {
		t.Errorf("result = %#v and UOW calls = %d, want zero result and 1 call", got, uow.calls)
	}
}

func TestAddTaskToProjectUseCaseExecutePropagatesRepositoryErrors(t *testing.T) {
	projectErr := errors.New("project lookup failed")
	uow, projectRepo, taskRepo, _ := newAddTaskToProjectFixture(legacyProjectFixture{ID: "project-1", UserID: "user-1"}, dao.Task{ID: "task-1", UserID: "user-1", Revision: 1})
	projectRepo.err = projectErr
	if _, err := NewAddTaskToProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1); !errors.Is(err, projectErr) {
		t.Errorf("project lookup error = %v, want %v", err, projectErr)
	}

	taskErr := errors.New("task lookup failed")
	uow, _, taskRepo, _ = newAddTaskToProjectFixture(legacyProjectFixture{ID: "project-1", UserID: "user-1"}, dao.Task{ID: "task-1", UserID: "user-1", Revision: 1})
	taskRepo.getErr = taskErr
	if _, err := NewAddTaskToProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1); !errors.Is(err, taskErr) {
		t.Errorf("task lookup error = %v, want %v", err, taskErr)
	}

	assignErr := errors.New("assignment failed")
	uow, _, taskRepo, _ = newAddTaskToProjectFixture(legacyProjectFixture{ID: "project-1", UserID: "user-1"}, dao.Task{ID: "task-1", UserID: "user-1", Revision: 1})
	taskRepo.assignErr = assignErr
	if _, err := NewAddTaskToProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1); !errors.Is(err, assignErr) {
		t.Errorf("assignment error = %v, want %v", err, assignErr)
	}
}

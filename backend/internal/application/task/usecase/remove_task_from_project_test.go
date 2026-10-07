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

type removeTaskFromProjectUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *removeTaskFromProjectUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type removeTaskFromProjectRepositoriesFake struct {
	taskProgressTestRepositories
	projects TaskProjectRepository
	tasks    TaskRepository
	accesses *[]string
}

func (repos removeTaskFromProjectRepositoriesFake) TaskProjects() TaskProjectRepository {
	*repos.accesses = append(*repos.accesses, "projects")
	return repos.projects
}

func (repos removeTaskFromProjectRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

type removeTaskFromProjectProjectRepositoryFake struct {
	project   TaskProject
	err       error
	userID    domain.UserID
	projectID domain.ProjectID
	calls     int
	accesses  *[]string
}

func (repo *removeTaskFromProjectProjectRepositoryFake) GetProjectByUserID(_ context.Context, userID, projectID string) (TaskProject, error) {
	repo.calls++
	repo.userID, repo.projectID = domain.UserID(userID), domain.ProjectID(projectID)
	*repo.accesses = append(*repo.accesses, "get-project")
	if repo.err != nil {
		return TaskProject{}, repo.err
	}
	return repo.project, nil
}

func (repo *removeTaskFromProjectProjectRepositoryFake) LockProjectByUserIDWithPermission(ctx context.Context, userID, projectID string, _ shared.Capability) (TaskProject, error) {
	project, err := repo.GetProjectByUserID(ctx, userID, projectID)
	if err != nil {
		return TaskProject{}, err
	}
	if project.ID != projectID || project.OwnerID != userID {
		return TaskProject{}, ErrTaskProjectNotFound
	}
	return project, nil
}

func (repo *removeTaskFromProjectProjectRepositoryFake) GetProjectByUserIDWithPermission(ctx context.Context, userID, projectID string, _ shared.Capability) (TaskProject, error) {
	return repo.GetProjectByUserID(ctx, userID, projectID)
}

type removeTaskFromProjectTaskRepositoryFake struct {
	taskProgressTestRepository
	task                   dao.Task
	removedTask            dao.Task
	getErr                 error
	removeErr              error
	userID                 domain.UserID
	taskID                 domain.TaskID
	removeUserID           domain.UserID
	removeTaskID           domain.TaskID
	removeProjectID        domain.ProjectID
	removeExpectedRevision int32
	getCalls               int
	removeCalls            int
	accesses               *[]string
}

func (repo *removeTaskFromProjectTaskRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.getCalls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "get-task")
	if repo.getErr != nil {
		return dao.Task{}, repo.getErr
	}
	return repo.task, nil
}

func (repo *removeTaskFromProjectTaskRepositoryFake) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}

func (repo *removeTaskFromProjectTaskRepositoryFake) RemoveFromProjectByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error) {
	repo.removeCalls++
	repo.removeUserID, repo.removeTaskID, repo.removeProjectID = userID, taskID, projectID
	repo.removeExpectedRevision = expectedRevision
	*repo.accesses = append(*repo.accesses, "remove-task")
	if repo.removeErr != nil {
		return dao.Task{}, repo.removeErr
	}
	return repo.removedTask, nil
}

func newRemoveTaskFromProjectFixture(project legacyProjectFixture, task dao.Task) (*removeTaskFromProjectUOWFake, *removeTaskFromProjectProjectRepositoryFake, *removeTaskFromProjectTaskRepositoryFake, *[]string) {
	var accesses []string
	projectRepo := &removeTaskFromProjectProjectRepositoryFake{project: TaskProject{ID: project.ID, OwnerID: project.UserID}, accesses: &accesses}
	taskRepo := &removeTaskFromProjectTaskRepositoryFake{task: task, accesses: &accesses}
	uow := &removeTaskFromProjectUOWFake{repos: removeTaskFromProjectRepositoriesFake{
		projects: projectRepo,
		tasks:    taskRepo,
		accesses: &accesses,
	}}
	return uow, projectRepo, taskRepo, &accesses
}

func TestRemoveTaskFromProjectUseCaseExecuteRemovesTaskAndPreservesTaskData(t *testing.T) {
	userID, projectID, taskID := domain.UserID("user-1"), domain.ProjectID("project-1"), domain.TaskID("task-1")
	childMinutes := 45
	want := dao.Task{
		ID: string(taskID), UserID: string(userID), ProjectID: "", Title: "Task",
		Description: "Keep task and children", EstimatedMinutes: &childMinutes,
		Priority: dao.Priority{Value: "high"}, Status: dao.TaskStatus{Value: "open"}, Revision: 2,
	}
	uow, projectRepo, taskRepo, accesses := newRemoveTaskFromProjectFixture(
		legacyProjectFixture{ID: string(projectID), UserID: string(userID)},
		dao.Task{ID: string(taskID), UserID: string(userID), ProjectID: string(projectID), Title: want.Title, Description: want.Description, EstimatedMinutes: &childMinutes, Priority: want.Priority, Status: want.Status, Revision: 1},
	)
	taskRepo.removedTask = want

	got, err := NewRemoveTaskFromProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), userID, projectID, taskID, 1)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if uow.calls != 1 || projectRepo.calls != 1 || taskRepo.getCalls != 1 || taskRepo.removeCalls != 1 {
		t.Errorf("calls = UOW:%d project:%d task get:%d remove:%d, want 1 each", uow.calls, projectRepo.calls, taskRepo.getCalls, taskRepo.removeCalls)
	}
	if projectRepo.userID != userID || projectRepo.projectID != projectID || taskRepo.userID != userID || taskRepo.taskID != taskID || taskRepo.removeUserID != userID || taskRepo.removeTaskID != taskID || taskRepo.removeProjectID != projectID || taskRepo.removeExpectedRevision != 1 {
		t.Errorf("repository arguments do not match requested owner, project, and task")
	}
	if !reflect.DeepEqual(*accesses, []string{"projects", "get-project", "tasks", "get-task", "remove-task"}) {
		t.Errorf("repository access order = %v, want project and task checks before removal", *accesses)
	}
}

func TestRemoveTaskFromProjectUseCaseExecuteIsIdempotentWhenTaskNotInProject(t *testing.T) {
	userID, projectID := domain.UserID("user-1"), domain.ProjectID("project-1")
	for _, testCase := range []struct {
		name string
		task dao.Task
	}{
		{name: "already detached", task: dao.Task{ID: "task-1", UserID: string(userID), Priority: dao.Priority{Value: "high"}, Revision: 1}},
		{name: "belongs to another project", task: dao.Task{ID: "task-1", UserID: string(userID), ProjectID: "project-2", Priority: dao.Priority{Value: "urgent"}, Revision: 1}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			uow, _, taskRepo, _ := newRemoveTaskFromProjectFixture(legacyProjectFixture{ID: string(projectID), UserID: string(userID)}, testCase.task)

			got, err := NewRemoveTaskFromProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), userID, projectID, "task-1", 1)
			want := testCase.task
			want.Status = dao.TaskStatus{Value: "open"} // Return the task with current child-progress status.
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("Execute() = %#v, %v; want projected task %#v, nil", got, err, want)
			}
			if taskRepo.removeCalls != 0 {
				t.Errorf("RemoveFromProjectByUserID() calls = %d, want 0", taskRepo.removeCalls)
			}
		})
	}
}

func TestRemoveTaskFromProjectUseCaseExecuteReturnsNotFoundForUnownedOrMissingProject(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		project legacyProjectFixture
		err     error
	}{
		{name: "missing project", err: ErrTaskProjectNotFound},
		{name: "project owned by another user", project: legacyProjectFixture{ID: "project-1", UserID: "other-user"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			uow, _, taskRepo, _ := newRemoveTaskFromProjectFixture(testCase.project, dao.Task{ID: "task-1", UserID: "user-1", ProjectID: "project-1"})
			if testCase.err != nil {
				uow.repos.(removeTaskFromProjectRepositoriesFake).projects.(*removeTaskFromProjectProjectRepositoryFake).err = testCase.err
			}

			got, err := NewRemoveTaskFromProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1)
			if !errors.Is(err, ErrTaskProjectNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskProjectNotFound)
			}
			if got != (dao.Task{}) || taskRepo.getCalls != 0 || taskRepo.removeCalls != 0 {
				t.Errorf("result = %#v, task reads = %d, removals = %d; want no task access", got, taskRepo.getCalls, taskRepo.removeCalls)
			}
		})
	}
}

func TestRemoveTaskFromProjectUseCaseExecuteReturnsNotFoundForUnownedOrMissingTask(t *testing.T) {
	for _, testCase := range []struct {
		name string
		task dao.Task
		err  error
	}{
		{name: "missing task", err: ErrTaskNotFound},
		{name: "task owned by another user", task: dao.Task{ID: "task-1", UserID: "other-user", ProjectID: "project-1"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			uow, _, taskRepo, _ := newRemoveTaskFromProjectFixture(legacyProjectFixture{ID: "project-1", UserID: "user-1"}, testCase.task)
			if testCase.err != nil {
				taskRepo.getErr = testCase.err
			}

			got, err := NewRemoveTaskFromProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1)
			if !errors.Is(err, ErrTaskNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
			}
			if got != (dao.Task{}) || taskRepo.removeCalls != 0 {
				t.Errorf("result = %#v and removals = %d, want zero result and no removal", got, taskRepo.removeCalls)
			}
		})
	}
}

func TestRemoveTaskFromProjectUseCaseExecutePropagatesUOWError(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &removeTaskFromProjectUOWFake{err: wantErr}

	got, err := NewRemoveTaskFromProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1)
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if got != (dao.Task{}) || uow.calls != 1 {
		t.Errorf("result = %#v and UOW calls = %d, want zero result and 1 call", got, uow.calls)
	}
}

func TestRemoveTaskFromProjectUseCaseExecutePropagatesRepositoryErrors(t *testing.T) {
	projectErr := errors.New("project lookup failed")
	uow, projectRepo, taskRepo, _ := newRemoveTaskFromProjectFixture(legacyProjectFixture{ID: "project-1", UserID: "user-1"}, dao.Task{ID: "task-1", UserID: "user-1", ProjectID: "project-1"})
	projectRepo.err = projectErr
	if _, err := NewRemoveTaskFromProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1); !errors.Is(err, projectErr) {
		t.Errorf("project lookup error = %v, want %v", err, projectErr)
	}

	taskErr := errors.New("task lookup failed")
	uow, _, taskRepo, _ = newRemoveTaskFromProjectFixture(legacyProjectFixture{ID: "project-1", UserID: "user-1"}, dao.Task{ID: "task-1", UserID: "user-1", ProjectID: "project-1", Revision: 1})
	taskRepo.getErr = taskErr
	if _, err := NewRemoveTaskFromProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1); !errors.Is(err, taskErr) {
		t.Errorf("task lookup error = %v, want %v", err, taskErr)
	}

	removeErr := errors.New("project removal failed")
	uow, _, taskRepo, _ = newRemoveTaskFromProjectFixture(legacyProjectFixture{ID: "project-1", UserID: "user-1"}, dao.Task{ID: "task-1", UserID: "user-1", ProjectID: "project-1", Revision: 1})
	taskRepo.removeErr = removeErr
	if got, err := NewRemoveTaskFromProjectUseCase(uow, &taskProgressSourceFake{}, nil).Execute(context.Background(), "user-1", "project-1", "task-1", 1); !errors.Is(err, removeErr) || got != (dao.Task{}) {
		t.Errorf("removal result = %#v, %v; want zero result and %v", got, err, removeErr)
	}
}

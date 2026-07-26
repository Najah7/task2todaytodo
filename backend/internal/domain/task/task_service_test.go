package task

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTaskServiceCreateStandaloneTask(t *testing.T) {
	repo := &fakeTaskRepository{}
	service := NewTaskService(repo, &fakeProjectRepository{})
	estimated := 30

	got, err := service.CreateStandaloneTask(context.Background(), fixedTaskIDGen("task-1"), "user-1", "Write tests", "Cover service", time.Time{}, &estimated, nil, "", "")
	if err != nil {
		t.Fatalf("CreateStandaloneTask() error = %v", err)
	}

	if got.ID != "task-1" || got.UserID != "user-1" || got.ProjectID != "" {
		t.Fatalf("task = %+v, want standalone task for user", got)
	}
	if got.Progress != 0 || got.Priority.String() != "low" || got.Status.String() != "open" {
		t.Fatalf("task = %+v, want default progress, priority, and status", got)
	}
	if repo.created.ProjectID != "" || repo.created.EstimatedMinutes == nil || *repo.created.EstimatedMinutes != 30 {
		t.Fatalf("created task = %+v, want standalone estimated task", repo.created)
	}
}

func TestTaskServiceCreateProjectTask(t *testing.T) {
	repo := &fakeTaskRepository{}
	service := NewTaskService(repo, &fakeProjectRepository{})

	got, err := service.CreateProjectTask(context.Background(), fixedTaskIDGen("task-1"), "user-1", "project-1", "Write API", "", time.Time{}, nil, nil, "high", "in_progress")
	if err != nil {
		t.Fatalf("CreateProjectTask() error = %v", err)
	}

	if got.ProjectID != "project-1" || got.Priority.String() != "high" || got.Status.String() != "in_progress" {
		t.Fatalf("task = %+v, want project-scoped task with provided values", got)
	}
	if repo.createdInProject.ProjectID != "project-1" {
		t.Fatalf("createdInProject = %+v, want project_id from path", repo.createdInProject)
	}
}

func TestTaskServiceCreateProjectTaskInheritsProjectPriority(t *testing.T) {
	repo := &fakeTaskRepository{}
	projectRepo := &fakeProjectRepository{
		projectByUser: mustExistingProjectForService(t, "project-1", "user-1", "work", "Project", "", "", 0, "urgent"),
	}
	service := NewTaskService(repo, projectRepo)

	got, err := service.CreateProjectTask(context.Background(), fixedTaskIDGen("task-1"), "user-1", "project-1", "Write API", "", time.Time{}, nil, nil, "", "")
	if err != nil {
		t.Fatalf("CreateProjectTask() error = %v", err)
	}

	if got.Priority.String() != "urgent" {
		t.Fatalf("task = %+v, want project priority inherited", got)
	}
	if projectRepo.getByUserID != "user-1" || projectRepo.getByProjectID != "project-1" {
		t.Fatalf("project lookup user=%q project=%q, want scoped lookup", projectRepo.getByUserID, projectRepo.getByProjectID)
	}
}

func TestTaskServiceCreateProjectTaskNotFound(t *testing.T) {
	repo := &fakeTaskRepository{createInProjectErr: ErrTaskProjectNotFound}
	service := NewTaskService(repo, &fakeProjectRepository{})

	_, err := service.CreateProjectTask(context.Background(), fixedTaskIDGen("task-1"), "user-1", "project-1", "Write API", "", time.Time{}, nil, nil, "", "")
	if !errors.Is(err, ErrTaskProjectNotFound) {
		t.Fatalf("CreateProjectTask() error = %v, want ErrTaskProjectNotFound", err)
	}
}

func TestTaskServiceCreateProjectTaskInheritPriorityProjectNotFound(t *testing.T) {
	repo := &fakeTaskRepository{}
	projectRepo := &fakeProjectRepository{getByUserErr: ErrProjectNotFound}
	service := NewTaskService(repo, projectRepo)

	_, err := service.CreateProjectTask(context.Background(), fixedTaskIDGen("task-1"), "user-1", "project-1", "Write API", "", time.Time{}, nil, nil, "", "")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("CreateProjectTask() error = %v, want ErrProjectNotFound", err)
	}
	if repo.createdInProject.ID != "" {
		t.Fatalf("createdInProject = %+v, want repository create not called", repo.createdInProject)
	}
}

func TestTaskServiceGetTaskReturnsAggregate(t *testing.T) {
	repo := &fakeTaskRepository{
		aggregate: TaskAggregate{
			Task: Task{ID: "task-1", UserID: "user-1", Title: "Task"},
			TodoItems: []TodoItem{
				{ID: "todo-1", TaskID: "task-1", Title: "Todo"},
			},
			TaskSchedules: []TaskSchedule{
				{ID: "schedule-1", TaskID: "task-1", Title: "Schedule"},
			},
		},
	}
	service := NewTaskService(repo, &fakeProjectRepository{})

	got, err := service.GetTask(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}

	if got.Task.ID != "task-1" || len(got.TodoItems) != 1 || len(got.TaskSchedules) != 1 {
		t.Fatalf("aggregate = %+v, want task with children", got)
	}
	if repo.aggregateUserID != "user-1" || repo.aggregateTaskID != "task-1" {
		t.Fatalf("repo lookup user=%q task=%q, want scoped lookup", repo.aggregateUserID, repo.aggregateTaskID)
	}
}

func TestTaskServiceUpdateTaskBasicPreservesDisallowedFields(t *testing.T) {
	estimated := 20
	actual := 5
	existingDueDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	resetDueDate := time.Time{}
	repo := &fakeTaskRepository{
		taskByUser: mustExistingTask(t, "task-1", "user-1", "project-1", "Old", "Old description", &estimated, &actual, 42, "urgent", "open"),
	}
	repo.taskByUser.DueDate = existingDueDate
	service := NewTaskService(repo, &fakeProjectRepository{})
	title := "New title"
	description := "New description"
	status := "done"

	got, err := service.UpdateTaskBasic(context.Background(), "user-1", "task-1", &title, &description, &status, &resetDueDate)
	if err != nil {
		t.Fatalf("UpdateTaskBasic() error = %v", err)
	}

	if got.Title != "New title" || got.Description != "New description" || got.Status.String() != "done" || !got.DueDate.IsZero() {
		t.Fatalf("task = %+v, want allowed fields updated", got)
	}
	if got.ProjectID != "project-1" || got.Priority.String() != "urgent" || got.Progress != 42 {
		t.Fatalf("task = %+v, want project, priority, and progress preserved", got)
	}
	if got.EstimatedMinutes == nil || *got.EstimatedMinutes != 20 || got.ActualMinutes == nil || *got.ActualMinutes != 5 {
		t.Fatalf("task = %+v, want estimation preserved", got)
	}
}

func TestTaskServiceUpdateTaskPriorityPreservesOtherFields(t *testing.T) {
	estimated := 20
	repo := &fakeTaskRepository{
		taskByUser: mustExistingTask(t, "task-1", "user-1", "project-1", "Task", "", &estimated, nil, 50, "low", "in_progress"),
	}
	service := NewTaskService(repo, &fakeProjectRepository{})

	got, err := service.UpdateTaskPriority(context.Background(), "user-1", "task-1", "high")
	if err != nil {
		t.Fatalf("UpdateTaskPriority() error = %v", err)
	}

	if got.Priority.String() != "high" {
		t.Fatalf("task = %+v, want priority updated", got)
	}
	if got.ProjectID != "project-1" || got.Status.String() != "in_progress" || got.Progress != 50 {
		t.Fatalf("task = %+v, want other fields preserved", got)
	}
}

func TestTaskServiceUpdateTaskEstimation(t *testing.T) {
	estimated := 20
	actual := 5
	newActual := 15
	repo := &fakeTaskRepository{
		taskByUser: mustExistingTask(t, "task-1", "user-1", "", "Task", "", &estimated, &actual, 10, "low", "open"),
	}
	service := NewTaskService(repo, &fakeProjectRepository{})

	got, err := service.UpdateTaskEstimation(context.Background(), "user-1", "task-1", nil, &newActual)
	if err != nil {
		t.Fatalf("UpdateTaskEstimation() error = %v", err)
	}

	if got.EstimatedMinutes == nil || *got.EstimatedMinutes != 20 {
		t.Fatalf("task = %+v, want estimated minutes preserved", got)
	}
	if got.ActualMinutes == nil || *got.ActualMinutes != 15 {
		t.Fatalf("task = %+v, want actual minutes updated", got)
	}
	if got.Progress != 10 {
		t.Fatalf("task = %+v, want progress preserved", got)
	}
}

func TestTaskServiceUpdateTaskEstimationRejectsEmptyRequest(t *testing.T) {
	service := NewTaskService(&fakeTaskRepository{}, &fakeProjectRepository{})

	_, err := service.UpdateTaskEstimation(context.Background(), "user-1", "task-1", nil, nil)
	if !errors.Is(err, ErrTaskEstimationUpdateEmpty) {
		t.Fatalf("UpdateTaskEstimation() error = %v, want ErrTaskEstimationUpdateEmpty", err)
	}
}

func TestTaskServiceDeleteTaskScopesByUser(t *testing.T) {
	repo := &fakeTaskRepository{}
	service := NewTaskService(repo, &fakeProjectRepository{})

	if err := service.DeleteTask(context.Background(), "user-1", "task-1"); err != nil {
		t.Fatalf("DeleteTask() error = %v", err)
	}

	if repo.deletedUserID != "user-1" || repo.deletedTaskID != "task-1" {
		t.Fatalf("delete user=%q task=%q, want scoped delete", repo.deletedUserID, repo.deletedTaskID)
	}
}

type fakeTaskRepository struct {
	TaskRepository

	created            Task
	createdInProject   Task
	createInProjectErr error
	taskByUser         Task
	taskByUserErr      error
	updated            Task
	aggregate          TaskAggregate
	aggregateUserID    UserID
	aggregateTaskID    TaskID
	deletedUserID      UserID
	deletedTaskID      TaskID
	deleteByUserErr    error
}

func (r *fakeTaskRepository) Create(_ context.Context, task Task) (Task, error) {
	r.created = task
	return task, nil
}

func (r *fakeTaskRepository) CreateInProject(_ context.Context, task Task) (Task, error) {
	r.createdInProject = task
	if r.createInProjectErr != nil {
		return NewZeroTask(), r.createInProjectErr
	}
	return task, nil
}

func (r *fakeTaskRepository) GetByUser(_ context.Context, _ UserID, _ TaskID) (Task, error) {
	if r.taskByUserErr != nil {
		return NewZeroTask(), r.taskByUserErr
	}
	return r.taskByUser, nil
}

func (r *fakeTaskRepository) GetAggregateByUser(_ context.Context, userID UserID, taskID TaskID) (TaskAggregate, error) {
	r.aggregateUserID = userID
	r.aggregateTaskID = taskID
	return r.aggregate, nil
}

func (r *fakeTaskRepository) UpdateByUser(_ context.Context, task Task) (Task, error) {
	r.updated = task
	return task, nil
}

func (r *fakeTaskRepository) DeleteByUser(_ context.Context, userID UserID, taskID TaskID) error {
	r.deletedUserID = userID
	r.deletedTaskID = taskID
	return r.deleteByUserErr
}

func fixedTaskIDGen(id string) func() string {
	return func() string {
		return id
	}
}

func mustExistingTask(
	t *testing.T,
	id TaskID,
	userID UserID,
	projectID ProjectID,
	title string,
	description string,
	estimatedMinutes *int,
	actualMinutes *int,
	progress int,
	priorityValue string,
	statusValue string,
) Task {
	t.Helper()
	priority := mustTaskPriority(t, priorityValue)
	status := mustTaskStatus(t, statusValue)
	createdAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	task, err := NewExistingTask(id, userID, projectID, title, description, time.Time{}, estimatedMinutes, actualMinutes, progress, priority, status, createdAt, updatedAt)
	if err != nil {
		t.Fatalf("NewExistingTask() error = %v", err)
	}
	return task
}

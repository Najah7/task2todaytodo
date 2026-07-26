package task

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
)

var (
	ErrTaskNotFound              = errors.New("task not found")
	ErrTaskProjectNotFound       = errors.New("project not found")
	ErrTaskEstimationUpdateEmpty = errors.New("estimated minutes or actual minutes must be provided")
)

type TaskAggregate struct {
	Task          Task
	TodoItems     []TodoItem
	TaskSchedules []TaskSchedule
}

type TaskService struct {
	repo        TaskRepository
	projectRepo ProjectRepository
}

func NewTaskService(repo TaskRepository, projectRepo ProjectRepository) *TaskService {
	return &TaskService{
		repo:        repo,
		projectRepo: projectRepo,
	}
}

func (s *TaskService) GetTask(ctx context.Context, userID UserID, taskID TaskID) (TaskAggregate, error) {
	return s.repo.GetAggregateByUser(ctx, userID, taskID)
}

func (s *TaskService) CreateStandaloneTask(
	ctx context.Context,
	idGen func() string,
	userID UserID,
	title string,
	description string,
	estimatedMinutes *int,
	actualMinutes *int,
	priorityValue string,
	statusValue string,
) (Task, error) {
	newTask, err := newTaskForCreate(idGen, userID, "", title, description, estimatedMinutes, actualMinutes, priorityValue, statusValue)
	if err != nil {
		return NewZeroTask(), err
	}

	return s.repo.Create(ctx, newTask)
}

func (s *TaskService) CreateProjectTask(
	ctx context.Context,
	idGen func() string,
	userID UserID,
	projectID ProjectID,
	title string,
	description string,
	estimatedMinutes *int,
	actualMinutes *int,
	priorityValue string,
	statusValue string,
) (Task, error) {
	if priorityValue == "" {
		project, err := s.projectRepo.GetByUser(ctx, userID, projectID)
		if err != nil {
			return NewZeroTask(), err
		}
		priorityValue = project.Priority.String()
	}

	newTask, err := newTaskForCreate(idGen, userID, projectID, title, description, estimatedMinutes, actualMinutes, priorityValue, statusValue)
	if err != nil {
		return NewZeroTask(), err
	}

	return s.repo.CreateInProject(ctx, newTask)
}

func (s *TaskService) UpdateTaskBasic(
	ctx context.Context,
	userID UserID,
	taskID TaskID,
	title *string,
	description *string,
	statusValue *string,
) (Task, error) {
	existing, err := s.repo.GetByUser(ctx, userID, taskID)
	if err != nil {
		return NewZeroTask(), err
	}

	updated := existing
	if title != nil {
		updated.Title = *title
	}
	if description != nil {
		updated.Description = *description
	}
	if statusValue != nil {
		status, err := NewTaskStatus(*statusValue)
		if err != nil {
			return NewZeroTask(), err
		}
		updated.Status = status
	}
	if err := updated.Validate(); err != nil {
		return NewZeroTask(), err
	}

	return s.repo.UpdateByUser(ctx, updated)
}

func (s *TaskService) UpdateTaskPriority(ctx context.Context, userID UserID, taskID TaskID, priorityValue string) (Task, error) {
	existing, err := s.repo.GetByUser(ctx, userID, taskID)
	if err != nil {
		return NewZeroTask(), err
	}

	priority, err := NewTaskPriority(priorityValue)
	if err != nil {
		return NewZeroTask(), err
	}

	updated := existing
	updated.Priority = priority
	if err := updated.Validate(); err != nil {
		return NewZeroTask(), err
	}

	return s.repo.UpdateByUser(ctx, updated)
}

func (s *TaskService) UpdateTaskEstimation(ctx context.Context, userID UserID, taskID TaskID, estimatedMinutes *int, actualMinutes *int) (Task, error) {
	if estimatedMinutes == nil && actualMinutes == nil {
		return NewZeroTask(), ErrTaskEstimationUpdateEmpty
	}

	existing, err := s.repo.GetByUser(ctx, userID, taskID)
	if err != nil {
		return NewZeroTask(), err
	}

	updated := existing
	if estimatedMinutes != nil {
		updated.EstimatedMinutes = estimatedMinutes
	}
	if actualMinutes != nil {
		updated.ActualMinutes = actualMinutes
	}
	if err := updated.Validate(); err != nil {
		return NewZeroTask(), err
	}

	return s.repo.UpdateByUser(ctx, updated)
}

func (s *TaskService) DeleteTask(ctx context.Context, userID UserID, taskID TaskID) error {
	return s.repo.DeleteByUser(ctx, userID, taskID)
}

func newTaskForCreate(
	idGen func() string,
	userID UserID,
	projectID ProjectID,
	title string,
	description string,
	estimatedMinutes *int,
	actualMinutes *int,
	priorityValue string,
	statusValue string,
) (Task, error) {
	id, err := shared.NewID(idGen())
	if err != nil {
		return NewZeroTask(), err
	}

	priority, err := newTaskPriorityOrDefault(priorityValue)
	if err != nil {
		return NewZeroTask(), err
	}

	status, err := newTaskStatusOrDefault(statusValue)
	if err != nil {
		return NewZeroTask(), err
	}

	return NewTaskWithDetails(
		TaskID(id),
		userID,
		projectID,
		title,
		description,
		estimatedMinutes,
		actualMinutes,
		0,
		priority,
		status,
	)
}

func newTaskPriorityOrDefault(value string) (TaskPriority, error) {
	if value == "" {
		value = "low"
	}
	return NewTaskPriority(value)
}

func newTaskStatusOrDefault(value string) (TaskStatus, error) {
	if value == "" {
		value = "open"
	}
	return NewTaskStatus(value)
}

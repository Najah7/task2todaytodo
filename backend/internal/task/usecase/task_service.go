package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/shared"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

var (
	ErrTaskEstimationUpdateEmpty = errors.New("estimated minutes or actual minutes must be provided")
)

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

func (s *TaskService) GetTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID) (domain.TaskAggregate, error) {
	return s.repo.GetAggregateByUser(ctx, userID, taskID)
}

func (s *TaskService) CreateStandaloneTask(
	ctx context.Context,
	idGen func() string,
	userID domain.UserID,
	title string,
	description string,
	dueDate time.Time,
	estimatedMinutes *int,
	actualMinutes *int,
	priorityValue string,
	statusValue string,
) (domain.Task, error) {
	newTask, err := newTaskForCreate(idGen, userID, "", title, description, dueDate, estimatedMinutes, actualMinutes, priorityValue, statusValue)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	return s.repo.Create(ctx, newTask)
}

func (s *TaskService) CreateProjectTask(
	ctx context.Context,
	idGen func() string,
	userID domain.UserID,
	projectID domain.ProjectID,
	title string,
	description string,
	dueDate time.Time,
	estimatedMinutes *int,
	actualMinutes *int,
	priorityValue string,
	statusValue string,
) (domain.Task, error) {
	if priorityValue == "" {
		project, err := s.projectRepo.GetByUser(ctx, userID, projectID)
		if err != nil {
			return domain.NewZeroTask(), err
		}
		priorityValue = project.Priority.String()
	}

	newTask, err := newTaskForCreate(idGen, userID, projectID, title, description, dueDate, estimatedMinutes, actualMinutes, priorityValue, statusValue)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	return s.repo.CreateInProject(ctx, newTask)
}

func (s *TaskService) UpdateTaskBasic(
	ctx context.Context,
	userID domain.UserID,
	taskID domain.TaskID,
	title *string,
	description *string,
	dueDate *time.Time,
) (domain.Task, error) {
	existing, err := s.repo.GetByUser(ctx, userID, taskID)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	updated := existing
	if title != nil {
		updated.Title = *title
	}
	if description != nil {
		updated.Description = *description
	}
	if dueDate != nil {
		updated.DueDate = *dueDate
	}
	if err := updated.Validate(); err != nil {
		return domain.NewZeroTask(), err
	}

	return s.repo.UpdateByUser(ctx, updated)
}

func (s *TaskService) UpdateTaskStatus(ctx context.Context, userID domain.UserID, taskID domain.TaskID, statusValue string) (domain.Task, error) {
	existing, err := s.repo.GetByUser(ctx, userID, taskID)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	status, err := domain.NewTaskStatus(statusValue)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	updated := existing
	updated.Status = status
	if err := updated.Validate(); err != nil {
		return domain.NewZeroTask(), err
	}

	return s.repo.UpdateByUser(ctx, updated)
}

func (s *TaskService) UpdateTaskPriority(ctx context.Context, userID domain.UserID, taskID domain.TaskID, priorityValue string) (domain.Task, error) {
	existing, err := s.repo.GetByUser(ctx, userID, taskID)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	priority, err := domain.NewTaskPriority(priorityValue)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	updated := existing
	updated.Priority = priority
	if err := updated.Validate(); err != nil {
		return domain.NewZeroTask(), err
	}

	return s.repo.UpdateByUser(ctx, updated)
}

func (s *TaskService) UpdateTaskEstimation(ctx context.Context, userID domain.UserID, taskID domain.TaskID, estimatedMinutes *int, actualMinutes *int) (domain.Task, error) {
	if estimatedMinutes == nil && actualMinutes == nil {
		return domain.NewZeroTask(), ErrTaskEstimationUpdateEmpty
	}

	existing, err := s.repo.GetByUser(ctx, userID, taskID)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	updated := existing
	if estimatedMinutes != nil {
		updated.EstimatedMinutes = estimatedMinutes
	}
	if actualMinutes != nil {
		updated.ActualMinutes = actualMinutes
	}
	if err := updated.Validate(); err != nil {
		return domain.NewZeroTask(), err
	}

	return s.repo.UpdateByUser(ctx, updated)
}

func (s *TaskService) DeleteTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID) error {
	return s.repo.DeleteByUser(ctx, userID, taskID)
}

func newTaskForCreate(
	idGen func() string,
	userID domain.UserID,
	projectID domain.ProjectID,
	title string,
	description string,
	dueDate time.Time,
	estimatedMinutes *int,
	actualMinutes *int,
	priorityValue string,
	statusValue string,
) (domain.Task, error) {
	id, err := shared.NewID(idGen())
	if err != nil {
		return domain.NewZeroTask(), err
	}

	priority, err := newTaskPriorityOrDefault(priorityValue)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	status, err := newTaskStatusOrDefault(statusValue)
	if err != nil {
		return domain.NewZeroTask(), err
	}

	return domain.NewTaskWithDetails(
		domain.TaskID(id),
		userID,
		projectID,
		title,
		description,
		dueDate,
		estimatedMinutes,
		actualMinutes,
		0,
		priority,
		status,
	)
}

func newTaskPriorityOrDefault(value string) (domain.TaskPriority, error) {
	if value == "" {
		value = "low"
	}
	return domain.NewTaskPriority(value)
}

func newTaskStatusOrDefault(value string) (domain.TaskStatus, error) {
	if value == "" {
		value = "open"
	}
	return domain.NewTaskStatus(value)
}

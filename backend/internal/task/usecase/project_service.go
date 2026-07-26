package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/shared"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

type ProjectService struct {
	repo ProjectRepository
}

func NewProjectService(repo ProjectRepository) *ProjectService {
	return &ProjectService{
		repo: repo,
	}
}

type ProjectUpdate struct {
	Type        *string
	Priority    *string
	Title       *string
	Goal        *string
	Description *string
	DueDate     *time.Time
	StartAt     *time.Time
	EndAt       *time.Time
}

func (s *ProjectService) Create(
	ctx context.Context,
	idGen func() string,
	userID domain.UserID,
	projectType string,
	priorityValue string,
	title string,
	goal string,
	description string,
	dueDate time.Time,
	startAt time.Time,
	endAt time.Time,
) (domain.Project, error) {
	id, err := shared.NewID(idGen())
	if err != nil {
		return domain.NewZeroProject(), err
	}

	if projectType == "" {
		projectType = "other"
	}
	t, err := domain.NewProjectType(projectType)
	if err != nil {
		return domain.NewZeroProject(), err
	}
	priority, err := newTaskPriorityOrDefault(priorityValue)
	if err != nil {
		return domain.NewZeroProject(), err
	}

	project, err := domain.NewProjectWithDetails(domain.ProjectID(id), userID, t, title, goal, description, dueDate, 0, priority, startAt, endAt)
	if err != nil {
		return domain.NewZeroProject(), err
	}

	return s.repo.Create(ctx, project)
}

func (s *ProjectService) GetAggregate(ctx context.Context, userID domain.UserID, id domain.ProjectID) (domain.ProjectAggregate, error) {
	return s.repo.GetAggregateByUser(ctx, userID, id)
}

func (s *ProjectService) Update(ctx context.Context, userID domain.UserID, id domain.ProjectID, update ProjectUpdate) (domain.Project, error) {
	project, err := s.repo.GetByUser(ctx, userID, id)
	if err != nil {
		return domain.NewZeroProject(), err
	}

	projectType := project.Type
	if update.Type != nil {
		projectType, err = domain.NewProjectType(*update.Type)
		if err != nil {
			return domain.NewZeroProject(), err
		}
	}
	priority := project.Priority
	if update.Priority != nil {
		priority, err = domain.NewTaskPriority(*update.Priority)
		if err != nil {
			return domain.NewZeroProject(), err
		}
	}

	title := project.Title
	if update.Title != nil {
		title = *update.Title
	}

	goal := project.Goal
	if update.Goal != nil {
		goal = *update.Goal
	}

	description := project.Description
	if update.Description != nil {
		description = *update.Description
	}

	dueDate := project.DueDate
	if update.DueDate != nil {
		dueDate = *update.DueDate
	}

	startAt := project.StartAt
	if update.StartAt != nil {
		startAt = *update.StartAt
	}

	endAt := project.EndAt
	if update.EndAt != nil {
		endAt = *update.EndAt
	}

	updated, err := domain.NewProjectWithDetails(
		project.ID,
		project.UserID,
		projectType,
		title,
		goal,
		description,
		dueDate,
		project.Progress,
		priority,
		startAt,
		endAt,
	)
	if err != nil {
		return domain.NewZeroProject(), err
	}

	return s.repo.UpdateByUser(ctx, userID, updated)
}

func (s *ProjectService) Delete(ctx context.Context, userID domain.UserID, id domain.ProjectID) error {
	return s.repo.DeleteByUser(ctx, userID, id)
}

package task

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
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
	userID UserID,
	projectType string,
	priorityValue string,
	title string,
	goal string,
	description string,
	dueDate time.Time,
	startAt time.Time,
	endAt time.Time,
) (Project, error) {
	id, err := shared.NewID(idGen())
	if err != nil {
		return NewZeroProject(), err
	}

	if projectType == "" {
		projectType = "other"
	}
	t, err := NewProjectType(projectType)
	if err != nil {
		return NewZeroProject(), err
	}
	priority, err := newTaskPriorityOrDefault(priorityValue)
	if err != nil {
		return NewZeroProject(), err
	}

	project, err := NewProjectWithDetails(ProjectID(id), userID, t, title, goal, description, dueDate, 0, priority, startAt, endAt)
	if err != nil {
		return NewZeroProject(), err
	}

	return s.repo.Create(ctx, project)
}

func (s *ProjectService) GetAggregate(ctx context.Context, userID UserID, id ProjectID) (ProjectAggregate, error) {
	return s.repo.GetAggregateByUser(ctx, userID, id)
}

func (s *ProjectService) Update(ctx context.Context, userID UserID, id ProjectID, update ProjectUpdate) (Project, error) {
	project, err := s.repo.GetByUser(ctx, userID, id)
	if err != nil {
		return NewZeroProject(), err
	}

	projectType := project.Type
	if update.Type != nil {
		projectType, err = NewProjectType(*update.Type)
		if err != nil {
			return NewZeroProject(), err
		}
	}
	priority := project.Priority
	if update.Priority != nil {
		priority, err = NewTaskPriority(*update.Priority)
		if err != nil {
			return NewZeroProject(), err
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

	updated, err := NewProjectWithDetails(
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
		return NewZeroProject(), err
	}

	return s.repo.UpdateByUser(ctx, userID, updated)
}

func (s *ProjectService) Delete(ctx context.Context, userID UserID, id ProjectID) error {
	return s.repo.DeleteByUser(ctx, userID, id)
}

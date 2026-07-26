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
	Title       *string
	Goal        *string
	Description *string
	StartAt     *time.Time
	EndAt       *time.Time
}

func (s *ProjectService) Create(
	ctx context.Context,
	idGen func() string,
	userID UserID,
	projectType string,
	title string,
	goal string,
	description string,
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

	project, err := NewProjectWithDetails(ProjectID(id), userID, t, title, goal, description, 0, startAt, endAt)
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
		project.Progress,
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

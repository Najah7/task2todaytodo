package usecase

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

type TaskPriorityService struct {
	repo TaskPriorityRepository
}

func NewTaskPriorityService(repo TaskPriorityRepository) *TaskPriorityService {
	return &TaskPriorityService{
		repo: repo,
	}
}

func (s *TaskPriorityService) List(ctx context.Context) ([]domain.TaskPriority, error) {
	return s.repo.List(ctx)
}

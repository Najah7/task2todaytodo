package usecase

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

type TaskStatusService struct {
	repo TaskStatusRepository
}

func NewTaskStatusService(repo TaskStatusRepository) *TaskStatusService {
	return &TaskStatusService{
		repo: repo,
	}
}

func (s *TaskStatusService) List(ctx context.Context) ([]domain.TaskStatus, error) {
	return s.repo.List(ctx)
}

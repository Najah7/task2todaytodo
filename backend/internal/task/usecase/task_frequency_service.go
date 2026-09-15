package usecase

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

type TaskFrequencyService struct {
	repo TaskFrequencyRepository
}

func NewTaskFrequencyService(repo TaskFrequencyRepository) *TaskFrequencyService {
	return &TaskFrequencyService{
		repo: repo,
	}
}

func (s *TaskFrequencyService) List(ctx context.Context) ([]domain.TaskFrequency, error) {
	return s.repo.List(ctx)
}

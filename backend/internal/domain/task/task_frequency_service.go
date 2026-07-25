package task

import "context"

type TaskFrequencyService struct {
	repo TaskFrequencyRepository
}

func NewTaskFrequencyService(repo TaskFrequencyRepository) *TaskFrequencyService {
	return &TaskFrequencyService{
		repo: repo,
	}
}

func (s *TaskFrequencyService) List(ctx context.Context) ([]TaskFrequency, error) {
	return s.repo.List(ctx)
}

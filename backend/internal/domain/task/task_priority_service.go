package task

import "context"

type TaskPriorityService struct {
	repo TaskPriorityRepository
}

func NewTaskPriorityService(repo TaskPriorityRepository) *TaskPriorityService {
	return &TaskPriorityService{
		repo: repo,
	}
}

func (s *TaskPriorityService) List(ctx context.Context) ([]TaskPriority, error) {
	return s.repo.List(ctx)
}

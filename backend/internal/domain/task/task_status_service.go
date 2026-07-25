package task

import "context"

type TaskStatusService struct {
	repo TaskStatusRepository
}

func NewTaskStatusService(repo TaskStatusRepository) *TaskStatusService {
	return &TaskStatusService{
		repo: repo,
	}
}

func (s *TaskStatusService) List(ctx context.Context) ([]TaskStatus, error) {
	return s.repo.List(ctx)
}

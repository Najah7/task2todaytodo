package task

import "context"

type ProjectTypeService struct {
	repo ProjectTypeRepository
}

func NewProjectTypeService(repo ProjectTypeRepository) *ProjectTypeService {
	return &ProjectTypeService{
		repo: repo,
	}
}

func (s *ProjectTypeService) List(ctx context.Context) ([]ProjectType, error) {
	return s.repo.List(ctx)
}

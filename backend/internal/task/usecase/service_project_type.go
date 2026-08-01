package usecase

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

type ProjectTypeService struct {
	repo ProjectTypeRepository
}

func NewProjectTypeService(repo ProjectTypeRepository) *ProjectTypeService {
	return &ProjectTypeService{
		repo: repo,
	}
}

func (s *ProjectTypeService) List(ctx context.Context) ([]domain.ProjectType, error) {
	return s.repo.List(ctx)
}

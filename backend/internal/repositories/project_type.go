package repositories

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
	"github.com/Najah7/task2todaytodo/internal/repositories/sqlc"
)

var _ domain.ProjectTypeRepository = ProjectTypeRepository{}

type ProjectTypeRepository struct {
	queries *sqlc.Queries
}

func NewProjectTypeRepository(db sqlc.DBTX) *ProjectTypeRepository {
	return &ProjectTypeRepository{
		queries: sqlc.New(db),
	}
}

func (r ProjectTypeRepository) List(ctx context.Context) ([]domain.ProjectType, error) {
	records, err := r.queries.ListProjectTypes(ctx)
	if err != nil {
		return nil, err
	}

	projectTypes := make([]domain.ProjectType, 0, len(records))
	for _, record := range records {
		projectType, err := domain.NewProjectType(record.Type)
		if err != nil {
			return nil, err
		}
		projectTypes = append(projectTypes, projectType)
	}
	return projectTypes, nil
}

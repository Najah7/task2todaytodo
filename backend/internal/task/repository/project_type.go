package repository

import (
	"context"

	usecase "github.com/Najah7/task2todaytodo/internal/task/usecase"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	"github.com/jackc/pgx/v5"
)

var _ usecase.ProjectTypeRepository = ProjectTypeRepository{}

type ProjectTypeRepository struct {
	queries *sqlc.Queries
}

func NewProjectTypeRepository(db sqlc.DBTX) *ProjectTypeRepository {
	return &ProjectTypeRepository{
		queries: sqlc.New(db),
	}
}

func (r *ProjectTypeRepository) WithTx(tx pgx.Tx) *ProjectTypeRepository {
	return &ProjectTypeRepository{
		queries: r.queries.WithTx(tx),
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

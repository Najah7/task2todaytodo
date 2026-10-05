package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
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

func (r ProjectTypeRepository) List(ctx context.Context) ([]dao.ProjectType, error) {
	records, err := r.queries.ListProjectTypes(ctx)
	if err != nil {
		return nil, err
	}

	projectTypes := make([]dao.ProjectType, 0, len(records))
	for _, record := range records {
		projectTypes = append(projectTypes, dao.ProjectType{
			Value:     record.Type,
			Label:     record.Label,
			LabelJp:   record.LabelJp,
			CreatedAt: pgUnix(record.CreatedAt),
			UpdatedAt: pgUnix(record.UpdatedAt),
		})
	}
	return projectTypes, nil
}

package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
)

var _ usecase.TaskFrequencyRepository = TaskFrequencyRepository{}

type TaskFrequencyRepository struct {
	queries *sqlc.Queries
}

func NewTaskFrequencyRepository(db sqlc.DBTX) *TaskFrequencyRepository {
	return &TaskFrequencyRepository{
		queries: sqlc.New(db),
	}
}

func (r *TaskFrequencyRepository) WithTx(tx pgx.Tx) *TaskFrequencyRepository {
	return &TaskFrequencyRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r TaskFrequencyRepository) List(ctx context.Context) ([]dao.TaskFrequency, error) {
	records, err := r.queries.ListTaskFrequencies(ctx)
	if err != nil {
		return nil, err
	}

	frequencies := make([]dao.TaskFrequency, 0, len(records))
	for _, record := range records {
		frequencies = append(frequencies, dao.TaskFrequency{
			Value:     record.Frequency,
			Label:     record.Label,
			LabelJp:   record.LabelJp,
			CreatedAt: pgUnix(record.CreatedAt),
			UpdatedAt: pgUnix(record.UpdatedAt),
		})
	}
	return frequencies, nil
}

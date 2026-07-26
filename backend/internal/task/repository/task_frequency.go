package repository

import (
	"context"
	usecase "github.com/Najah7/task2todaytodo/internal/task/usecase"

	"github.com/Najah7/task2todaytodo/internal/repositories/sqlc"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
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

func (r TaskFrequencyRepository) List(ctx context.Context) ([]domain.TaskFrequency, error) {
	records, err := r.queries.ListTaskFrequencies(ctx)
	if err != nil {
		return nil, err
	}

	frequencies := make([]domain.TaskFrequency, 0, len(records))
	for _, record := range records {
		frequency, err := domain.NewTaskFrequency(record.Frequency)
		if err != nil {
			return nil, err
		}
		frequencies = append(frequencies, frequency)
	}
	return frequencies, nil
}

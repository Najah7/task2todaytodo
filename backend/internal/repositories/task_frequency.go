package repositories

import (
	"context"

	domain "github.com/Najah7/task2schedule/internal/domain/task"
	"github.com/Najah7/task2schedule/internal/repositories/sqlc"
)

var _ domain.TaskFrequencyRepository = TaskFrequencyRepository{}

type TaskFrequencyRepository struct {
	queries *sqlc.Queries
}

func NewTaskFrequencyRepository(db sqlc.DBTX) *TaskFrequencyRepository {
	return &TaskFrequencyRepository{
		queries: sqlc.New(db),
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

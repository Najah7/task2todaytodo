package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
)

var _ usecase.TaskPriorityRepository = TaskPriorityRepository{}

type TaskPriorityRepository struct {
	queries *sqlc.Queries
}

func NewTaskPriorityRepository(db sqlc.DBTX) *TaskPriorityRepository {
	return &TaskPriorityRepository{
		queries: sqlc.New(db),
	}
}

func (r *TaskPriorityRepository) WithTx(tx pgx.Tx) *TaskPriorityRepository {
	return &TaskPriorityRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r TaskPriorityRepository) List(ctx context.Context) ([]dao.Priority, error) {
	records, err := r.queries.ListTaskPriorities(ctx)
	if err != nil {
		return nil, err
	}

	priorities := make([]dao.Priority, 0, len(records))
	for _, record := range records {
		priorities = append(priorities, dao.Priority{
			Value:     record.Priority,
			Label:     record.Label,
			LabelJp:   record.LabelJp,
			Weight:    int(record.Weight),
			CreatedAt: pgUnix(record.CreatedAt),
			UpdatedAt: pgUnix(record.UpdatedAt),
		})
	}
	return priorities, nil
}

package repository

import (
	"context"

	usecase "github.com/Najah7/task2todaytodo/internal/task/usecase"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	"github.com/jackc/pgx/v5"
)

var _ usecase.TaskStatusRepository = TaskStatusRepository{}

type TaskStatusRepository struct {
	queries *sqlc.Queries
}

func NewTaskStatusRepository(db sqlc.DBTX) *TaskStatusRepository {
	return &TaskStatusRepository{
		queries: sqlc.New(db),
	}
}

func (r *TaskStatusRepository) WithTx(tx pgx.Tx) *TaskStatusRepository {
	return &TaskStatusRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r TaskStatusRepository) List(ctx context.Context) ([]domain.TaskStatus, error) {
	records, err := r.queries.ListTaskStatuses(ctx)
	if err != nil {
		return nil, err
	}

	statuses := make([]domain.TaskStatus, 0, len(records))
	for _, record := range records {
		status, err := domain.NewTaskStatus(record.Status)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

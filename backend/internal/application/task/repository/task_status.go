package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
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

func (r TaskStatusRepository) List(ctx context.Context) ([]dao.TaskStatus, error) {
	records, err := r.queries.ListTaskStatuses(ctx)
	if err != nil {
		return nil, err
	}

	statuses := make([]dao.TaskStatus, 0, len(records))
	for _, record := range records {
		statuses = append(statuses, dao.TaskStatus{
			Value:     record.Status,
			Label:     record.Label,
			LabelJp:   record.LabelJp,
			CreatedAt: pgUnix(record.CreatedAt),
			UpdatedAt: pgUnix(record.UpdatedAt),
		})
	}
	return statuses, nil
}

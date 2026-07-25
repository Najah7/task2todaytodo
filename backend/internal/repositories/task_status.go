package repositories

import (
	"context"

	domain "github.com/Najah7/task2schedule/internal/domain/task"
	"github.com/Najah7/task2schedule/internal/repositories/sqlc"
)

var _ domain.TaskStatusRepository = TaskStatusRepository{}

type TaskStatusRepository struct {
	queries *sqlc.Queries
}

func NewTaskStatusRepository(db sqlc.DBTX) *TaskStatusRepository {
	return &TaskStatusRepository{
		queries: sqlc.New(db),
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

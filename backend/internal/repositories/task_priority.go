package repositories

import (
	"context"

	domain "github.com/Najah7/task2schedule/internal/domain/task"
	"github.com/Najah7/task2schedule/internal/repositories/sqlc"
)

var _ domain.TaskPriorityRepository = TaskPriorityRepository{}

type TaskPriorityRepository struct {
	queries *sqlc.Queries
}

func NewTaskPriorityRepository(db sqlc.DBTX) *TaskPriorityRepository {
	return &TaskPriorityRepository{
		queries: sqlc.New(db),
	}
}

func (r TaskPriorityRepository) List(ctx context.Context) ([]domain.TaskPriority, error) {
	records, err := r.queries.ListTaskPriorities(ctx)
	if err != nil {
		return nil, err
	}

	priorities := make([]domain.TaskPriority, 0, len(records))
	for _, record := range records {
		priority, err := domain.NewTaskPriority(record.Priority)
		if err != nil {
			return nil, err
		}
		priorities = append(priorities, priority)
	}
	return priorities, nil
}

package repositories

import (
	"context"

	domain "github.com/Najah7/task2schedule/internal/domain/task"
	"github.com/Najah7/task2schedule/internal/repositories/sqlc"
)

var _ domain.TodoItemRepository = TodoItemRepository{}

type TodoItemRepository struct {
	queries *sqlc.Queries
}

func NewTodoItemRepository(db sqlc.DBTX) *TodoItemRepository {
	return &TodoItemRepository{
		queries: sqlc.New(db),
	}
}

func (r TodoItemRepository) Get(ctx context.Context, id domain.TodoItemID) (domain.TodoItem, error) {
	record, err := r.queries.GetTodoItem(ctx, string(id))
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}
	return recordToTodoItem(record)
}

func (r TodoItemRepository) Create(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error) {
	record, err := r.queries.CreateTodoItem(ctx, sqlc.CreateTodoItemParams{
		ID:            string(item.ID),
		TaskID:        string(item.TaskID),
		Title:         item.Title,
		Description:   stringToPgText(item.Description),
		Completed:     item.Completed,
		Position:      int32(item.Position),
		IntervalWeeks: int32(item.IntervalWeeks),
	})
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}
	return recordToTodoItem(record)
}

func (r TodoItemRepository) Update(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error) {
	record, err := r.queries.UpdateTodoItem(ctx, sqlc.UpdateTodoItemParams{
		ID:            string(item.ID),
		TaskID:        string(item.TaskID),
		Title:         item.Title,
		Description:   stringToPgText(item.Description),
		Completed:     item.Completed,
		Position:      int32(item.Position),
		IntervalWeeks: int32(item.IntervalWeeks),
	})
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}
	return recordToTodoItem(record)
}

func (r TodoItemRepository) Delete(ctx context.Context, id domain.TodoItemID) error {
	return r.queries.DeleteTodoItem(ctx, string(id))
}

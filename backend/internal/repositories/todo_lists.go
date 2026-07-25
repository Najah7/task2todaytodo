package repositories

import (
	"context"
	"time"

	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
	"github.com/Najah7/task2todaytodo/internal/repositories/sqlc"
)

var _ domain.TodoListRepository = TodoListRepository{}

type TodoListRepository struct {
	queries *sqlc.Queries
}

func NewTodoListRepository(db sqlc.DBTX) *TodoListRepository {
	return &TodoListRepository{
		queries: sqlc.New(db),
	}
}

func (r TodoListRepository) Get(ctx context.Context, id domain.TodoListID) (domain.TodoList, error) {
	record, err := r.queries.GetTodoList(ctx, string(id))
	if err != nil {
		return domain.NewZeroTodoList(), err
	}
	return recordToTodoList(record)
}

func (r TodoListRepository) GetByUserAndDate(ctx context.Context, userID domain.UserID, listDate time.Time) (domain.TodoList, error) {
	record, err := r.queries.GetTodoListByUserAndDate(ctx, sqlc.GetTodoListByUserAndDateParams{
		UserID:   string(userID),
		ListDate: timeToPgDate(listDate),
	})
	if err != nil {
		return domain.NewZeroTodoList(), err
	}
	return recordToTodoList(record)
}

func (r TodoListRepository) ListByUser(ctx context.Context, userID domain.UserID) ([]domain.TodoList, error) {
	records, err := r.queries.ListTodoListsByUser(ctx, string(userID))
	if err != nil {
		return nil, err
	}
	return recordsToTodoLists(records)
}

func (r TodoListRepository) Create(ctx context.Context, list domain.TodoList) (domain.TodoList, error) {
	record, err := r.queries.CreateTodoList(ctx, sqlc.CreateTodoListParams{
		ID:       string(list.ID),
		UserID:   string(list.UserID),
		ListDate: timeToPgDate(list.ListDate),
	})
	if err != nil {
		return domain.NewZeroTodoList(), err
	}
	return recordToTodoList(record)
}

func (r TodoListRepository) Delete(ctx context.Context, id domain.TodoListID) error {
	return r.queries.DeleteTodoList(ctx, string(id))
}

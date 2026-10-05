package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
)

var _ usecase.TodoListRepository = TodoListRepository{}

type TodoListRepository struct {
	queries *sqlc.Queries
}

func NewTodoListRepository(db sqlc.DBTX) *TodoListRepository {
	return &TodoListRepository{
		queries: sqlc.New(db),
	}
}

func (r *TodoListRepository) WithTx(tx pgx.Tx) *TodoListRepository {
	return &TodoListRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r TodoListRepository) Get(ctx context.Context, id domain.TodoListID) (dao.TodoList, error) {
	record, err := r.queries.GetTodoList(ctx, string(id))
	if err != nil {
		return dao.TodoList{}, err
	}
	return recordToTodoList(record), nil
}

func (r TodoListRepository) GetByUserIDAndDate(ctx context.Context, userID domain.UserID, listDate time.Time) (dao.TodoList, error) {
	record, err := r.queries.GetTodoListByUserIDAndDate(ctx, sqlc.GetTodoListByUserIDAndDateParams{
		UserID:   string(userID),
		ListDate: timeToPgDate(listDate),
	})
	if err != nil {
		return dao.TodoList{}, err
	}
	return recordToTodoList(record), nil
}

func (r TodoListRepository) ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.TodoList, error) {
	records, err := r.queries.ListTodoListsByUserID(ctx, string(userID))
	if err != nil {
		return nil, err
	}
	return recordsToTodoLists(records), nil
}

func (r TodoListRepository) Create(ctx context.Context, list domain.TodoList) (dao.TodoList, error) {
	record, err := r.queries.CreateTodoList(ctx, sqlc.CreateTodoListParams{
		ID:       string(list.ID),
		UserID:   string(list.UserID),
		ListDate: timeToPgDate(list.ListDate),
	})
	if err != nil {
		return dao.TodoList{}, err
	}
	return recordToTodoList(record), nil
}

func (r TodoListRepository) Delete(ctx context.Context, id domain.TodoListID) error {
	return r.queries.DeleteTodoList(ctx, string(id))
}

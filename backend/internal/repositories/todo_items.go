package repositories

import (
	"context"
	"errors"

	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
	"github.com/Najah7/task2todaytodo/internal/repositories/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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

func (r *TodoItemRepository) WithTx(tx pgx.Tx) *TodoItemRepository {
	return &TodoItemRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r TodoItemRepository) Get(ctx context.Context, id domain.TodoItemID) (domain.TodoItem, error) {
	record, err := r.queries.GetTodoItem(ctx, string(id))
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}
	return recordToTodoItemRow(record)
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
		DueDate:       timeToPgDate(item.DueDate),
	})
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}
	return recordToTodoItem(record)
}

func (r TodoItemRepository) CreateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.TodoItem, appendToTail bool) (domain.TodoItem, error) {
	position := pgtype.Int4{}
	if !appendToTail {
		position = pgtype.Int4{Int32: int32(item.Position), Valid: true}
	}
	record, err := r.queries.CreateTodoItemByTaskAndUser(ctx, sqlc.CreateTodoItemByTaskAndUserParams{
		TaskID:        string(item.TaskID),
		UserID:        string(userID),
		Position:      position,
		ID:            string(item.ID),
		Title:         item.Title,
		Description:   stringToPgText(item.Description),
		IntervalWeeks: int32(item.IntervalWeeks),
		DueDate:       timeToPgDate(item.DueDate),
		Frequencies:   taskFrequencyStrings(item.Frequencies),
	})
	if err != nil {
		return domain.NewZeroTodoItem(), todoItemTaskNotFoundError(err)
	}
	return recordToCreatedTodoItemByTaskAndUserRow(record)
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
		DueDate:       timeToPgDate(item.DueDate),
	})
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}
	return recordToTodoItem(record)
}

func (r TodoItemRepository) CheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	_, err := r.queries.SetTodoItemCompletedByTaskAndUser(ctx, sqlc.SetTodoItemCompletedByTaskAndUserParams{
		ID:        string(id),
		TaskID:    string(taskID),
		UserID:    string(userID),
		Completed: true,
	})
	return todoItemRepositoryError(err)
}

func (r TodoItemRepository) UncheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	_, err := r.queries.SetTodoItemCompletedByTaskAndUser(ctx, sqlc.SetTodoItemCompletedByTaskAndUserParams{
		ID:        string(id),
		TaskID:    string(taskID),
		UserID:    string(userID),
		Completed: false,
	})
	return todoItemRepositoryError(err)
}

func (r TodoItemRepository) Delete(ctx context.Context, id domain.TodoItemID) error {
	return r.queries.DeleteTodoItem(ctx, string(id))
}

func (r TodoItemRepository) DeleteForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	_, err := r.queries.DeleteTodoItemByTaskAndUser(ctx, sqlc.DeleteTodoItemByTaskAndUserParams{
		ID:     string(id),
		TaskID: string(taskID),
		UserID: string(userID),
	})
	return todoItemRepositoryError(err)
}

func todoItemRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTodoItemNotFound
	}
	return todoItemConstraintError(err)
}

func todoItemTaskNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTodoItemTaskNotFound
	}
	return todoItemConstraintError(err)
}

func todoItemConstraintError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "todo_items_task_id_position_key" {
		return domain.ErrTodoItemPositionConflict
	}
	return err
}

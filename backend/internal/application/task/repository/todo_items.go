package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.TodoItemRepository = TodoItemRepository{}

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

func (r TodoItemRepository) Get(ctx context.Context, id domain.TodoItemID) (dao.TodoItem, error) {
	record, err := r.queries.GetTodoItem(ctx, string(id))
	if err != nil {
		return dao.TodoItem{}, err
	}
	return recordToTodoItemRow(record), nil
}

func (r TodoItemRepository) GetForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) (dao.TodoItem, error) {
	record, err := r.queries.GetTodoItemByTaskAndUserID(ctx, sqlc.GetTodoItemByTaskAndUserIDParams{
		ID:       string(id),
		TaskID:   string(taskID),
		ActorKey: string(userID),
	})
	if err != nil {
		return dao.TodoItem{}, todoItemRepositoryError(err)
	}
	return recordToTodoItemByTaskAndUserIDRow(record), nil
}

func (r TodoItemRepository) GetForCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID, capability shared.Capability) (dao.TodoItem, error) {
	record, err := r.queries.GetTodoItemByTaskAndUserIDForCommand(ctx, sqlc.GetTodoItemByTaskAndUserIDForCommandParams{
		ID:         string(id),
		TaskID:     string(taskID),
		ActorID:    string(userID),
		ResourceID: string(capability.Resource),
		Action:     sqlc.Action(capability.Action),
	})
	if err != nil {
		return dao.TodoItem{}, todoItemRepositoryError(err)
	}
	item := todoItemDAO(record.ID, record.TaskID, record.Title, record.Description, record.DueDate, record.Completed, record.Position, record.IntervalWeeks, record.Frequencies, record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException, sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
	if record.RepeatState.Valid {
		item.RepeatState = record.RepeatState.String
	}
	item.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
	return item, nil
}

func (r TodoItemRepository) ListByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TodoItem, error) {
	records, err := r.queries.ListTodoItemsByTaskAndUserID(ctx, sqlc.ListTodoItemsByTaskAndUserIDParams{
		TaskID: string(taskID),
		UserID: string(userID),
	})
	if err != nil {
		return nil, err
	}
	return recordsToTodoItems(records), nil
}

// ListByTaskForOccurrenceProjection includes deleted rows so callers can suppress tombstoned virtual dates.
func (r TodoItemRepository) ListByTaskForOccurrenceProjection(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TodoItem, error) {
	records, err := r.queries.ListTodoItemsForOccurrenceProjectionByTaskAndUserID(ctx, sqlc.ListTodoItemsForOccurrenceProjectionByTaskAndUserIDParams{
		TaskID: string(taskID),
		UserID: string(userID),
	})
	if err != nil {
		return nil, err
	}
	items := make([]dao.TodoItem, 0, len(records))
	for _, row := range records {
		item := todoItemDAO(row.ID, row.TaskID, row.Title, row.Description, row.DueDate, row.Completed, row.Position, row.IntervalWeeks, row.Frequencies, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.CreatedAt, row.UpdatedAt)
		if row.RepeatState.Valid {
			item.RepeatState = row.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		items = append(items, item)
	}
	return items, nil
}

func (r TodoItemRepository) ListByTaskForOccurrenceCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, capability shared.Capability) ([]dao.TodoItem, error) {
	records, err := r.queries.ListTodoItemsForOccurrenceCommandByTaskAndUserID(ctx, sqlc.ListTodoItemsForOccurrenceCommandByTaskAndUserIDParams{
		TaskID:     string(taskID),
		ActorID:    string(userID),
		ResourceID: string(capability.Resource),
		Action:     sqlc.Action(capability.Action),
	})
	if err != nil {
		return nil, err
	}
	items := make([]dao.TodoItem, 0, len(records))
	for _, row := range records {
		item := todoItemDAO(row.ID, row.TaskID, row.Title, row.Description, row.DueDate, row.Completed, row.Position, row.IntervalWeeks, row.Frequencies, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.CreatedAt, row.UpdatedAt)
		if row.RepeatState.Valid {
			item.RepeatState = row.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		items = append(items, item)
	}
	return items, nil
}

func (r TodoItemRepository) ListByTaskCursor(ctx context.Context, userID domain.UserID, taskID domain.TaskID, limit int, anchor *usecase.CursorAnchor) ([]dao.TodoItem, error) {
	arg := sqlc.ListTodoItemsByTaskAndUserIDCursorPageParams{TaskID: string(taskID), ActorKey: string(userID), PageLimit: int32(limit)}
	if anchor != nil {
		date, err := time.Parse("2006-01-02", anchor.Date)
		if err != nil {
			return nil, usecase.ErrInvalidTaskPage
		}
		arg.CursorPosition = pgtype.Int4{Int32: int32(anchor.Position), Valid: true}
		arg.CursorDate = pgtype.Date{Time: date, Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	records, err := r.queries.ListTodoItemsByTaskAndUserIDCursorPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	items := make([]dao.TodoItem, 0, len(records))
	for _, row := range records {
		item := todoItemDAO(row.ID, row.TaskID, row.Title, row.Description, row.DueDate, row.Completed, row.Position, row.IntervalWeeks, row.Frequencies, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.CreatedAt, row.UpdatedAt)
		if row.RepeatState.Valid {
			item.RepeatState = row.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		items = append(items, item)
	}
	return items, nil
}

func (r TodoItemRepository) ListActiveSeriesByUserID(ctx context.Context, userID domain.UserID) ([]dao.TodoItem, error) {
	records, err := r.queries.ListActiveTodoItemSeriesByUserID(ctx, string(userID))
	if err != nil {
		return nil, err
	}
	items := make([]dao.TodoItem, 0, len(records))
	for _, record := range records {
		item := todoItemDAO(record.ID, record.TaskID, record.Title, record.Description,
			record.DueDate, record.Completed, record.Position, record.IntervalWeeks, record.Frequencies,
			record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException,
			sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
		if record.RepeatState.Valid {
			item.RepeatState = record.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
		items = append(items, item)
	}
	return items, nil
}

func (r TodoItemRepository) Create(ctx context.Context, item domain.TodoItem) (dao.TodoItem, error) {
	recurrence, err := todoItemRecurrence(item)
	if err != nil {
		return dao.TodoItem{}, err
	}
	record, err := r.queries.CreateTodoItem(ctx, sqlc.CreateTodoItemParams{
		ID:             string(item.ID),
		TaskID:         string(item.TaskID),
		Title:          item.Title,
		Description:    stringToPgText(item.Description),
		Completed:      item.Completed,
		Position:       int32(item.Position),
		IntervalWeeks:  int32(item.IntervalWeeks),
		DueDate:        timeToPgDate(item.DueDate),
		SeriesID:       recurrence.seriesID,
		OccurrenceDate: recurrence.occurrenceDate,
		Timezone:       recurrence.timezone,
		IsException:    item.IsException,
	})
	if err != nil {
		return dao.TodoItem{}, err
	}
	result := recordToCreatedTodoItemRow(record)
	return applyTodoItemRecurrence(result, item, recurrence), nil
}

func (r TodoItemRepository) CreateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.TodoItem, appendToTail bool) (dao.TodoItem, error) {
	recurrence, err := todoItemRecurrence(item)
	if err != nil {
		return dao.TodoItem{}, err
	}
	position := pgtype.Int4{}
	if !appendToTail {
		position = pgtype.Int4{Int32: int32(item.Position), Valid: true}
	}
	record, err := r.queries.CreateTodoItemByTaskAndUserID(ctx, sqlc.CreateTodoItemByTaskAndUserIDParams{
		TaskID:         string(item.TaskID),
		UserID:         string(userID),
		Position:       position,
		ID:             string(item.ID),
		Title:          item.Title,
		Description:    stringToPgText(item.Description),
		IntervalWeeks:  int32(item.IntervalWeeks),
		DueDate:        timeToPgDate(item.DueDate),
		Frequencies:    taskFrequencyStrings(item.Frequencies),
		SeriesID:       recurrence.seriesID,
		OccurrenceDate: recurrence.occurrenceDate,
		Timezone:       recurrence.timezone,
		IsException:    item.IsException,
	})
	if err != nil {
		return dao.TodoItem{}, todoItemTaskNotFoundError(err)
	}
	result := recordToCreatedTodoItemByTaskAndUserIDRow(record)
	return applyTodoItemRecurrence(result, item, recurrence), nil
}

func (r TodoItemRepository) Update(ctx context.Context, item domain.TodoItem) (dao.TodoItem, error) {
	recurrence, err := todoItemRecurrence(item)
	if err != nil {
		return dao.TodoItem{}, err
	}
	record, err := r.queries.UpdateTodoItem(ctx, sqlc.UpdateTodoItemParams{
		ID:             string(item.ID),
		TaskID:         string(item.TaskID),
		Title:          item.Title,
		Description:    stringToPgText(item.Description),
		Completed:      item.Completed,
		Position:       int32(item.Position),
		DueDate:        timeToPgDate(item.DueDate),
		SeriesID:       recurrence.seriesID,
		OccurrenceDate: recurrence.occurrenceDate,
		Timezone:       recurrence.timezone,
		IsException:    item.IsException,
	})
	if err != nil {
		return dao.TodoItem{}, err
	}
	result := recordToUpdatedTodoItemRow(record)
	return applyTodoItemRecurrence(result, item, recurrence), nil
}

// CreateOccurrenceForOwnedTask materializes one occurrence under the source series.
func (r TodoItemRepository) CreateOccurrenceForOwnedTask(ctx context.Context, userID domain.UserID, item domain.TodoItem) (dao.TodoItem, error) {
	recurrence, err := todoItemRecurrence(item)
	if err != nil {
		return dao.TodoItem{}, err
	}
	record, err := r.queries.CreateTodoItemOccurrenceByTaskAndUserID(ctx, sqlc.CreateTodoItemOccurrenceByTaskAndUserIDParams{
		ID:             string(item.ID),
		UserID:         string(userID),
		SeriesID:       recurrence.seriesID,
		DueDate:        timeToPgDate(item.DueDate),
		OccurrenceDate: recurrence.occurrenceDate,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// A tombstoned occurrence stays absent on later refills.
			return dao.TodoItem{}, nil
		}
		return dao.TodoItem{}, todoItemTaskNotFoundError(err)
	}
	result := recordToCreatedTodoItemOccurrenceByTaskAndUserIDRow(record)
	return applyTodoItemRecurrence(result, item, recurrence), nil
}

// UpdateForOwnedTask updates one saved occurrence while keeping series rules separate.
func (r TodoItemRepository) UpdateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.TodoItem) (dao.TodoItem, error) {
	record, err := r.queries.UpdateTodoItemByTaskAndUserID(ctx, sqlc.UpdateTodoItemByTaskAndUserIDParams{
		ID:          string(item.ID),
		TaskID:      string(item.TaskID),
		UserID:      string(userID),
		Title:       item.Title,
		Description: stringToPgText(item.Description),
		DueDate:     timeToPgDate(item.DueDate),
		Position:    int32(item.Position),
	})
	if err != nil {
		return dao.TodoItem{}, todoItemRepositoryError(err)
	}
	return recordToUpdatedTodoItemByTaskAndUserIDRow(record), nil
}

func (r TodoItemRepository) SetPositionForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID, position int) error {
	_, err := r.queries.UpdateTodoItemPositionByTaskAndUserID(ctx, sqlc.UpdateTodoItemPositionByTaskAndUserIDParams{
		ID:       string(id),
		TaskID:   string(taskID),
		UserID:   string(userID),
		Position: int32(position),
	})
	return todoItemRepositoryError(err)
}

func (r TodoItemRepository) ReorderForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID, newPosition int) (dao.TodoItem, error) {
	record, err := r.queries.ReorderTodoItemsByTaskAndUserID(ctx, sqlc.ReorderTodoItemsByTaskAndUserIDParams{
		ID:       string(id),
		TaskID:   string(taskID),
		UserID:   string(userID),
		Position: int32(newPosition),
	})
	if err != nil {
		return dao.TodoItem{}, todoItemRepositoryError(err)
	}
	return recordToReorderedTodoItemByTaskAndUserIDRow(record), nil
}

func (r TodoItemRepository) CheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	_, err := r.queries.SetTodoItemCompletedByTaskAndUserID(ctx, sqlc.SetTodoItemCompletedByTaskAndUserIDParams{
		ID:        string(id),
		TaskID:    string(taskID),
		ActorKey:  string(userID),
		Completed: true,
	})
	return todoItemRepositoryError(err)
}

func (r TodoItemRepository) UncheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	_, err := r.queries.SetTodoItemCompletedByTaskAndUserID(ctx, sqlc.SetTodoItemCompletedByTaskAndUserIDParams{
		ID:        string(id),
		TaskID:    string(taskID),
		ActorKey:  string(userID),
		Completed: false,
	})
	return todoItemRepositoryError(err)
}

func (r TodoItemRepository) Delete(ctx context.Context, id domain.TodoItemID) error {
	return r.queries.DeleteTodoItem(ctx, string(id))
}

func (r TodoItemRepository) DeleteForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	_, err := r.queries.DeleteTodoItemByTaskAndUserID(ctx, sqlc.DeleteTodoItemByTaskAndUserIDParams{
		ID:       string(id),
		TaskID:   string(taskID),
		ActorKey: string(userID),
	})
	return todoItemRepositoryError(err)
}

func (r TodoItemRepository) TombstoneForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	_, err := r.queries.TombstoneTodoItemByTaskAndUserID(ctx, sqlc.TombstoneTodoItemByTaskAndUserIDParams{
		ID:     string(id),
		TaskID: string(taskID),
		UserID: string(userID),
	})
	return todoItemRepositoryError(err)
}

func (r TodoItemRepository) DeleteUneditedFutureBySeries(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, fromDate time.Time) (int64, error) {
	return r.queries.DeleteUneditedFutureTodoItemsBySeries(ctx, sqlc.DeleteUneditedFutureTodoItemsBySeriesParams{
		UserID:   string(userID),
		TaskID:   string(taskID),
		SeriesID: string(seriesID),
		FromDate: timeToPgDate(fromDate),
	})
}

func (r TodoItemRepository) DeleteUneditedFutureByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, now time.Time) (int64, error) {
	return r.queries.DeleteUneditedFutureTodoItemsByTask(ctx, sqlc.DeleteUneditedFutureTodoItemsByTaskParams{
		UserID: string(userID),
		TaskID: string(taskID),
		FromAt: timeToPgTime(now),
	})
}

type todoItemRecurrenceValues struct {
	seriesID       string
	occurrenceDate pgtype.Date
	timezone       string
}

func todoItemRecurrence(item domain.TodoItem) (todoItemRecurrenceValues, error) {
	seriesID := string(item.SeriesID)
	if seriesID == "" {
		seriesID = string(item.ID)
	}
	if item.OccurrenceDate.IsZero() || item.Timezone == "" {
		return todoItemRecurrenceValues{}, domain.ErrRecurrenceMetadataInvalid
	}
	if _, err := time.LoadLocation(item.Timezone); err != nil {
		return todoItemRecurrenceValues{}, domain.ErrRecurrenceTimezoneInvalid
	}
	return todoItemRecurrenceValues{
		seriesID:       seriesID,
		occurrenceDate: timeToPgDate(item.OccurrenceDate),
		timezone:       item.Timezone,
	}, nil
}

func applyTodoItemRecurrence(item dao.TodoItem, source domain.TodoItem, metadata todoItemRecurrenceValues) dao.TodoItem {
	item.SeriesID = metadata.seriesID
	item.OccurrenceDate = metadata.occurrenceDate.Time.Format("2006-01-02")
	item.Timezone = metadata.timezone
	item.IsException = source.IsException
	item.Deleted = source.Deleted
	return item
}

func todoItemRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrTodoItemNotFound
	}
	return todoItemConstraintError(err)
}

func todoItemTaskNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrTodoItemTaskNotFound
	}
	return todoItemConstraintError(err)
}

func todoItemConstraintError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && (pgErr.ConstraintName == "todo_items_task_occurrence_position_key" || pgErr.ConstraintName == "todo_items_task_id_position_key") {
		return usecase.ErrTodoItemPositionConflict
	}
	return err
}

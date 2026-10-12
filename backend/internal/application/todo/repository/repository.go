package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.TodoRepository = (*TodoRepository)(nil)
var _ usecase.ItemRepository = (*ItemRepository)(nil)
var _ usecase.ActionItemTodoLifecycle = (*ItemRepository)(nil)
var _ usecase.TaskTodoLifecycle = (*ItemRepository)(nil)

type TodoRepository struct {
	db      sqlc.DBTX
	queries *sqlc.Queries
}

func NewTodoRepository(db sqlc.DBTX) *TodoRepository {
	return &TodoRepository{db: db, queries: sqlc.New(db)}
}

func (r *TodoRepository) WithTx(tx pgx.Tx) *TodoRepository {
	return NewTodoRepository(tx)
}

func (r *TodoRepository) LockUser(ctx context.Context, userID string) error {
	return r.queries.LockTodoUser(ctx, userID)
}

func (r *TodoRepository) GetByUserDate(ctx context.Context, userID string, date time.Time) (dao.Todo, error) {
	record, err := r.queries.GetTodoByUserDate(ctx, sqlc.GetTodoByUserDateParams{UserID: userID, TodoDate: pgDate(date)})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.Todo{}, usecase.ErrNotFound
	}
	if err != nil {
		return dao.Todo{}, err
	}
	return todoDAO(record), nil
}

func (r *TodoRepository) Create(ctx context.Context, model domain.Todo) (dao.Todo, error) {
	if err := model.Validate(); err != nil {
		return dao.Todo{}, err
	}
	record, err := r.queries.CreateTodo(ctx, sqlc.CreateTodoParams{
		ID: model.ID, UserID: model.UserID, TodoDate: pgDate(model.Date), Timezone: model.Timezone,
	})
	if err != nil {
		return dao.Todo{}, err
	}
	return todoDAO(record), nil
}

func (r *TodoRepository) BumpRevision(ctx context.Context, id string, expected int32) (int32, error) {
	revision, err := r.queries.BumpTodoRevision(ctx, sqlc.BumpTodoRevisionParams{ID: id, ExpectedRevision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, usecase.ErrRevisionConflict
	}
	return revision, err
}

func (r *TodoRepository) UpdateTimezoneAndBumpRevision(ctx context.Context, model domain.Todo, expected int32) (int32, error) {
	if err := model.Validate(); err != nil {
		return 0, err
	}
	revision, err := r.queries.UpdateTodoTimezoneAndBumpRevision(ctx, sqlc.UpdateTodoTimezoneAndBumpRevisionParams{
		ID: model.ID, Timezone: model.Timezone, ExpectedRevision: expected,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, usecase.ErrRevisionConflict
	}
	return revision, err
}

func todoDAO(record sqlc.Todo) dao.Todo {
	return dao.Todo{ID: record.ID, UserID: record.UserID, Date: record.TodoDate.Time,
		Timezone: record.Timezone, Revision: record.Revision, CreatedAt: record.CreatedAt.Time, UpdatedAt: record.UpdatedAt.Time}
}

type ItemRepository struct {
	db      sqlc.DBTX
	queries *sqlc.Queries
}

func NewItemRepository(db sqlc.DBTX) *ItemRepository {
	return &ItemRepository{db: db, queries: sqlc.New(db)}
}

func (r *ItemRepository) WithTx(tx pgx.Tx) *ItemRepository {
	return NewItemRepository(tx)
}

func (r *ItemRepository) ListByTodoID(ctx context.Context, todoID string) ([]dao.Item, error) {
	records, err := r.queries.ListTodoItemsByTodoID(ctx, todoID)
	if err != nil {
		return nil, err
	}
	items := make([]dao.Item, 0, len(records))
	for _, record := range records {
		items = append(items, itemDAOFields(record.ID, record.TodoID, record.Kind, record.ActionItemID, record.TaskID, record.ScheduleRuleID, record.OccurrenceDate, record.PlannedMinutes, record.ScheduledStart, record.ScheduledEnd, record.Completed, record.Hidden, record.Position, record.CreatedAt, record.UpdatedAt))
	}
	return items, nil
}

func (r *ItemRepository) Insert(ctx context.Context, model domain.TodoItem) (dao.Item, error) {
	if err := model.Validate(); err != nil {
		return dao.Item{}, err
	}
	record, err := r.queries.InsertTodoItem(ctx, itemParams(model))
	if err != nil {
		return dao.Item{}, err
	}
	return itemDAOFields(record.ID, record.TodoID, record.Kind, record.ActionItemID, record.TaskID, record.ScheduleRuleID, record.OccurrenceDate, record.PlannedMinutes, record.ScheduledStart, record.ScheduledEnd, record.Completed, record.Hidden, record.Position, record.CreatedAt, record.UpdatedAt), nil
}

func (r *ItemRepository) Save(ctx context.Context, model domain.TodoItem) (dao.Item, error) {
	if err := model.Validate(); err != nil {
		return dao.Item{}, err
	}
	rows, err := r.queries.UpdateTodoItemState(ctx, sqlc.UpdateTodoItemStateParams{
		Completed: model.Completed, Hidden: model.Hidden, Position: int32(model.Position), ID: model.ID, TodoID: model.TodoID,
	})
	if err != nil {
		return dao.Item{}, err
	}
	if rows != 1 {
		return dao.Item{}, usecase.ErrFrameNotFound
	}
	return dao.Item{ID: model.ID, TodoID: model.TodoID, Kind: string(model.Kind), ActionItemID: model.ActionItemID, TaskID: model.TaskID,
		ScheduleRuleID: model.ScheduleRuleID, OccurrenceDate: model.OccurrenceDate, PlannedMinutes: model.PlannedMinutes,
		ScheduledStart: model.Start, ScheduledEnd: model.End, Completed: model.Completed, Hidden: model.Hidden,
		Position: int32(model.Position)}, nil
}

func (r *ItemRepository) ReplacePlanPreservingCompletedWork(ctx context.Context, todoID string, models []domain.TodoItem) error {
	seenIDs := make(map[string]bool, len(models))
	seenPositions := make(map[int]bool, len(models))
	for _, model := range models {
		if err := model.Validate(); err != nil || model.TodoID != todoID || seenIDs[model.ID] || seenPositions[model.Position] {
			return domain.ErrInvalidTodoItem
		}
		seenIDs[model.ID] = true
		seenPositions[model.Position] = true
	}
	if _, err := r.db.Exec(ctx, "SET CONSTRAINTS todo_items_todo_position_key DEFERRED"); err != nil {
		return err
	}
	if err := r.queries.DeleteTodoItemsExceptCompletedWork(ctx, todoID); err != nil {
		return err
	}
	for _, model := range models {
		if model.Kind.IsWork() && model.Completed {
			rows, err := r.queries.UpdateTodoItemPosition(ctx, sqlc.UpdateTodoItemPositionParams{
				Position: int32(model.Position), ID: model.ID, TodoID: todoID,
			})
			if err != nil {
				return err
			}
			if rows != 1 {
				return fmt.Errorf("completed action item %s disappeared during plan replacement", model.ID)
			}
			continue
		}
		if _, err := r.Insert(ctx, model); err != nil {
			return err
		}
	}
	return nil
}

func (r *ItemRepository) DeleteByID(ctx context.Context, todoID, itemID string) error {
	rows, err := r.queries.DeleteTodoItemByID(ctx, sqlc.DeleteTodoItemByIDParams{ID: itemID, TodoID: todoID})
	if err != nil {
		return err
	}
	if rows != 1 {
		return usecase.ErrFrameNotFound
	}
	return nil
}

func (r *ItemRepository) HasTodayActionItem(ctx context.Context, userID, actionItemID string) (bool, error) {
	return r.queries.HasTodayTodoItemForActionItem(ctx, sqlc.HasTodayTodoItemForActionItemParams{UserID: userID, ActionItemID: actionItemID})
}

func (r *ItemRepository) HasCompletedFrame(ctx context.Context, actionItemID string) (bool, error) {
	return r.queries.HasCompletedTodoFrameForActionItem(ctx, actionItemID)
}

func (r *ItemRepository) HasTodayWork(ctx context.Context, userID string, ref dao.WorkSourceRef) (bool, error) {
	return r.queries.HasTodayTodoWork(ctx, sqlc.HasTodayTodoWorkParams{UserID: userID, Kind: ref.Kind, SourceID: ref.ID})
}

func (r *ItemRepository) HasCompletedWork(ctx context.Context, ref dao.WorkSourceRef) (bool, error) {
	return r.queries.HasCompletedTodoWork(ctx, sqlc.HasCompletedTodoWorkParams{Kind: ref.Kind, SourceID: ref.ID})
}

func (r *ItemRepository) HasCompletedTaskFrame(ctx context.Context, taskID string) (bool, error) {
	return r.queries.HasCompletedTodoFrameForTask(ctx, taskID)
}

func (r *ItemRepository) HasTodayTaskFrame(ctx context.Context, userID, taskID string) (bool, error) {
	return r.queries.HasTodayTodoFrameForTask(ctx, sqlc.HasTodayTodoFrameForTaskParams{UserID: userID, TaskID: taskID})
}

func (r *ItemRepository) HasTodayTaskWork(ctx context.Context, userID, taskID string) (bool, error) {
	return r.queries.HasTodayTodoWorkForTask(ctx, sqlc.HasTodayTodoWorkForTaskParams{UserID: userID, TaskID: taskID})
}

func (r *ItemRepository) CompleteTodayTaskFrames(ctx context.Context, userID, taskID string) error {
	_, err := r.queries.CompleteTodayTodoFramesForTask(ctx, sqlc.CompleteTodayTodoFramesForTaskParams{UserID: userID, TaskID: taskID})
	return err
}

func (r *ItemRepository) ReopenTaskFrames(ctx context.Context, taskID string) error {
	_, err := r.queries.ReopenTodoFramesForTask(ctx, taskID)
	return err
}

func (r *ItemRepository) CompleteTodayFrames(ctx context.Context, userID, actionItemID string) error {
	_, err := r.queries.CompleteTodayTodoFramesForActionItem(ctx, sqlc.CompleteTodayTodoFramesForActionItemParams{
		UserID: userID, ActionItemID: actionItemID,
	})
	return err
}

func (r *ItemRepository) ReopenFrames(ctx context.Context, actionItemID string) error {
	_, err := r.queries.ReopenTodoFramesForActionItem(ctx, actionItemID)
	return err
}

func itemParams(model domain.TodoItem) sqlc.InsertTodoItemParams {
	return sqlc.InsertTodoItemParams{
		ID: model.ID, TodoID: model.TodoID, Kind: string(model.Kind),
		ActionItemID: pgText(model.ActionItemID), TaskID: pgText(model.TaskID), ScheduleRuleID: pgText(model.ScheduleRuleID),
		OccurrenceDate: pgNullableDate(model.OccurrenceDate), PlannedMinutes: pgInt(model.PlannedMinutes),
		ScheduledStart: pgTimestamp(model.Start), ScheduledEnd: pgTimestamp(model.End),
		Completed: model.Completed, Hidden: model.Hidden, Position: int32(model.Position),
	}
}

func itemDAOFields(id, todoID, kind string, actionID, taskID, scheduleID pgtype.Text, occurrence pgtype.Date, minutes pgtype.Int4, start, end pgtype.Timestamptz, completed, hidden bool, position int32, created, updated pgtype.Timestamptz) dao.Item {
	return dao.Item{ID: id, TodoID: todoID, Kind: kind, ActionItemID: pgTextValue(actionID), TaskID: pgTextValue(taskID),
		ScheduleRuleID: pgTextValue(scheduleID), OccurrenceDate: pgDateValue(occurrence), PlannedMinutes: pgIntValue(minutes),
		ScheduledStart: pgTimestampValue(start), ScheduledEnd: pgTimestampValue(end), Completed: completed, Hidden: hidden,
		Position: position, CreatedAt: created.Time, UpdatedAt: updated.Time}
}

func pgDate(value time.Time) pgtype.Date {
	return pgtype.Date{Time: time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

func pgNullableDate(value time.Time) pgtype.Date {
	if value.IsZero() {
		return pgtype.Date{}
	}
	return pgDate(value)
}

func pgText(value string) pgtype.Text { return pgtype.Text{String: value, Valid: value != ""} }

func pgInt(value *int) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*value), Valid: true}
}

func pgTimestamp(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func pgTextValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func pgIntValue(value pgtype.Int4) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int32)
	return &v
}

func pgDateValue(value pgtype.Date) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func pgTimestampValue(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time
	return &v
}

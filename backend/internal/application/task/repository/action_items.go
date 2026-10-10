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

var _ usecase.ActionItemRepository = ActionItemRepository{}

type ActionItemRepository struct {
	queries *sqlc.Queries
}

func NewActionItemRepository(db sqlc.DBTX) *ActionItemRepository {
	return &ActionItemRepository{
		queries: sqlc.New(db),
	}
}

func (r *ActionItemRepository) WithTx(tx pgx.Tx) *ActionItemRepository {
	return &ActionItemRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r ActionItemRepository) Get(ctx context.Context, id domain.ActionItemID) (dao.ActionItem, error) {
	record, err := r.queries.GetActionItem(ctx, string(id))
	if err != nil {
		return dao.ActionItem{}, err
	}
	return recordToActionItemRow(record), nil
}

func (r ActionItemRepository) GetForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) (dao.ActionItem, error) {
	record, err := r.queries.GetActionItemByTaskAndUserID(ctx, sqlc.GetActionItemByTaskAndUserIDParams{
		ID:       string(id),
		TaskID:   string(taskID),
		ActorKey: string(userID),
	})
	if err != nil {
		return dao.ActionItem{}, actionItemRepositoryError(err)
	}
	return recordToActionItemByTaskAndUserIDRow(record), nil
}

func (r ActionItemRepository) GetForCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, capability shared.Capability) (dao.ActionItem, error) {
	record, err := r.queries.GetActionItemByTaskAndUserIDForCommand(ctx, sqlc.GetActionItemByTaskAndUserIDForCommandParams{
		ID:         string(id),
		TaskID:     string(taskID),
		ActorID:    string(userID),
		ResourceID: string(capability.Resource),
		Action:     sqlc.Action(capability.Action),
	})
	if err != nil {
		return dao.ActionItem{}, actionItemRepositoryError(err)
	}
	item := actionItemDAO(record.ID, record.TaskID, record.Title, record.Description, record.DueDate, record.Completed, record.Position, record.IntervalWeeks, record.Frequencies, record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException, sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
	if record.RepeatState.Valid {
		item.RepeatState = record.RepeatState.String
	}
	item.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
	return applyActionItemPlanning(item, record.EstimatedMinutes, record.Priority), nil
}

func (r ActionItemRepository) ListByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.ActionItem, error) {
	records, err := r.queries.ListActionItemsByTaskAndUserID(ctx, sqlc.ListActionItemsByTaskAndUserIDParams{
		TaskID: string(taskID),
		UserID: string(userID),
	})
	if err != nil {
		return nil, err
	}
	return recordsToActionItems(records), nil
}

// ListByTaskForOccurrenceProjection includes deleted rows so callers can suppress tombstoned virtual dates.
func (r ActionItemRepository) ListByTaskForOccurrenceProjection(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.ActionItem, error) {
	records, err := r.queries.ListActionItemsForOccurrenceProjectionByTaskAndUserID(ctx, sqlc.ListActionItemsForOccurrenceProjectionByTaskAndUserIDParams{
		TaskID: string(taskID),
		UserID: string(userID),
	})
	if err != nil {
		return nil, err
	}
	items := make([]dao.ActionItem, 0, len(records))
	for _, row := range records {
		item := actionItemDAO(row.ID, row.TaskID, row.Title, row.Description, row.DueDate, row.Completed, row.Position, row.IntervalWeeks, row.Frequencies, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.CreatedAt, row.UpdatedAt)
		if row.RepeatState.Valid {
			item.RepeatState = row.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		items = append(items, applyActionItemPlanning(item, row.EstimatedMinutes, row.Priority))
	}
	return items, nil
}

func (r ActionItemRepository) ReadTaskListProjection(ctx context.Context, userID domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	sources := dao.TaskListProjectionSources{ActionItemsByTask: make(map[string][]dao.ActionItem, len(taskIDs)), SkippedByTask: make(map[string]map[string]map[string]bool, len(taskIDs))}
	if len(taskIDs) == 0 {
		return sources, nil
	}
	rows, err := r.queries.ListActionItemsForOccurrenceProjectionByTaskIDsAndUserID(ctx, sqlc.ListActionItemsForOccurrenceProjectionByTaskIDsAndUserIDParams{TaskIds: taskIDs, UserID: string(userID)})
	if err != nil {
		return dao.TaskListProjectionSources{}, err
	}
	for _, row := range rows {
		item := actionItemDAO(row.ID, row.TaskID, row.Title, row.Description, row.DueDate, row.Completed, row.Position, row.IntervalWeeks, row.Frequencies, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.CreatedAt, row.UpdatedAt)
		if row.RepeatState.Valid {
			item.RepeatState = row.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		item = applyActionItemPlanning(item, row.EstimatedMinutes, row.Priority)
		sources.ActionItemsByTask[item.TaskID] = append(sources.ActionItemsByTask[item.TaskID], item)
	}
	skippedRows, err := r.queries.ListActionItemSkippedOccurrencesByTaskIDsAndUserID(ctx, sqlc.ListActionItemSkippedOccurrencesByTaskIDsAndUserIDParams{TaskIds: taskIDs, UserID: string(userID)})
	if err != nil {
		return dao.TaskListProjectionSources{}, err
	}
	for _, row := range skippedRows {
		if sources.SkippedByTask[row.TaskID] == nil {
			sources.SkippedByTask[row.TaskID] = make(map[string]map[string]bool)
		}
		if sources.SkippedByTask[row.TaskID][row.SeriesID] == nil {
			sources.SkippedByTask[row.TaskID][row.SeriesID] = make(map[string]bool)
		}
		if date := pgDateString(row.OccurrenceDate); date != nil {
			sources.SkippedByTask[row.TaskID][row.SeriesID][*date] = true
		}
	}
	return sources, nil
}

func (r ActionItemRepository) ListByTaskForOccurrenceCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, capability shared.Capability) ([]dao.ActionItem, error) {
	records, err := r.queries.ListActionItemsForOccurrenceCommandByTaskAndUserID(ctx, sqlc.ListActionItemsForOccurrenceCommandByTaskAndUserIDParams{
		TaskID:     string(taskID),
		ActorID:    string(userID),
		ResourceID: string(capability.Resource),
		Action:     sqlc.Action(capability.Action),
	})
	if err != nil {
		return nil, err
	}
	items := make([]dao.ActionItem, 0, len(records))
	for _, row := range records {
		item := actionItemDAO(row.ID, row.TaskID, row.Title, row.Description, row.DueDate, row.Completed, row.Position, row.IntervalWeeks, row.Frequencies, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.CreatedAt, row.UpdatedAt)
		if row.RepeatState.Valid {
			item.RepeatState = row.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		items = append(items, applyActionItemPlanning(item, row.EstimatedMinutes, row.Priority))
	}
	return items, nil
}

func (r ActionItemRepository) ListByTaskCursor(ctx context.Context, userID domain.UserID, taskID domain.TaskID, limit int, anchor *usecase.CursorAnchor) ([]dao.ActionItem, error) {
	arg := sqlc.ListActionItemsByTaskAndUserIDCursorPageParams{TaskID: string(taskID), ActorKey: string(userID), PageLimit: int32(limit)}
	if anchor != nil {
		date, err := time.Parse("2006-01-02", anchor.Date)
		if err != nil {
			return nil, usecase.ErrInvalidTaskPage
		}
		arg.CursorPosition = pgtype.Int4{Int32: int32(anchor.Position), Valid: true}
		arg.CursorDate = pgtype.Date{Time: date, Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	records, err := r.queries.ListActionItemsByTaskAndUserIDCursorPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	items := make([]dao.ActionItem, 0, len(records))
	for _, row := range records {
		item := actionItemDAO(row.ID, row.TaskID, row.Title, row.Description, row.DueDate, row.Completed, row.Position, row.IntervalWeeks, row.Frequencies, row.SeriesID, row.OccurrenceDate, row.Timezone, row.IsException, sqlcBoolean(row.Deleted), row.CreatedAt, row.UpdatedAt)
		if row.RepeatState.Valid {
			item.RepeatState = row.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(row.FrequencyAnchorDate)
		items = append(items, applyActionItemPlanning(item, row.EstimatedMinutes, row.Priority))
	}
	return items, nil
}

func (r ActionItemRepository) ListActiveSeriesByUserID(ctx context.Context, userID domain.UserID) ([]dao.ActionItem, error) {
	records, err := r.queries.ListActiveActionItemSeriesByUserID(ctx, string(userID))
	if err != nil {
		return nil, err
	}
	items := make([]dao.ActionItem, 0, len(records))
	for _, record := range records {
		item := actionItemDAO(record.ID, record.TaskID, record.Title, record.Description,
			record.DueDate, record.Completed, record.Position, record.IntervalWeeks, record.Frequencies,
			record.SeriesID, record.OccurrenceDate, record.Timezone, record.IsException,
			sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt)
		if record.RepeatState.Valid {
			item.RepeatState = record.RepeatState.String
		}
		item.FrequencyAnchorDate = pgDateUnix(record.FrequencyAnchorDate)
		items = append(items, applyActionItemPlanning(item, record.EstimatedMinutes, record.Priority))
	}
	return items, nil
}

func (r ActionItemRepository) Create(ctx context.Context, item domain.ActionItem) (dao.ActionItem, error) {
	recurrence, err := actionItemRecurrence(item)
	if err != nil {
		return dao.ActionItem{}, err
	}
	record, err := r.queries.CreateActionItem(ctx, sqlc.CreateActionItemParams{
		ID:               string(item.ID),
		TaskID:           string(item.TaskID),
		Title:            item.Title,
		Description:      stringToPgText(item.Description),
		Completed:        item.Completed,
		Position:         int32(item.Position),
		IntervalWeeks:    int32(item.IntervalWeeks),
		DueDate:          timeToPgDate(item.DueDate),
		EstimatedMinutes: intPointerToPgInt(item.EstimatedMinutes),
		Priority:         taskPriorityString(item.Priority),
		SeriesID:         recurrence.seriesID,
		OccurrenceDate:   recurrence.occurrenceDate,
		Timezone:         recurrence.timezone,
		IsException:      item.IsException,
	})
	if err != nil {
		return dao.ActionItem{}, err
	}
	result := recordToCreatedActionItemRow(record)
	return applyActionItemRecurrence(result, item, recurrence), nil
}

func (r ActionItemRepository) CreateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.ActionItem, appendToTail bool) (dao.ActionItem, error) {
	recurrence, err := actionItemRecurrence(item)
	if err != nil {
		return dao.ActionItem{}, err
	}
	position := pgtype.Int4{}
	if !appendToTail {
		position = pgtype.Int4{Int32: int32(item.Position), Valid: true}
	}
	record, err := r.queries.CreateActionItemByTaskAndUserID(ctx, sqlc.CreateActionItemByTaskAndUserIDParams{
		TaskID:           string(item.TaskID),
		UserID:           string(userID),
		Position:         position,
		ID:               string(item.ID),
		Title:            item.Title,
		Description:      stringToPgText(item.Description),
		IntervalWeeks:    int32(item.IntervalWeeks),
		DueDate:          timeToPgDate(item.DueDate),
		EstimatedMinutes: intPointerToPgInt(item.EstimatedMinutes),
		Priority:         taskPriorityString(item.Priority),
		Frequencies:      taskFrequencyStrings(item.Frequencies),
		SeriesID:         recurrence.seriesID,
		OccurrenceDate:   recurrence.occurrenceDate,
		Timezone:         recurrence.timezone,
		IsException:      item.IsException,
	})
	if err != nil {
		return dao.ActionItem{}, actionItemTaskNotFoundError(err)
	}
	result := recordToCreatedActionItemByTaskAndUserIDRow(record)
	return applyActionItemRecurrence(result, item, recurrence), nil
}

func (r ActionItemRepository) Update(ctx context.Context, item domain.ActionItem) (dao.ActionItem, error) {
	recurrence, err := actionItemRecurrence(item)
	if err != nil {
		return dao.ActionItem{}, err
	}
	record, err := r.queries.UpdateActionItem(ctx, sqlc.UpdateActionItemParams{
		ID:               string(item.ID),
		TaskID:           string(item.TaskID),
		Title:            item.Title,
		Description:      stringToPgText(item.Description),
		Completed:        item.Completed,
		Position:         int32(item.Position),
		DueDate:          timeToPgDate(item.DueDate),
		EstimatedMinutes: intPointerToPgInt(item.EstimatedMinutes),
		Priority:         taskPriorityString(item.Priority),
		SeriesID:         recurrence.seriesID,
		OccurrenceDate:   recurrence.occurrenceDate,
		Timezone:         recurrence.timezone,
		IsException:      item.IsException,
	})
	if err != nil {
		return dao.ActionItem{}, err
	}
	result := recordToUpdatedActionItemRow(record)
	return applyActionItemRecurrence(result, item, recurrence), nil
}

// CreateOccurrenceForOwnedTask materializes one occurrence under the source series.
func (r ActionItemRepository) CreateOccurrenceForOwnedTask(ctx context.Context, userID domain.UserID, item domain.ActionItem) (dao.ActionItem, error) {
	recurrence, err := actionItemRecurrence(item)
	if err != nil {
		return dao.ActionItem{}, err
	}
	record, err := r.queries.CreateActionItemOccurrenceByTaskAndUserID(ctx, sqlc.CreateActionItemOccurrenceByTaskAndUserIDParams{
		ID:             string(item.ID),
		UserID:         string(userID),
		SeriesID:       recurrence.seriesID,
		DueDate:        timeToPgDate(item.DueDate),
		OccurrenceDate: recurrence.occurrenceDate,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// A tombstoned occurrence stays absent on later refills.
			return dao.ActionItem{}, nil
		}
		return dao.ActionItem{}, actionItemTaskNotFoundError(err)
	}
	result := recordToCreatedActionItemOccurrenceByTaskAndUserIDRow(record)
	return applyActionItemRecurrence(result, item, recurrence), nil
}

// UpdateForOwnedTask updates one saved occurrence while keeping series rules separate.
func (r ActionItemRepository) UpdateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.ActionItem) (dao.ActionItem, error) {
	record, err := r.queries.UpdateActionItemByTaskAndUserID(ctx, sqlc.UpdateActionItemByTaskAndUserIDParams{
		ID:               string(item.ID),
		TaskID:           string(item.TaskID),
		UserID:           string(userID),
		Title:            item.Title,
		Description:      stringToPgText(item.Description),
		DueDate:          timeToPgDate(item.DueDate),
		Position:         int32(item.Position),
		EstimatedMinutes: intPointerToPgInt(item.EstimatedMinutes),
		Priority:         taskPriorityString(item.Priority),
	})
	if err != nil {
		return dao.ActionItem{}, actionItemRepositoryError(err)
	}
	return recordToUpdatedActionItemByTaskAndUserIDRow(record), nil
}

func (r ActionItemRepository) SetPositionForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, position int) error {
	_, err := r.queries.UpdateActionItemPositionByTaskAndUserID(ctx, sqlc.UpdateActionItemPositionByTaskAndUserIDParams{
		ID:       string(id),
		TaskID:   string(taskID),
		UserID:   string(userID),
		Position: int32(position),
	})
	return actionItemRepositoryError(err)
}

func (r ActionItemRepository) ReorderForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, newPosition int) (dao.ActionItem, error) {
	record, err := r.queries.ReorderActionItemsByTaskAndUserID(ctx, sqlc.ReorderActionItemsByTaskAndUserIDParams{
		ID:       string(id),
		TaskID:   string(taskID),
		UserID:   string(userID),
		Position: int32(newPosition),
	})
	if err != nil {
		return dao.ActionItem{}, actionItemRepositoryError(err)
	}
	return recordToReorderedActionItemByTaskAndUserIDRow(record), nil
}

func (r ActionItemRepository) CheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error {
	_, err := r.queries.SetActionItemCompletedByTaskAndUserID(ctx, sqlc.SetActionItemCompletedByTaskAndUserIDParams{
		ID:        string(id),
		TaskID:    string(taskID),
		ActorKey:  string(userID),
		Completed: true,
	})
	return actionItemRepositoryError(err)
}

func (r ActionItemRepository) UncheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error {
	_, err := r.queries.SetActionItemCompletedByTaskAndUserID(ctx, sqlc.SetActionItemCompletedByTaskAndUserIDParams{
		ID:        string(id),
		TaskID:    string(taskID),
		ActorKey:  string(userID),
		Completed: false,
	})
	return actionItemRepositoryError(err)
}

func (r ActionItemRepository) Delete(ctx context.Context, id domain.ActionItemID) error {
	return r.queries.DeleteActionItem(ctx, string(id))
}

func (r ActionItemRepository) DeleteForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error {
	_, err := r.queries.DeleteActionItemByTaskAndUserID(ctx, sqlc.DeleteActionItemByTaskAndUserIDParams{
		ID:       string(id),
		TaskID:   string(taskID),
		ActorKey: string(userID),
	})
	return actionItemRepositoryError(err)
}

func (r ActionItemRepository) TombstoneForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error {
	_, err := r.queries.TombstoneActionItemByTaskAndUserID(ctx, sqlc.TombstoneActionItemByTaskAndUserIDParams{
		ID:     string(id),
		TaskID: string(taskID),
		UserID: string(userID),
	})
	return actionItemRepositoryError(err)
}

func (r ActionItemRepository) DeleteUneditedFutureBySeries(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, fromDate time.Time) (int64, error) {
	return r.queries.DeleteUneditedFutureActionItemsBySeries(ctx, sqlc.DeleteUneditedFutureActionItemsBySeriesParams{
		UserID:   string(userID),
		TaskID:   string(taskID),
		SeriesID: string(seriesID),
		FromDate: timeToPgDate(fromDate),
	})
}

func (r ActionItemRepository) DeleteUneditedFutureByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, now time.Time) (int64, error) {
	return r.queries.DeleteUneditedFutureActionItemsByTask(ctx, sqlc.DeleteUneditedFutureActionItemsByTaskParams{
		UserID: string(userID),
		TaskID: string(taskID),
		FromAt: timeToPgTime(now),
	})
}

type actionItemRecurrenceValues struct {
	seriesID       string
	occurrenceDate pgtype.Date
	timezone       string
}

func actionItemRecurrence(item domain.ActionItem) (actionItemRecurrenceValues, error) {
	seriesID := string(item.SeriesID)
	if seriesID == "" {
		seriesID = string(item.ID)
	}
	if item.OccurrenceDate.IsZero() || item.Timezone == "" {
		return actionItemRecurrenceValues{}, domain.ErrRecurrenceMetadataInvalid
	}
	if _, err := time.LoadLocation(item.Timezone); err != nil {
		return actionItemRecurrenceValues{}, domain.ErrRecurrenceTimezoneInvalid
	}
	return actionItemRecurrenceValues{
		seriesID:       seriesID,
		occurrenceDate: timeToPgDate(item.OccurrenceDate),
		timezone:       item.Timezone,
	}, nil
}

func applyActionItemRecurrence(item dao.ActionItem, source domain.ActionItem, metadata actionItemRecurrenceValues) dao.ActionItem {
	item.SeriesID = metadata.seriesID
	item.OccurrenceDate = metadata.occurrenceDate.Time.Format("2006-01-02")
	item.Timezone = metadata.timezone
	item.IsException = source.IsException
	item.Deleted = source.Deleted
	return item
}

func actionItemRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrActionItemNotFound
	}
	return actionItemConstraintError(err)
}

func actionItemTaskNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrActionItemTaskNotFound
	}
	return actionItemConstraintError(err)
}

func actionItemConstraintError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && (pgErr.ConstraintName == "action_items_task_occurrence_position_key" || pgErr.ConstraintName == "action_items_task_id_position_key") {
		return usecase.ErrActionItemPositionConflict
	}
	return err
}

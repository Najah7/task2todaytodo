package repository

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.ActionItemRepository = ActionItemRepository{}

type ActionItemRepository struct {
	queries *sqlc.Queries
}

func NewActionItemRepository(db sqlc.DBTX) *ActionItemRepository {
	return &ActionItemRepository{queries: sqlc.New(db)}
}

func (r *ActionItemRepository) WithTx(tx pgx.Tx) *ActionItemRepository {
	return &ActionItemRepository{queries: r.queries.WithTx(tx)}
}

func (r ActionItemRepository) GetForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) (dao.ActionItem, error) {
	record, err := r.queries.GetActionItemByTaskAndUserID(ctx, sqlc.GetActionItemByTaskAndUserIDParams{
		ID: string(id), TaskID: string(taskID), UserID: string(userID),
	})
	if err != nil {
		return dao.ActionItem{}, actionItemRepositoryError(err)
	}
	return actionItemDAO(record.ID, record.TaskID, record.Title, record.Description, record.DueDate,
		record.EstimatedMinutes, record.ActualMinutes, record.Started, sqlcBoolean(record.EstimateEditable), record.Priority, record.Completed, record.Position,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt), nil
}

func (r ActionItemRepository) GetForCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, capability shared.Capability) (dao.ActionItem, error) {
	record, err := r.queries.GetActionItemByTaskAndUserIDForCommand(ctx, sqlc.GetActionItemByTaskAndUserIDForCommandParams{
		ID: string(id), TaskID: string(taskID), ActorID: string(userID),
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if err != nil {
		return dao.ActionItem{}, actionItemRepositoryError(err)
	}
	return actionItemDAO(record.ID, record.TaskID, record.Title, record.Description, record.DueDate,
		record.EstimatedMinutes, record.ActualMinutes, record.Started, sqlcBoolean(record.EstimateEditable), record.Priority, record.Completed, record.Position,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt), nil
}

func (r ActionItemRepository) ListByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.ActionItem, error) {
	records, err := r.queries.ListActionItemsByTaskAndUserID(ctx, sqlc.ListActionItemsByTaskAndUserIDParams{
		TaskID: string(taskID), UserID: string(userID),
	})
	if err != nil {
		return nil, err
	}
	items := make([]dao.ActionItem, 0, len(records))
	for _, record := range records {
		items = append(items, actionItemDAO(record.ID, record.TaskID, record.Title, record.Description,
			record.DueDate, record.EstimatedMinutes, record.ActualMinutes, record.Started, sqlcBoolean(record.EstimateEditable), record.Priority, record.Completed, record.Position,
			sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt))
	}
	return items, nil
}

func (r ActionItemRepository) ReadTaskListProjection(ctx context.Context, userID domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	sources := dao.TaskListProjectionSources{ActionItemsByTask: make(map[string][]dao.ActionItem, len(taskIDs)), StartedTasks: make(map[string]bool, len(taskIDs)), TasksWithChildren: make(map[string]bool, len(taskIDs)), CanCreateActionItems: make(map[string]bool, len(taskIDs))}
	if len(taskIDs) == 0 {
		return sources, nil
	}
	rows, err := r.queries.ListActionItemsForTaskIDsAndUserID(ctx, sqlc.ListActionItemsForTaskIDsAndUserIDParams{
		TaskIds: taskIDs, UserID: string(userID),
	})
	if err != nil {
		return dao.TaskListProjectionSources{}, err
	}
	for _, row := range rows {
		item := actionItemDAO(row.ID, row.TaskID, row.Title, row.Description, row.DueDate,
			row.EstimatedMinutes, row.ActualMinutes, row.Started, sqlcBoolean(row.EstimateEditable), row.Priority, row.Completed, row.Position,
			sqlcBoolean(row.Deleted), row.CreatedAt, row.UpdatedAt)
		sources.ActionItemsByTask[item.TaskID] = append(sources.ActionItemsByTask[item.TaskID], item)
	}
	startedRows, err := r.queries.ListTasksWithCompletedTodoFrames(ctx, sqlc.ListTasksWithCompletedTodoFramesParams{TaskIds: taskIDs, UserID: string(userID)})
	if err != nil {
		return dao.TaskListProjectionSources{}, err
	}
	for _, row := range startedRows {
		if row.Valid {
			sources.StartedTasks[row.String] = true
		}
	}
	childrenRows, err := r.queries.ListTasksWithActionItemChildren(ctx, sqlc.ListTasksWithActionItemChildrenParams{TaskIds: taskIDs, UserID: string(userID)})
	if err != nil {
		return dao.TaskListProjectionSources{}, err
	}
	for _, row := range childrenRows {
		sources.TasksWithChildren[row] = true
	}
	createRows, err := r.queries.ListTasksWithActionItemCreatePermission(ctx, sqlc.ListTasksWithActionItemCreatePermissionParams{TaskIds: taskIDs, UserID: string(userID)})
	if err != nil {
		return dao.TaskListProjectionSources{}, err
	}
	for _, row := range createRows {
		sources.CanCreateActionItems[row] = true
	}
	return sources, nil
}

func (r ActionItemRepository) CountForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, capability shared.Capability) (int, error) {
	count, err := r.queries.CountActionItemsForTaskCommand(ctx, sqlc.CountActionItemsForTaskCommandParams{
		TaskID: string(taskID), ActorID: string(userID), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	return int(count), err
}

func (r ActionItemRepository) CountForProjectPlanLimit(ctx context.Context, projectID string) (int, error) {
	count, err := r.queries.CountActionItemsForProjectPlanLimit(ctx, projectID)
	return int(count), err
}

func (r ActionItemRepository) CountProjectlessForUserPlanLimit(ctx context.Context, userID domain.UserID) (int, error) {
	count, err := r.queries.CountProjectlessActionItemsForUserPlanLimit(ctx, string(userID))
	return int(count), err
}

func (r ActionItemRepository) CountForTaskPlanLimit(ctx context.Context, taskID domain.TaskID) (int, error) {
	count, err := r.queries.CountActionItemsForTaskPlanLimit(ctx, string(taskID))
	return int(count), err
}

func (r ActionItemRepository) ListByTaskCursor(ctx context.Context, userID domain.UserID, taskID domain.TaskID, limit int, anchor *usecase.CursorAnchor) ([]dao.ActionItem, error) {
	arg := sqlc.ListActionItemsByTaskAndUserIDCursorPageParams{TaskID: string(taskID), UserID: string(userID), PageLimit: int32(limit)}
	if anchor != nil {
		arg.CursorPosition = pgtype.Int4{Int32: int32(anchor.Position), Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	records, err := r.queries.ListActionItemsByTaskAndUserIDCursorPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	items := make([]dao.ActionItem, 0, len(records))
	for _, record := range records {
		items = append(items, actionItemDAO(record.ID, record.TaskID, record.Title, record.Description,
			record.DueDate, record.EstimatedMinutes, record.ActualMinutes, record.Started, sqlcBoolean(record.EstimateEditable), record.Priority, record.Completed, record.Position,
			sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt))
	}
	return items, nil
}

func (r ActionItemRepository) CreateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.ActionItem, appendToTail bool) (dao.ActionItem, error) {
	position := pgtype.Int4{}
	if !appendToTail {
		position = pgtype.Int4{Int32: int32(item.Position), Valid: true}
	}
	record, err := r.queries.CreateActionItemByTaskAndUserID(ctx, sqlc.CreateActionItemByTaskAndUserIDParams{
		TaskID: string(item.TaskID), UserID: string(userID), Position: position,
		ID: string(item.ID), Title: item.Title, Description: stringToPgText(item.Description),
		DueDate: timeToPgDate(item.DueDate), EstimatedMinutes: intPointerToPgInt(item.EstimatedMinutes),
		Priority: taskPriorityString(item.Priority),
	})
	if err != nil {
		return dao.ActionItem{}, actionItemTaskNotFoundError(err)
	}
	return actionItemDAO(record.ID, record.TaskID, record.Title, record.Description, record.DueDate,
		record.EstimatedMinutes, record.ActualMinutes, record.Started, sqlcBoolean(record.EstimateEditable), record.Priority, record.Completed, record.Position,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt), nil
}

func (r ActionItemRepository) UpdateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.ActionItem) (dao.ActionItem, error) {
	record, err := r.queries.UpdateActionItemByTaskAndUserID(ctx, sqlc.UpdateActionItemByTaskAndUserIDParams{
		ID: string(item.ID), TaskID: string(item.TaskID), UserID: string(userID),
		Title: item.Title, Description: stringToPgText(item.Description), DueDate: timeToPgDate(item.DueDate),
		EstimatedMinutes: intPointerToPgInt(item.EstimatedMinutes), Priority: taskPriorityString(item.Priority),
		ActualMinutes: intPointerToPgInt(item.ActualMinutes),
		Position:      int32(item.Position),
	})
	if err != nil {
		return dao.ActionItem{}, actionItemRepositoryError(err)
	}
	return actionItemDAO(record.ID, record.TaskID, record.Title, record.Description, record.DueDate,
		record.EstimatedMinutes, record.ActualMinutes, record.Started, sqlcBoolean(record.EstimateEditable), record.Priority, record.Completed, record.Position,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt), nil
}

func (r ActionItemRepository) SetPositionForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, position int) error {
	_, err := r.queries.UpdateActionItemPositionByTaskAndUserID(ctx, sqlc.UpdateActionItemPositionByTaskAndUserIDParams{
		ID: string(id), TaskID: string(taskID), UserID: string(userID), Position: int32(position),
	})
	return actionItemRepositoryError(err)
}

func (r ActionItemRepository) ReorderForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, newPosition int) (dao.ActionItem, error) {
	record, err := r.queries.ReorderActionItemsByTaskAndUserID(ctx, sqlc.ReorderActionItemsByTaskAndUserIDParams{
		ID: string(id), TaskID: string(taskID), UserID: string(userID), Position: int32(newPosition),
	})
	if err != nil {
		return dao.ActionItem{}, actionItemRepositoryError(err)
	}
	return actionItemDAO(record.ID, record.TaskID, record.Title, record.Description, record.DueDate,
		record.EstimatedMinutes, record.ActualMinutes, record.Started, sqlcBoolean(record.EstimateEditable), record.Priority, record.Completed, record.Position,
		sqlcBoolean(record.Deleted), record.CreatedAt, record.UpdatedAt), nil
}

func (r ActionItemRepository) CheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error {
	return r.setCompleted(ctx, userID, taskID, id, true)
}

func (r ActionItemRepository) CompleteWithActualMinutesForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, actualMinutes *int) error {
	_, err := r.queries.CompleteActionItemWithActualMinutesByTaskAndUserID(ctx, sqlc.CompleteActionItemWithActualMinutesByTaskAndUserIDParams{
		ID: string(id), TaskID: string(taskID), UserID: string(userID), ActualMinutes: intPointerToPgInt(actualMinutes),
	})
	return actionItemRepositoryError(err)
}

func (r ActionItemRepository) UncheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error {
	return r.setCompleted(ctx, userID, taskID, id, false)
}

func (r ActionItemRepository) setCompleted(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, completed bool) error {
	_, err := r.queries.SetActionItemCompletedByTaskAndUserID(ctx, sqlc.SetActionItemCompletedByTaskAndUserIDParams{
		ID: string(id), TaskID: string(taskID), UserID: string(userID), Completed: completed,
	})
	return actionItemRepositoryError(err)
}

func (r ActionItemRepository) DeleteForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error {
	_, err := r.queries.DeleteActionItemByTaskAndUserID(ctx, sqlc.DeleteActionItemByTaskAndUserIDParams{
		ID: string(id), TaskID: string(taskID), UserID: string(userID),
	})
	return actionItemRepositoryError(err)
}

func actionItemRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrActionItemNotFound
	}
	return err
}

func actionItemTaskNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrActionItemTaskNotFound
	}
	return err
}
